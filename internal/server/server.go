package server

import (
	"encoding/json"
	"log"
	"net/http"
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
			log.Printf("HTTP REQUEST: %s %s %s from %s", r.Method, r.Host, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("/", http.FileServer(http.Dir("./static")))
	
	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// Connectivity check handlers for Tesla/Chromium/Android/Apple
	connectivityHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	
	// Google / Android / Chrome
	mux.HandleFunc("/generate_204", connectivityHandler)
	mux.HandleFunc("/gen_204", connectivityHandler)
	// Gnome / Linux
	mux.HandleFunc("/check_network_status", connectivityHandler)
	// Microsoft
	mux.HandleFunc("/connecttest.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Microsoft Connect Test"))
	})
	// Apple
	mux.HandleFunc("/hotspot-detect.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>"))
	})
	// Catch all for any common portal paths
	mux.HandleFunc("/success.txt", connectivityHandler)
	mux.HandleFunc("/ncsi.txt", connectivityHandler)
	
	// Internal Ping for testing
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("PONG - Server is reachable"))
	})

	// Register API handlers from main
	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	log.Printf("Starting server on %s", s.addr)
	server := &http.Server{
		Addr:    s.addr,
		Handler: logger(mux),
	}
	return server.ListenAndServe()
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

	log.Printf("New client connected: %s", r.RemoteAddr)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			s.mu.Lock()
			delete(s.clients, conn)
			s.mu.Unlock()
			log.Printf("Client disconnected: %v", err)
			break
		}
		s.msgChan <- msg
	}
}

func (s *Server) SendMessage(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for client := range s.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Failed to send message: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) Messages() <-chan []byte {
	return s.msgChan
}
