package server

import (
	"log"
	"net"

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
	// Listen on all interfaces for maximum reliability
	addr, err := net.ResolveUDPAddr("udp", ":53")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("DNS Spoofer CRITICAL: Failed to bind to :53. Is another DNS server running? Error: %v", err)
		return err
	}
	s.conn = conn

	log.Printf("DNS Spoofer ACTIVE on %s. All queries will resolve to %s", s.conn.LocalAddr().String(), s.targetIP)

	go func() {
		buf := make([]byte, 512)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("DNS READ ERROR: %v", err)
				break
			}

			var msg dnsmessage.Message
			if err := msg.Unpack(buf[:n]); err != nil {
				log.Printf("DNS UNPACK ERROR from %s: %v", remoteAddr, err)
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

	// Iterate all questions in the message
	for _, question := range msg.Questions {
		name := question.Name.String()
		log.Printf("DNS REQUEST [%s] from %s: %s", question.Type.String(), addr.String(), name)

		if question.Type != dnsmessage.TypeA {
			continue
		}

		// Spoof everything to our host IP
		ip := net.ParseIP(s.targetIP).To4()
		if ip == nil {
			continue
		}

		answer := dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  question.Name,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
				TTL:   10, // Short TTL for rapid testing
			},
			Body: &dnsmessage.AResource{A: [4]byte{ip[0], ip[1], ip[2], ip[3]}},
		}

		msg.Response = true
		msg.Authoritative = true
		msg.Answers = append(msg.Answers, answer)
		log.Printf("DNS SPOOFED [%s] -> %s", name, s.targetIP)
	}

	if len(msg.Answers) == 0 {
		return
	}

	packed, err := msg.Pack()
	if err != nil {
		log.Printf("DNS pack error: %v", err)
		return
	}

	s.conn.WriteToUDP(packed, addr)
}

func (s *DNSSpoofer) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
}
