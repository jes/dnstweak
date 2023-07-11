package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"

	"github.com/miekg/dns"
)

type DnsTweakHandler struct {
	Replace  map[string][]net.IP
	Upstream string
}

func (handler *DnsTweakHandler) ServeDNS(w dns.ResponseWriter, msg *dns.Msg) {
	if len(msg.Question) != 1 {
		// TODO: not fatal
		log.Fatal("unsupported question set length: %d (expected 1)", len(msg.Question))
	}

	// if the question is an A lookup for a hostname we control, answer it ourselves, otherwise pass it on upstream
	replace, exists := handler.Replace[msg.Question[0].Name]
	if exists && msg.Question[0].Qtype == dns.TypeA {
		r := new(dns.Msg)
		r.SetReply(msg)
		for _, replacement := range replace {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   msg.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: replacement,
			}
			r.Answer = append(r.Answer, rr)
		}
		rand.Shuffle(len(r.Answer), func(i, j int) {
			r.Answer[i], r.Answer[j] = r.Answer[j], r.Answer[i]
		})
		w.WriteMsg(r)
	} else {
		handler.PassThrough(w, msg)
	}
}

func (handler *DnsTweakHandler) PassThrough(w dns.ResponseWriter, msg *dns.Msg) {
	c := new(dns.Client)

	r, rtt, err := c.Exchange(msg, handler.Upstream)
	if err != nil {
		// TODO: not fatal
		log.Fatal(err)
	}

	fmt.Printf("rtt = %v\n", rtt)

	fmt.Printf("r = %#v\n", r)
	fmt.Printf("question set = %#v\n", r.Question)
	fmt.Printf("answer set = %#v\n", r.Answer)

	w.WriteMsg(r)
}

func main() {
	replace := make(map[string][]net.IP)
	replace["google.com."] = make([]net.IP, 0)
	replace["google.com."] = append(replace["google.com."], net.ParseIP("127.0.0.1"))
	replace["google.com."] = append(replace["google.com."], net.ParseIP("127.0.0.2"))

	handler := DnsTweakHandler{
		Replace:  replace,
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
