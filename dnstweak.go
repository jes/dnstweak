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
	Server               *dns.Server
	PleaseExit           bool
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
	clientProcess := ""

	if d.LookInProc {
		switch w.RemoteAddr().(type) {
		case *net.UDPAddr:
			u := w.RemoteAddr().(*net.UDPAddr)
			clientProcess = FindProcess(u)
		}
	}

	if len(req.Question) != 1 {
		if clientProcess == "" {
			log.Printf("%v: unsupported question set length: %d (expected 1): %v\n", w.RemoteAddr(), len(req.Question), req.Question)
		} else {
			log.Printf("%v (%s): unsupported question set length: %d (expected 1): %v\n", w.RemoteAddr(), clientProcess, len(req.Question), req.Question)
		}
		resp := d.PassThrough(req)
		if resp != nil {
			w.WriteMsg(resp)
		}
		return
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
		switch req.Question[0].Qtype {
		case dns.TypeA, dns.TypeAAAA, dns.TypePTR:
			ips := make([]string, 0)
			for _, answer := range resp.Answer {
				switch answer.(type) {
				case *dns.A:
					ips = append(ips, answer.(*dns.A).A.String())
				case *dns.AAAA:
					ips = append(ips, answer.(*dns.AAAA).AAAA.String())
				case *dns.CNAME:
					ips = append(ips, strings.TrimSuffix(answer.(*dns.CNAME).Target, "."))
				case *dns.PTR:
					ips = append(ips, strings.TrimSuffix(answer.(*dns.PTR).Ptr, "."))
				default:
					ips = append(ips, answer.String())
				}
			}
			line += strings.Join(ips, ",")
		default:
			// TODO: SRV, more?
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

func (d *DnsTweak) SetupResolvConf() {
	oldContent, resolver, err := UpdateResolvConf(d.Server.PacketConn.LocalAddr().String())
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
	d.PleaseExit = true
	d.Finish()
}

func (d *DnsTweak) Finish() {
	if d.SpliceIntoResolvConf && d.OldResolvConf != "" {
		err := RestoreResolvConf(d.OldResolvConf)
		if err != nil {
			log.Printf("%v", err)
		}
	}
	if d.Server != nil {
		d.Server.Shutdown()
	}
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
		d.Server = &dns.Server{
			Addr:    a,
			Net:     "udp",
			Handler: d,
		}
		d.Server.NotifyStartedFunc = func() {
			log.Printf("listening on %v\n", d.Server.PacketConn.LocalAddr())
			if d.SpliceIntoResolvConf {
				d.SetupResolvConf()
			}
		}

		err = d.Server.ListenAndServe()
		if err != nil {
			log.Printf("%v\n", err)
		}
		if d.PleaseExit {
			os.Exit(0)
		}
		d.Finish() // just in case resolv.conf needs restoring
	}
	return err
}

// a spec should be like foo.example.com=1.2.3.4,5.6.7.8
func (d *DnsTweak) AddOverrideSpec(spec string) {
	if d.Override == nil {
		d.Override = make(map[string][]net.IP)
	}

	host, ips_csv, found := strings.Cut(spec, "=")
	if !found {
		log.Fatalf("spec '%s' does not contain '='\n", spec)
	}

	// append trailing dot for FQDN
	if host[len(host)-1] != '.' {
		host = host + "."
	}
	if _, exists := d.Override[host]; !exists {
		d.Override[host] = make([]net.IP, 0)
	}

	// add each of the IP addresses
	ips := strings.Split(ips_csv, ",")
	for _, ipstr := range ips {
		ip := net.ParseIP(ipstr)
		if ip == nil {
			log.Fatalf("can't parse ip address '%s'", ipstr)
		}
		d.Override[host] = append(d.Override[host], ip)
	}
}
