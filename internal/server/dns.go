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
	// Bind DIRECTLY to port 53.
	// We use 0.0.0.0 to catch all incoming queries on all interfaces.
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:53")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("!!! DNS BIND ERROR !!! Could not listen on port 53: %v", err)
		log.Printf("HINT: Run 'sudo setcap cap_net_bind_service=+ep ./tesla-streamer'")
		return err
	}
	s.conn = conn

	log.Printf("DNS SPOOFER ACTIVE on %s. All names -> %s", s.conn.LocalAddr().String(), s.targetIP)

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
		
		// ALWAYS LOG EVERY PROBE
		log.Printf("!!! DNS PROBE DETECTED !!! From=%s Name=%s Type=%s", addr.String(), name, question.Type.String())

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
				TTL:   10,
			},
			Body: &dnsmessage.AResource{A: [4]byte{ip[0], ip[1], ip[2], ip[3]}},
		}

		msg.Response = true
		msg.Authoritative = true
		msg.Answers = append(msg.Answers, answer)
		log.Printf("!!! DNS SPOOFED !!! %s -> %s", name, s.targetIP)
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
