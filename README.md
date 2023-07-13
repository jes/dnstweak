# dnstweak

**Quick local DNS spoofing tool**

`dnstweak` is a program that allows you to run something like:

    $ sudo ./dnstweak foo.example.com=10.0.0.1,10.0.0.2

So that `foo.example.com` appears to resolve to `10.0.0.1` and `10.0.0.2`.

It runs a DNS server that proxies all other lookups to the previous resolver from
`/etc/resolv.conf`, and it inserts itself as the new resolver in
`/etc/resolv.conf`.

Lookups for `foo.example.com` will get the 2 given A records
(`10.0.0.1` and `10.0.0.2`) in a random order. It also logs requests and
responses to stdout. Once you stop it with Ctrl-C it will put the previous
`/etc/resolv.conf` contents back in place.

See the [Releases](https://github.com/jes/dnstweak/releases/) page for Linux
binaries, or build it yourself with `go build`.

## Features

`dnstweak` can:

 - forge responses to select DNS requests configured on the command line
 - automatically take over all system DNS queries by inserting itself into `/etc/resolv.conf`
 - lookup client addresses in `procfs` to find out which process a request came from

## Why?

Sometimes you want to test how a piece of software responds to different DNS
records without actually changing the real DNS records.

## Why not?

If it crashes it will trash your `/etc/resolv.conf`. I recommend taking a backup
copy of `/etc/resolv.conf` before you start.

## Example session

Open 2 terminal windows. We'll run `dnstweak` in the first and `ping` in
the second.

First terminal:

    $ sudo ./dnstweak example.com=127.0.0.1
    2023/07/11 20:41:35 dnstweak starts
    2023/07/11 20:41:35 listening on 127.0.0.1:53
    2023/07/11 20:41:35 using 127.0.0.53:53 as upstream resolver

`dnstweak` stays running. It has inserted itself as the local resolver by
modifying `/etc/resolv.conf`.

If you run `dnstweak` as a non-root user, it will still work as a DNS server,
but it won't be able to listen on port 53 or insert itself into `/etc/resolv.conf`.

In the other terminal, start a `ping` to `example.com`:

    $ ping example.com

Now we get some log output from `dnstweak` in our first terminal:

    2023/07/11 20:41:37 127.0.0.1:48439 (ping/9975): A example.com: 127.0.0.1 (overridden)

A request came from `127.0.0.1:48439`, which is a process called `ping` with
PID 9975.
It was an A lookup for `example.com`, and we returned `127.0.0.1`. Back in the other terminal:

    PING example.com (127.0.0.1) 56(84) bytes of data.
    64 bytes from localhost (127.0.0.1): icmp_seq=1 ttl=64 time=0.020 ms
    64 bytes from localhost (127.0.0.1): icmp_seq=2 ttl=64 time=0.037 ms

Great success.

## Installation

Either get a binary from the [Releases](https://github.com/jes/dnstweak/releases/) page, and:

    $ sudo cp dnstweak.x86_64 /usr/bin/dnstweak
    $ sudo chmod +x /usr/bin/dnstweak

Or build it yourself (see below).

## Usage

    dnstweak v0.1

    usage: dnstweak [options] SPEC...

    options:
      -listen string
            listen address (IP:PORT or just PORT) (default: see below)
      -no-proc
            disable discovering the client process by looking in /proc
      -no-resolvconf
            disable automatic update of /etc/resolv.conf
      -upstream string
            upstream DNS server (IP:PORT or just IP) (default: see below)

    In the absence of -listen, dnstweak will first try to listen on any loopback IP
    address (127.0.0.0/24) on port 53, and failing that use a random port number on
    127.0.0.1.

    In the absence of -upstream, dnstweak will take the first nameserver configured
    in /etc/resolv.conf.

    Each SPEC is a hostname, followed by an "=" sign, followed by a
    comma-separated list of 1 or more IP addresses (for example
    "example.com=127.0.0.1").

    dnstweak is a program by James Stanley. You can email me at
    james@incoherency.co.uk or read my blog at https://incoherency.co.uk/

## Build

`dnstweak` is written in go. You can build it with:

    $ go build

And then run:

    $ ./dnstweak -help

Run the tests with:

    $ go test

## Future

In the future, maybe `dnstweak` will gain options to:

 - listen on IPv6
 - populate the DNS override map from `/etc/hosts`
 - override responses to AAAA, CNAME, PTR, SRV requests
 - make fake NXDOMAIN responses
 - drop requests
 - take a zonefile in a BIND-ish format instead of the made-up command-line format

## Other tools like this

https://github.com/iphelix/dnschef

https://github.com/marekjelen/dnshack

## Contact

James Stanley

https://incoherency.co.uk

james@incoherency.co.uk
