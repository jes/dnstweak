package main

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func Test(t *testing.T) {
	// setup an upstream server answering for "upstream.example.com"
	upstreamHost := "upstream.example.com"
	upstreamIp := "1.2.3.4"
	upstreamAddress := "127.0.0.1:12345" // TODO: let linux pick the port
	upstream := makeDnsTweak("10.10.10.10:53", upstreamHost, upstreamIp)

	// setup a local server answering for "local.example.com"
	localHost := "local.example.com"
	localIp := "5.6.7.8"
	localAddress := "127.0.0.1:12346" // TODO: let linux pick the port
	local := makeDnsTweak(upstreamAddress, localHost, localIp)

	// no answers before we start the servers running
	checkNoAnswer(upstreamAddress, upstreamHost, "before starting", t)
	checkNoAnswer(localAddress, localHost, "before starting", t)

	// start the servers
	go runServer(upstream, upstreamAddress, t)
	go runServer(local, localAddress, t)

	// TODO: time is not a synchronisation primitive (take a channel or something to say when they started?)
	time.Sleep(1 * time.Second)

	// sensible answers after they start running
	checkAnswer(upstreamAddress, upstreamHost, upstreamIp, "own override", t)
	checkAnswer(localAddress, localHost, localIp, "own override", t)
	checkNoAnswer(upstreamAddress, localHost, "upstream doesn't answer for local", t)
	checkAnswer(localAddress, upstreamHost, upstreamIp, "local forwards to upstream", t)

	upstream.Finish()

	// no more upstream after upstream has finished
	checkNoAnswer(upstreamAddress, upstreamHost, "upstream has finished", t)
	checkAnswer(localAddress, localHost, localIp, "local still works after upstream has finished", t)
	checkNoAnswer(localAddress, upstreamHost, "local doesn't get a response from upstream after upstream has finished", t)

	local.Finish()

	// no more local after local has finished
	checkNoAnswer(localAddress, localHost, "local has finished", t)
}

func runServer(d *DnsTweak, listenAddress string, t *testing.T) {
	err := d.ListenAndServe(listenAddress)
	if err != nil {
		t.Errorf("%v", err)
	}
}

func dnsQuery(addr string, host string) (*dns.Msg, error) {
	req := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Opcode: dns.OpcodeQuery,
		},
		Question: make([]dns.Question, 1),
	}
	req.Question[0] = dns.Question{Name: dns.Fqdn(host), Qtype: dns.TypeA, Qclass: uint16(dns.ClassINET)}
	c := new(dns.Client)
	r, _, err := c.Exchange(req, addr)
	return r, err
}

func checkAnswer(addr string, host string, ip string, test string, t *testing.T) {
	r, err := dnsQuery(addr, host)
	if err != nil {
		t.Errorf("%s: (@%s: %s): expected success, got error: %v", test, addr, host, err)
		return
	}
	if len(r.Answer) != 1 {
		t.Errorf("%s: (@%s: %s): expected 1 answer, got %d: %v", test, addr, host, len(r.Answer), r.Answer)
		return
	}
	switch r.Answer[0].(type) {
	case *dns.A:
		ans := r.Answer[0].(*dns.A)
		if ans.A.String() != ip {
			t.Errorf("%s: (@%s: %s): expected %s, got %s: %v", test, addr, host, ip, ans.A, r.Answer)
		}
	default:
		t.Errorf("%s (@%s: %s): expected answer to be A record: %v", test, addr, host, r.Answer)
	}
}

func checkNoAnswer(addr string, host string, test string, t *testing.T) {
	r, err := dnsQuery(addr, host)
	if err == nil {
		t.Errorf("%s (@%s: %s): expected failure, got response: %v", test, addr, host, r.Answer)
	}
}

func makeDnsTweak(upstream string, host string, ip string) *DnsTweak {
	d := DnsTweak{
		Upstream:             upstream,
		SpliceIntoResolvConf: false,
		LookInProc:           false,
	}
	d.AddOverrideSpec(host + "=" + ip)
	return &d
}
