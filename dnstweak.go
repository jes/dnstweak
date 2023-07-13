package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/miekg/dns"
)

type DnsTweak struct {
	Override             map[string][]net.IP
	Upstream             string
	SpliceIntoResolvConf bool
	LookInProc           bool
	OldResolvConf        string
}

func A_record(name string, ipaddr net.IP) *dns.A {
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: ipaddr,
	}
}

func (d *DnsTweak) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	if len(req.Question) != 1 {
		log.Printf("unsupported question set length: %d (expected 1)", len(req.Question))
		// TODO: log the request even though it may have an empty question set
		resp := d.PassThrough(req)
		if resp != nil {
			w.WriteMsg(resp)
		}
		return
	}

	clientProcess := ""

	if d.LookInProc {
		switch w.RemoteAddr().(type) {
		case *net.UDPAddr:
			u := w.RemoteAddr().(*net.UDPAddr)
			clientProcess = FindProcess(u)
		}
	}

	resp, overridden := d.Response(req)
	if !overridden {
		resp = d.PassThrough(req)
	}

	d.Log(w, req, resp, overridden, clientProcess)
	if resp != nil {
		w.WriteMsg(resp)
	}
}

func (d *DnsTweak) Log(w dns.ResponseWriter, req *dns.Msg, resp *dns.Msg, overridden bool, client string) {
	qtype := dns.Type(req.Question[0].Qtype)
	name := req.Question[0].Name

	// strip trailing dot from FQDN
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}

	// format question
	var line string
	if client == "" {
		line = fmt.Sprintf("%v: %v %s: ", w.RemoteAddr(), qtype, name)
	} else {
		line = fmt.Sprintf("%v (%s): %v %s: ", w.RemoteAddr(), client, qtype, name)
	}

	if resp != nil {
		// format answer for A queries only
		if req.Question[0].Qtype == dns.TypeA {
			ips := make([]string, 0)
			for _, answer := range resp.Answer {
				switch answer.(type) {
				case *dns.A:
					// TODO: where the answer is an A record for a different domain than the one requested, somehow show this?
					ips = append(ips, answer.(*dns.A).A.String())
				default:
					// TODO: better formatting of (for example) CNAME answers
					ips = append(ips, answer.String())
				}
			}
			line += strings.Join(ips, ",")
		} else {
			// TODO: better formatting of other types of answer, most importantly CNAME, AAAA, PTR
			line += fmt.Sprintf("%v", resp.Answer)
		}
	} else {
		line += "(error)"
	}
	if overridden {
		line += " (overridden)"
	}
	log.Printf("%s\n", line)
}

func (d *DnsTweak) Response(req *dns.Msg) (*dns.Msg, bool) {
	qtype := req.Question[0].Qtype
	name := req.Question[0].Name

	if qtype != dns.TypeA {
		return nil, false
	}

	override, exists := d.Override[name]
	if !exists {
		return nil, false
	}

	resp := new(dns.Msg)
	resp.SetReply(req)
	for _, ipaddr := range override {
		resp.Answer = append(resp.Answer, A_record(name, ipaddr))
	}
	rand.Shuffle(len(resp.Answer), func(i, j int) {
		resp.Answer[i], resp.Answer[j] = resp.Answer[j], resp.Answer[i]
	})

	return resp, true
}

func (d *DnsTweak) PassThrough(req *dns.Msg) *dns.Msg {
	c := new(dns.Client)

	resp, _, err := c.Exchange(req, d.Upstream)
	if err != nil {
		log.Printf("%v", err)
		return nil
	}

	return resp
}

func (d *DnsTweak) SetupResolvConf(server dns.Server) {
	oldContent, resolver, err := UpdateResolvConf(server.PacketConn.LocalAddr().String())
	if err != nil {
		log.Printf("%v (do you need to be root?)\n", err)
	}
	if resolver != "" {
		if d.Upstream == "" {
			log.Printf("using %s as upstream resolver\n", resolver)
			d.Upstream = resolver
		}
	}
	if oldContent != "" {
		d.OldResolvConf = oldContent
		go d.HandleSignals()
	}
}

func (d *DnsTweak) HandleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	sig := <-c
	log.Printf("received signal: %v\n", sig)
	d.Finish()
}

func (d *DnsTweak) Finish() {
	if d.SpliceIntoResolvConf {
		err := RestoreResolvConf(d.OldResolvConf)
		if err != nil {
			log.Printf("%v", err)
		}
	}
	os.Exit(0)
}

func (d *DnsTweak) ListenAndServe(addr string) error {
	log.Println("dnstweak starts")

	addrs := make([]string, 0)

	if addr != "" {
		addrs = append(addrs, addr)
	} else {
		for i := 1; i < 256; i++ {
			a := fmt.Sprintf("127.0.0.%d:53", i)
			addrs = append(addrs, a)
		}
		addrs = append(addrs, "127.0.0.1:0")
	}

	var err error
	for _, a := range addrs {
		server := dns.Server{
			Addr:    a,
			Net:     "udp",
			Handler: d,
		}
		server.NotifyStartedFunc = func() {
			log.Printf("listening on %v\n", server.PacketConn.LocalAddr())
			if d.SpliceIntoResolvConf {
				d.SetupResolvConf(server)
			}
		}

		err = server.ListenAndServe()
		if err != nil {
			log.Printf("%v\n", err)
		}
	}
	return err
}
