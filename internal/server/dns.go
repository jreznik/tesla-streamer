package server

import (
	"log"
	"net"
	"time"

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
	// Use a non-privileged port to avoid conflicts with dnsmasq/systemd-resolved
	// We will use firewalld to redirect traffic from 53 to 5353
	addr, err := net.ResolveUDPAddr("udp", ":5353")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.conn = conn

	log.Printf("DNS Spoofer ACTIVE on %s. Forwarding traffic to %s", s.conn.LocalAddr().String(), s.targetIP)

	// Heartbeat to ensure logs are flowing and process is alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			log.Println("HEARTBEAT: DNS Spoofer and Web Server are running...")
		}
	}()

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
				log.Printf("DNS UNPACK ERROR: %v", err)
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
		log.Printf("DNS PROBE [%s] from %s: %s", question.Type.String(), addr.String(), name)

		if question.Type != dnsmessage.TypeA {
			continue
		}

		// Use detected target IP
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
		log.Printf("DNS HIJACK: %s -> %s", name, s.targetIP)
	}

	if len(msg.Answers) == 0 {
		return
	}

	packed, err := msg.Pack()
	if err != nil {
		log.Printf("DNS PACK ERROR: %v", err)
		return
	}

	s.conn.WriteToUDP(packed, addr)
}

func (s *DNSSpoofer) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
}
