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

	// 1. WebSocket Handler (High Priority)
	mux.HandleFunc("/ws", s.handleWebSocket)

	// 2. Control API Handlers
	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	// 3. Static Files
	fileServer := http.FileServer(http.Dir("./static"))

	// 4. Greedy Middleware & Router
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// LOG EVERYTHING
		log.Printf("!!! HTTP REQUEST !!! %s %s %s from %s", r.Method, r.Host, r.URL.Path, r.RemoteAddr)
		for k, v := range r.Header {
			log.Printf("  HEADER: %s = %v", k, v)
		}
		os.Stdout.Sync()

		// Is it a known captive portal check path?
		path := r.URL.Path
		isProbe := strings.Contains(path, "generate_204") || 
			strings.Contains(path, "gen_204") || 
			strings.Contains(path, "check_network_status") ||
			strings.Contains(path, "connecttest") ||
			strings.Contains(path, "hotspot-detect") ||
			strings.Contains(path, "success.txt")

		// If it's a probe, give them exactly what they want: 204 No Content
		if isProbe {
			log.Printf("!!! SATISFYING PROBE !!! -> 204")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// If it's a request for our actual app domain or IP, serve the files
		// Otherwise, if it's some random domain like 'www.google.com' (intercepted),
		// we return 204 to trick the background service into thinking internet is up.
		if r.Host == "10.42.0.1" || r.Host == "10.42.0.1:8080" || r.Host == "localhost:8080" || strings.Contains(r.Host, "tesla.stream") {
			mux.ServeHTTP(w, r) // Let ServeMux handle defined routes
		} else {
			// Random domain probe
			log.Printf("!!! DOMAIN HIJACK PROBE !!! (%s) -> 204", r.Host)
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Default fallback to file server
	mux.Handle("/", fileServer)

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
