package main

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

type MyHandler struct {
	// TODO: should contain map of answer replacements
}

func (handler *MyHandler) ServeDNS(w dns.ResponseWriter, msg *dns.Msg) {
	if len(msg.Question) != 1 {
		// TODO: not fatal
		log.Fatal("unsupported question set length: %d (expected 1)", len(msg.Question))
	}

	// if the question is an A lookup for a hostname we control, answer it ourselves, otherwise pass it on upstream
	if msg.Question[0].Name == "google.com." && msg.Question[0].Qtype == dns.TypeA {
		r := new(dns.Msg)
		r.SetReply(msg)
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   msg.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP("127.0.0.1"),
		}
		r.Answer = append(r.Answer, rr)
		w.WriteMsg(r)
	} else {
		c := new(dns.Client)
		c.Net = "udp"

		r, rtt, err := c.Exchange(msg, "8.8.8.8:53") // TODO: not hard-coded
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
}

func main() {
	handler := MyHandler{}

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
