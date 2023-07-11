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
	listen := flag.String("listen", "", "listen address (IP:PORT or just PORT) (default: see below)")
	upstream := flag.String("upstream", "", "upstream DNS server (IP:PORT or just IP) (default: see below)")
	noResolvConf := flag.Bool("no-resolvconf", false, "disable automatic update of /etc/resolv.conf")
	noProc := flag.Bool("no-proc", false, "disable discovering the client process by looking in /proc")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: dnstweak [options] SPEC...\n\noptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "In the absence of -listen, dnstweak will first try to listen on any loopback IP\naddress (127.0.0.0/24) on port 53, and failing that use a random port number on\n127.0.0.1.\n\n")
		fmt.Fprintf(os.Stderr, "In the absence of -upstream, dnstweak will take the first nameserver configured\nin /etc/resolv.conf.\n\n")
		fmt.Fprintf(os.Stderr, "Each SPEC is a hostname, followed by an \"=\" sign, followed by a\ncomma-separated list of 1 or more IP addresses (for example\n\"example.com=127.0.0.1\").\n\n")
		fmt.Fprintf(os.Stderr, "dnstweak is a program by James Stanley. You can email me at\njames@incoherency.co.uk or read my blog at https://incoherency.co.uk/\n")
	}
	flag.Parse()

	listenAddress := *listen
	if listenAddress != "" && !strings.Contains(listenAddress, ":") {
		listenAddress = "127.0.0.1:" + listenAddress
	}

	upstreamAddress := *upstream
	if upstreamAddress != "" && !strings.Contains(upstreamAddress, ":") {
		upstreamAddress = upstreamAddress + ":53"
	}

	override := make(map[string][]net.IP)
	for _, spec := range flag.Args() {
		parseOverrideSpec(spec, override)
	}

	dnstweak := DnsTweak{
		Override:             override,
		Upstream:             upstreamAddress,
		SpliceIntoResolvConf: !*noResolvConf,
		LookInProc:           !*noProc,
	}
	err := dnstweak.ListenAndServe(listenAddress)

	if err != nil {
		log.Fatal(err)
	}
}
