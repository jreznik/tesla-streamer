package server

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
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
	// Standard output logging
	log.SetFlags(log.Ltime | log.Lmicroseconds)
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
	// Global logger middleware
	logger := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("!!! HTTP REQUEST DETECTED !!! Method=%s Host=%s Path=%s Remote=%s", r.Method, r.Host, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./static")))
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// Connectivity check handlers
	connectivityHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("!!! CAPTIVE PORTAL PROBE !!! %s", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}
	
	mux.HandleFunc("/generate_204", connectivityHandler)
	mux.HandleFunc("/gen_204", connectivityHandler)
	mux.HandleFunc("/check_network_status", connectivityHandler)
	mux.HandleFunc("/connecttest.txt", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("!!! MSFT CONNECT TEST !!!")
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Microsoft Connect Test"))
	})
	mux.HandleFunc("/hotspot-detect.html", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("!!! APPLE HOTSPOT DETECT !!!")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>"))
	})
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("!!! MANUAL PING !!!")
		w.Write([]byte("PONG - Server is reachable"))
	})

	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	// Listen on Port 80 AND the requested address (if different)
	go func() {
		log.Printf("Attempting to listen on Port 80 (Standard Web)...")
		ln, err := net.Listen("tcp", ":80")
		if err != nil {
			log.Printf("!!! PORT 80 BIND ERROR !!! %v", err)
			return
		}
		log.Printf("SUCCESS: Server active on Port 80")
		http.Serve(ln, logger(mux))
	}()

	log.Printf("Starting primary listener on %s", s.addr)
	return http.ListenAndServe(s.addr, logger(mux))
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()

	log.Printf("!!! NEW WEBSOCKET CLIENT !!! %s", r.RemoteAddr)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			s.mu.Lock()
			delete(s.clients, conn)
			s.mu.Unlock()
			log.Printf("WebSocket disconnected: %s", r.RemoteAddr)
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
