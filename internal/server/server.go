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
		log.Printf("!!! RAW TCP CONNECTION !!! From: %s", c.RemoteAddr())
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
		ua := r.Header.Get("User-Agent")
		host := r.Host
		path := r.URL.Path

		log.Printf("!!! HTTP ACCESS !!! %s %s %s [UA: %s]", r.Method, host, path, ua)
		os.Stdout.Sync()

		// 1. Identify Background System Services
		isSystem := strings.Contains(ua, "Dalvik") || 
			strings.Contains(ua, "CaptivePortal") || 
			strings.Contains(ua, "NetworkCheck") ||
			ua == ""

		// 2. Identify Connectivity Probes by Path
		isProbePath := strings.Contains(path, "generate_204") || 
			strings.Contains(path, "gen_204") || 
			strings.Contains(path, "check_network_status") ||
			strings.Contains(path, "success.txt") ||
			strings.Contains(path, "ncsi.txt") ||
			strings.Contains(path, "connecttest") ||
			strings.Contains(path, "hotspot-detect")

		// 3. Force Standard Online Responses
		// Set headers first
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Connection", "close")
		w.Header().Set("X-Android-Response", "204")

		if isSystem || isProbePath {
			// A. Microsoft NCSI Specific String
			if strings.Contains(path, "connecttest.txt") || strings.Contains(path, "ncsi.txt") {
				log.Printf("!!! SATISFYING NCSI PROBE !!! -> 200 OK")
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Microsoft Connect Test"))
				return
			}

			// B. Apple Success Specific HTML
			if strings.Contains(path, "hotspot-detect.html") {
				log.Printf("!!! SATISFYING APPLE PROBE !!! -> 200 OK")
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>"))
				return
			}

			// C. Universal 204 No Content for all other background probes
			log.Printf("!!! SATISFYING CONNECTIVITY CHECK !!! -> 204 No Content")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 4. Intentional App Access
		// Serve actual app for real browsers or intentional IP/Domain access
		isBrowser := strings.Contains(ua, "Mozilla") || strings.Contains(ua, "Chrome") || strings.Contains(ua, "Safari")
		isTargetHost := host == "10.42.0.1" || host == "10.42.0.1:8080" || strings.Contains(host, "tesla.stream")

		if isBrowser && isTargetHost {
			if path == "/" || strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
				mux.ServeHTTP(w, r)
			} else {
				fileServer.ServeHTTP(w, r)
			}
			return
		}

		// 5. Catch-all for hijacked domains (Greedy 204)
		// Convince background services that they hit the internet.
		log.Printf("!!! GREEDY HIJACK (%s) !!! -> 204 No Content", host)
		w.WriteHeader(http.StatusNoContent)
	})

	log.Printf("TESLA STREAMER BACKEND READY ON %s", s.addr)
	os.Stdout.Sync()

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
