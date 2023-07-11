package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

// a spec should be like foo.example.com=1.2.3.4,5.6.7.8
func parseOverrideSpec(spec string, override map[string][]net.IP) {
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

func main() {
	listen := flag.String("listen", "127.0.0.1:1053", "listen address (IP:PORT or just PORT)")
	upstream := flag.String("upstream", "8.8.8.8:53", "upstream DNS server (IP:PORT or just IP)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: dnstweak [options] SPEC...\n\noptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Each SPEC is a hostname, followed by an \"=\" sign, followed by a\ncomma-separated list of 1 or more IP addresses.\n\n")
		fmt.Fprintf(os.Stderr, "dnstweak is a program by James Stanley. You can email me at\njames@incoherency.co.uk or read my blog at https://incoherency.co.uk/\n")
	}
	flag.Parse()

	listenAddress := *listen
	if !strings.Contains(listenAddress, ":") {
		listenAddress = "127.0.0.1:" + listenAddress
	}

	upstreamAddress := *upstream
	if !strings.Contains(upstreamAddress, ":") {
		upstreamAddress = upstreamAddress + ":53"
	}

	override := make(map[string][]net.IP)
	for _, spec := range flag.Args() {
		parseOverrideSpec(spec, override)
	}

	dnstweak := DnsTweak{
		Override: override,
		Upstream: upstreamAddress,
	}
	err := dnstweak.ListenAndServe(listenAddress)

	if err != nil {
		log.Fatal(err)
	}
}
