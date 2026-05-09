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
			log.Printf("!!! HTTP ACCESS !!! %s %s %s from %s", r.Method, r.Host, r.URL.Path, r.RemoteAddr)
			os.Stdout.Sync()
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./static")))
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	connectivityHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("!!! PORTAL PROBE SUCCESS !!! %s", r.URL.Path)
		os.Stdout.Sync()
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
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

	for pattern, handler := range s.handlers {
		mux.HandleFunc(pattern, handler)
	}

	log.Printf("--------------------------------------------------")
	log.Printf("TESLA STREAMER BACKEND STARTING ON %s", s.addr)
	log.Printf("--------------------------------------------------")
	os.Stdout.Sync()

	// Raw connection logger
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	go func() {
		for {
			// We can't easily wrap net.Listener to log without more boilerplate,
			// but http.Server will call our logger middleware.
		}
	}()

	return http.Serve(ln, logger(mux))
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

	log.Printf("NEW WEBSOCKET CLIENT: %s", r.RemoteAddr)
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
