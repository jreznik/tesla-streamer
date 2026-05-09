package server

import (
	"log"
	"net"
	"os"

	"golang.org/x/net/dns/dnsmessage"
)

type DNSSpoofer struct {
	targetIP string
	conn     *net.UDPConn
}

func NewDNSSpoofer(targetIP string) *DNSSpoofer {
	return &DNSSpoofer{targetIP: targetIP}
}

func (s *DNSSpoofer) Start() error {
	// Listen on 5353 to avoid conflict with dnsmasq (which NM starts on 53)
	// We will use iptables REDIRECT to send 53 -> 5353
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:5353")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("!!! DNS STARTUP ERROR !!!: %v", err)
		return err
	}
	s.conn = conn

	log.Printf("--------------------------------------------------")
	log.Printf("DNS SPOOFER ALIVE on %s", s.conn.LocalAddr().String())
	log.Printf("All incoming queries will resolve to %s", s.targetIP)
	log.Printf("--------------------------------------------------")
	os.Stdout.Sync() // Force flush

	go func() {
		buf := make([]byte, 512)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}

			var msg dnsmessage.Message
			if err := msg.Unpack(buf[:n]); err != nil {
				continue
			}

			go s.handleQuery(remoteAddr, msg)
		}
	}()

	return nil
}

func (s *DNSSpoofer) handleQuery(addr *net.UDPAddr, msg dnsmessage.Message) {
	if len(msg.Questions) == 0 {
		return
	}

	for _, question := range msg.Questions {
		name := question.Name.String()
		log.Printf("INCOMING DNS PROBE: %s from %s", name, addr.String())
		os.Stdout.Sync()

		if question.Type != dnsmessage.TypeA {
			continue
		}

		ip := net.ParseIP(s.targetIP).To4()
		if ip == nil {
			continue
		}

		answer := dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  question.Name,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
				TTL:   5,
			},
			Body: &dnsmessage.AResource{A: [4]byte{ip[0], ip[1], ip[2], ip[3]}},
		}

		msg.Response = true
		msg.Authoritative = true
		msg.Answers = append(msg.Answers, answer)
		log.Printf("SPOOFING ANSWER: %s -> %s", name, s.targetIP)
		os.Stdout.Sync()
	}

	if len(msg.Answers) == 0 {
		return
	}

	packed, err := msg.Pack()
	if err != nil {
		return
	}

	s.conn.WriteToUDP(packed, addr)
}

func (s *DNSSpoofer) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
}
