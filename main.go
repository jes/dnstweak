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
	c := new(dns.Client)
	c.Net = "udp"

	r, rtt, err := c.Exchange(msg, "8.8.8.8:53") // TODO: not hard-coded
	if err != nil {
		// TODO: not fatal
		log.Fatal(err)
	}

	fmt.Println("rtt = %v", rtt)
	for _, answer := range r.Answer {
		fmt.Printf("answer: %#v\n", answer)
		switch answer.(type) {
		case *dns.A:
			a := answer.(*dns.A)
			// TODO: not hard-coded
			if a.Hdr.Name == "google.com." {
				a.A = net.IP{127, 0, 0, 1}
			}
		}

	}

	w.WriteMsg(r)
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
