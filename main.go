package main

import (
	"fmt"
	"log"

	"github.com/miekg/dns"
)

type MyHandler struct {
}

func (handler *MyHandler) ServeDNS(w dns.ResponseWriter, msg *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(msg)

	for _, question := range msg.Question {
		fmt.Printf("question: %v\n", question)
	}

	w.WriteMsg(response)
}

func main() {
	handler := MyHandler{}

	server := dns.Server{
		Addr:    "127.0.0.1:1053",
		Net:     "udp",
		Handler: &handler,
	}

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}
}
