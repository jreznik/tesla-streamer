package server

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

type DNSSpoofer struct {
	targetIP string
	conn     *net.UDPConn
	offline  bool
	mu       sync.Mutex
}

func NewDNSSpoofer(targetIP string) *DNSSpoofer {
	return &DNSSpoofer{
		targetIP: targetIP,
		offline:  false,
	}
}

func (s *DNSSpoofer) SetOffline(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offline = enabled
	log.Printf("DNS Spoofer Mode: Offline=%v", enabled)
}

func (s *DNSSpoofer) Start() error {
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

	log.Printf("DNS SPOOFER ALIVE on %s", s.conn.LocalAddr().String())
	os.Stdout.Sync()

	go func() {
		buf := make([]byte, 512)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}

			raw := make([]byte, n)
			copy(raw, buf[:n])

			var msg dnsmessage.Message
			if err := msg.Unpack(raw); err != nil {
				continue
			}

			go s.handleQuery(remoteAddr, msg, raw)
		}
	}()

	return nil
}

func (s *DNSSpoofer) handleQuery(addr *net.UDPAddr, msg dnsmessage.Message, raw []byte) {
	if len(msg.Questions) == 0 {
		return
	}

	s.mu.Lock()
	offline := s.offline
	s.mu.Unlock()

	shouldSpoof := false
	for _, question := range msg.Questions {
		name := question.Name.String()
		
		// Always spoof our specific domains
		if strings.Contains(name, "tesla.stream") {
			shouldSpoof = true
			break
		}
		
		// If in offline mode, we spoof everything to simulate internet
		if offline {
			shouldSpoof = true
			break
		}
	}

	if shouldSpoof {
		s.respondSpoofed(addr, msg)
	} else {
		// Online Mode: Forward to real DNS so car's internet doesn't break
		s.forwardQuery(addr, raw)
	}
}

func (s *DNSSpoofer) respondSpoofed(addr *net.UDPAddr, msg dnsmessage.Message) {
	for _, question := range msg.Questions {
		name := question.Name.String()
		log.Printf("DNS SPOOF: %s [%s]", name, question.Type.String())
		
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
			}
		}
		// AAAA is handled by returning an empty authoritative success (NOERROR)
	}

	packed, err := msg.Pack()
	if err != nil {
		return
	}
	s.conn.WriteToUDP(packed, addr)
}

func (s *DNSSpoofer) forwardQuery(addr *net.UDPAddr, raw []byte) {
	// Simple UDP proxy to Google DNS
	upstream, err := net.DialTimeout("udp", "8.8.8.8:53", 2*time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()

	_, err = upstream.Write(raw)
	if err != nil {
		return
	}

	upstream.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 512)
	n, err := upstream.Read(resp)
	if err != nil {
		return
	}

	s.conn.WriteToUDP(resp[:n], addr)
}

func (s *DNSSpoofer) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
}
