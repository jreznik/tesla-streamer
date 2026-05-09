package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Server struct {
	addr     string
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	mu       sync.Mutex
	msgChan  chan []byte
	handlers map[string]http.HandlerFunc
}

func NewServer(addr string) *Server {
	log.SetOutput(os.Stdout)
	return &Server{
		addr: addr,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients:  make(map[*websocket.Conn]bool),
		msgChan:  make(chan []byte, 100),
		handlers: make(map[string]http.HandlerFunc),
	}
}

func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.handlers[pattern] = handler
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	fileServer := http.FileServer(http.Dir("./static"))

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		log.Printf("!!! HTTP ACCESS !!! %s %s %s [UA: %s]", r.Method, r.Host, r.URL.Path, ua)
		
		// EXHAUSTIVE HEADER LOGGING
		for k, v := range r.Header {
			log.Printf("  HEADER [%s] = %v", k, v)
		}
		os.Stdout.Sync()

		// Anti-Cache Headers (Universal)
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("X-Tesla-Streamer-Spoof", "true")
		w.Header().Set("Connection", "close")

		path := r.URL.Path

		// 1. Identify System Probes
		isSystem := strings.Contains(ua, "Dalvik") || 
			strings.Contains(ua, "CaptivePortal") || 
			strings.Contains(ua, "NetworkCheck") ||
			ua == ""

		// 2. Identify Known Probe Paths
		isProbePath := strings.Contains(path, "generate_204") || 
			strings.Contains(path, "gen_204") || 
			strings.Contains(path, "check_network_status") ||
			strings.Contains(path, "connecttest") ||
			strings.Contains(path, "hotspot-detect") ||
			strings.Contains(path, "success.txt") ||
			strings.Contains(path, "success.html")

		// 3. Satisfy Probes
		if isSystem || isProbePath {
			// Specific handling for root path probes from system (e.g. hitting gateway IP directly)
			if path == "/" {
				log.Printf("!!! SATISFYING SYSTEM ROOT PROBE !!! -> 200 OK 'success'")
				os.Stdout.Sync()
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success\n"))
				return
			}

			// Specific handling for Microsoft / Android NCSI strings
			if strings.Contains(path, "connecttest.txt") || strings.Contains(path, "ncsi.txt") {
				log.Printf("!!! SATISFYING NCSI PROBE !!! -> 200 OK 'Microsoft Connect Test'")
				os.Stdout.Sync()
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Microsoft Connect Test"))
				return
			}

			// Standard 204 success for all other probes
			log.Printf("!!! SATISFYING PROBE PATH !!! -> 204 No Content")
			os.Stdout.Sync()
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 4. Intentional App Access
		// Check for real browser User-Agent
		isBrowser := strings.Contains(ua, "Mozilla") || strings.Contains(ua, "Chrome") || strings.Contains(ua, "Safari")

		if isBrowser && (r.Host == "10.42.0.1" || r.Host == "10.42.0.1:8080" || r.Host == "localhost:8080" || strings.Contains(r.Host, "tesla.stream")) {
			// Root request = serve app
			if path == "/" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
				mux.ServeHTTP(w, r)
			} else {
				fileServer.ServeHTTP(w, r)
			}
			return
		}

		// 5. Greedy Hijack (Random domains hijacked by DNS)
		// We return 204 to trick background internet checks that resolve random names.
		log.Printf("!!! GREEDY HIJACK (%s) !!! -> 204 No Content", r.Host)
		os.Stdout.Sync()
		w.WriteHeader(http.StatusNoContent)
	})

	log.Printf("TESLA STREAMER BACKEND READY ON %s", s.addr)
	os.Stdout.Sync()

	return http.ListenAndServe(s.addr, mainHandler)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()

	log.Printf("WEBSOCKET CONNECTED: %s", r.RemoteAddr)
	os.Stdout.Sync()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			s.mu.Lock()
			delete(s.clients, conn)
			s.mu.Unlock()
			break
		}
		s.msgChan <- msg
	}
}

func (s *Server) SendMessage(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *Server) Messages() <-chan []byte {
	return s.msgChan
}
