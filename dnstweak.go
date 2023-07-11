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

func (d *DnsTweak) ServeDNS(w dns.ResponseWriter, msg *dns.Msg) {
	if len(msg.Question) != 1 {
		// TODO: not fatal
		log.Fatal("unsupported question set length: %d (expected 1)", len(msg.Question))
	}

	clientProcess := ""

	// TODO: look at w.RemoteAddr() to find out if it's a local connection,
	// and if so try to find out which process is on the other end
	switch w.RemoteAddr().(type) {
	case *net.UDPAddr:
		u := w.RemoteAddr().(*net.UDPAddr)
		clientProcess = FindProcess(u)
	}

	r, overridden := d.Response(msg)
	if !overridden {
		r = d.PassThrough(msg)
	}

	d.Log(w, r, overridden, clientProcess)
	w.WriteMsg(r)
}

func (d *DnsTweak) Log(w dns.ResponseWriter, msg *dns.Msg, overridden bool, client string) {
	qtype := dns.Type(msg.Question[0].Qtype)
	name := msg.Question[0].Name

	// strip trailing dot from FQDN
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}

	// format question
	var line string
	if client == "" {
		line = fmt.Sprintf("%v: %v %s", w.RemoteAddr(), qtype, name)
	} else {
		line = fmt.Sprintf("%v (%s): %v %s", w.RemoteAddr(), client, qtype, name)
	}

	// format answer for A queries only
	if msg.Question[0].Qtype == dns.TypeA {
		line += ": "
		ips := make([]string, 0)
		for _, answer := range msg.Answer {
			switch answer.(type) {
			case *dns.A:
				ips = append(ips, fmt.Sprintf("%v", answer.(*dns.A).A))
			}
		}
		line += strings.Join(ips, ",")
	} else {
		// TODO: better formatting of other types of answer, most importantly CNAME, AAAA, PTR
		line += fmt.Sprintf(": %v", msg.Answer)
	}
	if overridden {
		line += " (overridden)"
	}
	log.Printf("%s\n", line)
}

func (d *DnsTweak) Response(msg *dns.Msg) (*dns.Msg, bool) {
	qtype := msg.Question[0].Qtype
	name := msg.Question[0].Name

	if qtype != dns.TypeA {
		return nil, false
	}

	override, exists := d.Override[name]
	if !exists {
		return nil, false
	}

	r := new(dns.Msg)
	r.SetReply(msg)
	for _, ipaddr := range override {
		r.Answer = append(r.Answer, A_record(name, ipaddr))
	}
	rand.Shuffle(len(r.Answer), func(i, j int) {
		r.Answer[i], r.Answer[j] = r.Answer[j], r.Answer[i]
	})

	return r, true
}

func (d *DnsTweak) PassThrough(msg *dns.Msg) *dns.Msg {
	c := new(dns.Client)

	r, _, err := c.Exchange(msg, d.Upstream)
	if err != nil {
		// TODO: not fatal
		log.Fatal(err)
	}

	return r
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
