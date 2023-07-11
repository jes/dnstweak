package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type DnsTweakHandler struct {
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

func (handler *DnsTweakHandler) ServeDNS(w dns.ResponseWriter, msg *dns.Msg) {
	if len(msg.Question) != 1 {
		// TODO: not fatal
		log.Fatal("unsupported question set length: %d (expected 1)", len(msg.Question))
	}

	r, overridden := handler.Response(msg)
	if !overridden {
		r = handler.PassThrough(msg)
	}

	handler.Log(r, overridden)
	w.WriteMsg(r)
}

func (handler *DnsTweakHandler) Log(msg *dns.Msg, overridden bool) {
	qtype := dns.Type(msg.Question[0].Qtype)
	name := msg.Question[0].Name

	// strip trailing dot from FQDN
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}

	// format question
	line := fmt.Sprintf("%v %s", qtype, name)

	// format answer for A queries only
	if msg.Question[0].Qtype == dns.TypeA {
		line = line + ": "
		ips := make([]string, 0)
		for _, answer := range msg.Answer {
			ips = append(ips, fmt.Sprintf("%v", answer.(*dns.A).A))
		}
		line = line + strings.Join(ips, ",")
	}
	if overridden {
		line = line + " (overridden)"
	}
	fmt.Printf("%s\n", line)
}

func (handler *DnsTweakHandler) Response(msg *dns.Msg) (*dns.Msg, bool) {
	qtype := msg.Question[0].Qtype
	name := msg.Question[0].Name

	if qtype != dns.TypeA {
		return nil, false
	}

	override, exists := handler.Override[name]
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

func (handler *DnsTweakHandler) PassThrough(msg *dns.Msg) *dns.Msg {
	c := new(dns.Client)

	r, _, err := c.Exchange(msg, handler.Upstream)
	if err != nil {
		// TODO: not fatal
		log.Fatal(err)
	}

	return r
}

func main() {
	override := make(map[string][]net.IP)
	override["google.com."] = make([]net.IP, 0)
	override["google.com."] = append(override["google.com."], net.ParseIP("127.0.0.1"))
	override["google.com."] = append(override["google.com."], net.ParseIP("127.0.0.2"))

	handler := DnsTweakHandler{
		Override: override,
		Upstream: "8.8.8.8:53",
	}

	server := dns.Server{
		Addr:    "127.0.0.1:1053", // TODO: not hard-coded port
		Net:     "udp",
		Handler: &handler,
	}

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}
}
