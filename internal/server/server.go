package server

import (
	"encoding/json"
	"fmt"
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
	// Force logs to be unbuffered and immediate
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	fmt.Println("CRITICAL: Extreme Logging Initialized. Every request will be reported.")

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

	log.Printf("STARTING EXTREME SERVER ON %s", s.addr)
	
	// Create a listener to log raw connection attempts
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler: logger(mux),
	}

	return server.Serve(ln)
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
