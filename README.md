# dnstweak

**Quick local DNS modification tool**

**This is not implemented yet**

`dnstweak` will be a program that allows you to run something like:

    $ dnstweak foo.example.com=10.0.0.1,10.0.0.2

It will then launch a DNS server that passes all lookups except A lookups for
`foo.example.com` on to whatever was the previously-configured resolver in
`/etc/resolv.conf`, and inserts itself as the new resolver in
`/etc/resolv.conf`. Lookups for `foo.example.com` will get the 2 given A records
(`10.0.0.1` and `10.0.0.2`) in a random order. It will also log all requests and
responses to stdout. Once you stop it with Ctrl-C it will put the previous
`/etc/resolv.conf` contents back and exit.

It will also support usage like:

    $ dnstweak -f example.com.zone

To load the given zone file (in something resembling BIND's format) to give more
fine-grained control.

## Why?

Sometimes you want to test how a piece of software responds to different DNS
records without actually changing the real DNS records.

## Usage

    usage: dnstweak [-f ZONEFILE] SPEC...

    options:
        -f ZONEFILE    Load the given zonefile

    Each SPEC is a hostname, followed by an "=" sign, followed by a
    comma-separated list of 1 or more IP addresses.

## Build

`dnstweak` is written in go. You can build it with:

    $ go build

And then run:

    $ ./dnstweak

## Other tools like this

https://github.com/iphelix/dnschef

https://github.com/marekjelen/dnshack

## Contact

James Stanley

https://incoherency.co.uk

james@incoherency.co.uk
