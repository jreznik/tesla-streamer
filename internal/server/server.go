package server

import (
	"encoding/json"
	"log"
	"net"
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

type connLogger struct {
	net.Listener
}

func (l *connLogger) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		log.Printf("!!! RAW TCP CONNECTION !!! From: %s -> To: %s", c.RemoteAddr(), c.LocalAddr())
		os.Stdout.Sync()
	}
	return c, err
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	fileServer := http.FileServer(http.Dir("./static"))

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("!!! HTTP ACCESS !!! %s %s %s from %s", r.Method, r.Host, r.URL.Path, r.RemoteAddr)
		os.Stdout.Sync()

		// Priority 1: Check for known connectivity probes regardless of Host
		path := r.URL.Path
		isProbe := strings.Contains(path, "generate_204") || 
			strings.Contains(path, "gen_204") || 
			strings.Contains(path, "check_network_status") ||
			strings.Contains(path, "connecttest") ||
			strings.Contains(path, "hotspot-detect") ||
			strings.Contains(path, "success.txt") ||
			strings.Contains(path, "ncsi.txt")

		if isProbe {
			log.Printf("!!! SATISFYING CONNECTIVITY PROBE !!! -> 204")
			os.Stdout.Sync()
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("X-Tesla-Streamer-Spoof", "true")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Priority 2: Serve the app if it's the right Host
		if r.Host == "10.42.0.1" || r.Host == "10.42.0.1:8080" || r.Host == "localhost:8080" || strings.Contains(r.Host, "tesla.stream") {
			// Check if it's a root request to serve index.html
			if r.URL.Path == "/" {
				mux.ServeHTTP(w, r)
			} else {
				// Static file fallback
				fileServer.ServeHTTP(w, r)
			}
		} else {
			// Priority 3: Greedy hijack for random domains
			log.Printf("!!! GREEDY HIJACK (%s) !!! -> 204", r.Host)
			os.Stdout.Sync()
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.WriteHeader(http.StatusNoContent)
		}
	})

	log.Printf("TESLA STREAMER BACKEND READY ON %s", s.addr)
	os.Stdout.Sync()

	// Wrap the listener to log raw connections
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	loggedLn := &connLogger{ln}

	return http.Serve(loggedLn, mainHandler)
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
