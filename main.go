package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
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
	log.Printf("%s\n", line)
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
	listen := flag.String("listen", "127.0.0.1:1053", "listen address (IP:PORT or just PORT)")
	upstream := flag.String("upstream", "8.8.8.8:53", "upstream DNS server (IP:PORT or just IP)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: dnstweak [options] SPEC...\n\noptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Each SPEC is a hostname, followed by an \"=\" sign, followed by a comma-separated list of 1 or more IP addresses.\n")
		fmt.Fprintf(os.Stderr, "dnstweak is a program by James Stanley. You can email me at james@incoherency.co.uk or read my blog at https://incoherency.co.uk/\n")
	}
	flag.Parse()

	override := make(map[string][]net.IP)

	// a spec should be like foo.example.com=1.2.3.4,5.6.7.8
	for _, spec := range flag.Args() {
		host, ips_csv, found := strings.Cut(spec, "=")
		if !found {
			log.Fatalf("spec '%s' does not contain '='\n", spec)
		}

		// append trailing dot for FQDN
		if host[len(host)-1] != '.' {
			host = host + "."
		}
		if _, exists := override[host]; !exists {
			override[host] = make([]net.IP, 0)
		}

		// add each of the IP addresses
		ips := strings.Split(ips_csv, ",")
		for _, ipstr := range ips {
			ip := net.ParseIP(ipstr)
			if ip == nil {
				log.Fatalf("can't parse ip address '%s'", ipstr)
			}
			override[host] = append(override[host], ip)
		}
	}

	listenAddress := *listen
	if !strings.Contains(listenAddress, ":") {
		listenAddress = "127.0.0.1:" + listenAddress
	}

	upstreamAddress := *upstream
	if !strings.Contains(upstreamAddress, ":") {
		upstreamAddress = upstreamAddress + ":53"
	}

	handler := DnsTweakHandler{
		Override: override,
		Upstream: upstreamAddress,
	}

	server := dns.Server{
		Addr:    listenAddress,
		Net:     "udp",
		Handler: &handler,
	}

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}
}
