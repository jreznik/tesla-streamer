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
	// Bind to high port 5354. No root/setcap needed.
	// Firewall will REDIRECT 53 -> 5354
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:5354")
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
	log.Printf("DNS SPOOFER ACTIVE on %s (User Mode)", s.conn.LocalAddr().String())
	log.Printf("Redirecting all probes to %s", s.targetIP)
	log.Printf("--------------------------------------------------")
	os.Stdout.Sync()

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
		log.Printf("INCOMING DNS PROBE: %s [%s]", name, question.Type.String())
		os.Stdout.Sync()

		msg.Response = true
		msg.Authoritative = true

		if question.Type == dnsmessage.TypeA {
			ip := net.ParseIP(s.targetIP).To4()
			if ip != nil {
				answer := dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{
						Name:  question.Name,
						Type:  dnsmessage.TypeA,
						Class: dnsmessage.ClassINET,
						TTL:   5,
					},
					Body: &dnsmessage.AResource{A: [4]byte{ip[0], ip[1], ip[2], ip[3]}},
				}
				msg.Answers = append(msg.Answers, answer)
				log.Printf("SPOOFING ANSWER A -> %s", s.targetIP)
				os.Stdout.Sync()
			}
		} else if question.Type == dnsmessage.TypeAAAA {
			log.Printf("SPOOFING AAAA -> EMPTY (Force IPv4)")
			os.Stdout.Sync()
		}
	}

	if len(msg.Answers) == 0 && !msg.Response {
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
