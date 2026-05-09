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
	// Attempt to bind specifically to the hotspot IP
	// This avoids conflicts with system-wide DNS servers on 127.0.0.1
	addr, err := net.ResolveUDPAddr("udp", s.targetIP+":53")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("DNS Spoofer: Failed to bind to %s:53, trying :53...", s.targetIP)
		// Fallback to all interfaces
		addr, _ = net.ResolveUDPAddr("udp", ":53")
		conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return err
		}
	}
	s.conn = conn

	log.Printf("DNS Spoofer active on %s. Resolving all queries to %s", s.conn.LocalAddr().String(), s.targetIP)

	go func() {
		buf := make([]byte, 512)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("DNS read error: %v", err)
				break
			}

			var msg dnsmessage.Message
			if err := msg.Unpack(buf[:n]); err != nil {
				log.Printf("DNS unpack error: %v", err)
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
		log.Printf("DNS Query [%s]: %s from %s", question.Type.String(), name, addr.String())

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
				TTL:   60,
			},
			Body: &dnsmessage.AResource{A: [4]byte{ip[0], ip[1], ip[2], ip[3]}},
		}

		msg.Response = true
		msg.Authoritative = true
		msg.Answers = append(msg.Answers, answer)
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
