package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	// Robust static asset detection
	staticDir := "./static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Check relative to binary
		exePath, err := os.Executable()
		if err == nil {
			staticDir = filepath.Join(filepath.Dir(exePath), "static")
			if _, err := os.Stat(staticDir); os.IsNotExist(err) {
				// Check two levels up (for build/bin structure)
				staticDir = filepath.Join(filepath.Dir(filepath.Dir(exePath)), "static")
			}
		}
	}
	log.Printf("Using static assets from: %s", staticDir)
	fileServer := http.FileServer(http.Dir(staticDir))

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		host := r.Host
		path := r.URL.Path

		log.Printf("!!! HTTP ACCESS !!! %s %s %s [UA: %s]", r.Method, host, path, ua)
		os.Stdout.Sync()

		// Headers for all responses
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Connection", "close")

		// 1. ConnMan / Tesla WISPr Specific
		if strings.Contains(path, "status.html") {
			log.Printf("!!! SATISFYING CONNMAN PROBE !!! -> 200 OK 'online'")
			w.Header().Set("X-ConnMan-Status", "online")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>ConnMan Online Check</body></html>"))
			return
		}

		// 2. Microsoft / Android NCSI
		if strings.Contains(path, "connecttest.txt") || strings.Contains(path, "ncsi.txt") {
			log.Printf("!!! SATISFYING NCSI PROBE !!! -> 200 OK")
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Microsoft Connect Test"))
			return
		}

		// 3. Apple Detection
		if strings.Contains(path, "hotspot-detect.html") || strings.Contains(path, "success.html") {
			log.Printf("!!! SATISFYING APPLE PROBE !!! -> 200 OK")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>"))
			return
		}

		// 4. Identification of Background Probes
		isSystem := strings.Contains(ua, "Dalvik") || 
			strings.Contains(ua, "CaptivePortal") || 
			strings.Contains(ua, "NetworkCheck") ||
			strings.Contains(ua, "ConnMan") ||
			strings.Contains(ua, "wispr") ||
			ua == ""

		isProbePath := strings.Contains(path, "generate_204") || strings.Contains(path, "gen_204")

		if isProbePath || (isSystem && path == "/") {
			log.Printf("!!! SATISFYING CONNECTIVITY CHECK !!! -> 204 No Content")
			w.Header().Set("X-Android-Response", "204")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 5. Normal App Access
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

		// 6. Final Greedy Hijack (Random domain unresolved)
		log.Printf("!!! GREEDY HIJACK (%s) !!! -> 204 No Content", host)
		w.Header().Set("X-Tesla-Streamer-Spoof", "true")
		w.WriteHeader(http.StatusNoContent)
	})

	log.Printf("--------------------------------------------------")
	log.Printf("TESLA STREAMER BACKEND READY ON %s (User Mode)", s.addr)
	log.Printf("--------------------------------------------------")
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
