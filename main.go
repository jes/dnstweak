package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

var VERSION = "v0.2"

func main() {
	listen := flag.String("listen", "", "listen address (IP:PORT or just PORT) (default: see below)")
	upstream := flag.String("upstream", "", "upstream DNS server (IP:PORT or just IP) (default: see below)")
	noResolvConf := flag.Bool("no-resolvconf", false, "disable automatic update of /etc/resolv.conf")
	noProc := flag.Bool("no-proc", false, "disable discovering the client process by looking in /proc")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "dnstweak %s\n\n", VERSION)
		fmt.Fprintf(out, "usage: dnstweak [options] SPEC...\n\noptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(out, "\n")
		fmt.Fprintf(out, "In the absence of -listen, dnstweak will first try to listen on any loopback IP\naddress (127.0.0.0/24) on port 53, and failing that use a random port number on\n127.0.0.1.\n\n")
		fmt.Fprintf(out, "In the absence of -upstream, dnstweak will take the first nameserver configured\nin /etc/resolv.conf.\n\n")
		fmt.Fprintf(out, "Each SPEC is a hostname, followed by an \"=\" sign, followed by a\ncomma-separated list of 1 or more IP addresses (for example\n\"example.com=127.0.0.1\").\n\n")
		fmt.Fprintf(out, "dnstweak is a program by James Stanley. You can email me at\njames@incoherency.co.uk or read my blog at https://incoherency.co.uk/\n")
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

	dnstweak := DnsTweak{
		Upstream:             upstreamAddress,
		SpliceIntoResolvConf: !*noResolvConf,
		LookInProc:           !*noProc,
	}
	for _, spec := range flag.Args() {
		dnstweak.AddOverrideSpec(spec)
	}
	err := dnstweak.ListenAndServe(listenAddress)
	if err != nil {
		log.Fatal(err)
	}
}
