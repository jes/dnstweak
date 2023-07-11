package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type DnsTweak struct {
	Override map[string][]net.IP
	Upstream string
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

	// TODO: look at w.RemoteAddr() to find out if it's a local connection,
	// and if so try to find out which process is on the other end

	r, overridden := d.Response(msg)
	if !overridden {
		r = d.PassThrough(msg)
	}

	d.Log(w, r, overridden)
	w.WriteMsg(r)
}

func (d *DnsTweak) Log(w dns.ResponseWriter, msg *dns.Msg, overridden bool) {
	qtype := dns.Type(msg.Question[0].Qtype)
	name := msg.Question[0].Name

	// strip trailing dot from FQDN
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}

	// format question
	line := fmt.Sprintf("%v: %v %s", w.RemoteAddr(), qtype, name)

	// format answer for A queries only
	if msg.Question[0].Qtype == dns.TypeA {
		line = line + ": "
		ips := make([]string, 0)
		for _, answer := range msg.Answer {
			switch answer.(type) {
			case *dns.A:
				ips = append(ips, fmt.Sprintf("%v", answer.(*dns.A).A))
			}
		}
		line = line + strings.Join(ips, ",")
	}
	if overridden {
		line = line + " (overridden)"
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

func (d *DnsTweak) ListenAndServe(addr string) error {
	server := dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: d,
	}

	return server.ListenAndServe()
}
