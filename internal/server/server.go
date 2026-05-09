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

func (s *Server) Start() error {
	logger := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("!!! HTTP REQUEST !!! Method=%s Host=%s Path=%s Remote=%s", r.Method, r.Host, r.URL.Path, r.RemoteAddr)
			for k, v := range r.Header {
				log.Printf("  HEADER: %s = %v", k, v)
			}
			os.Stdout.Sync()
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	fileServer := http.FileServer(http.Dir("./static"))

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		isProbe := strings.Contains(path, "generate_204") || 
			strings.Contains(path, "gen_204") || 
			strings.Contains(path, "check_network_status") ||
			strings.Contains(path, "connecttest") ||
			strings.Contains(path, "hotspot-detect") ||
			strings.Contains(path, "success.txt")

		if isProbe {
			log.Printf("!!! SATISFYING CONNECTIVITY PROBE !!! -> 204")
			os.Stdout.Sync()
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Host == "10.42.0.1" || r.Host == "10.42.0.1:8080" || r.Host == "localhost:8080" || strings.Contains(r.Host, "tesla.stream") {
			mux.ServeHTTP(w, r)
		} else {
			log.Printf("!!! HIJACKING RANDOM DOMAIN REQUEST (%s) !!! -> 204", r.Host)
			os.Stdout.Sync()
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mux.Handle("/", fileServer)

	// Listen on Port 80 AND the requested address
	go func() {
		log.Printf("Attempting to bind directly to Port 80 (Standard Web)...")
		ln, err := net.Listen("tcp", ":80")
		if err != nil {
			log.Printf("!!! PORT 80 BIND ERROR !!!: %v (Use sudo setcap)", err)
			return
		}
		log.Printf("SUCCESS: Server listening on Port 80")
		os.Stdout.Sync()
		http.Serve(ln, logger(mainHandler))
	}()

	log.Printf("Starting primary listener on %s", s.addr)
	os.Stdout.Sync()
	return http.ListenAndServe(s.addr, logger(mainHandler))
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
