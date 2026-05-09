// Tesla Streamer - High-performance screen streaming for Tesla browsers
// Copyright (C) 2026 Jaroslav Reznik
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	addr string
	// Simplified signaling: one peer for now
	mu         sync.Mutex
	peerConn   *websocket.Conn
	msgChannel chan []byte
}

func NewServer(addr string) *Server {
	return &Server{
		addr:       addr,
		msgChannel: make(chan []byte, 100),
	}
}

func (s *Server) Start() error {
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/ws", s.handleWebSocket)
	
	// Connectivity check handlers for Tesla/Chromium
	connectivityHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Connectivity check from %s: %s", r.RemoteAddr, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}
	
	http.HandleFunc("/generate_204", connectivityHandler)
	http.HandleFunc("/gen_204", connectivityHandler)
	http.HandleFunc("/check_network_status", connectivityHandler)
	http.HandleFunc("/connecttest.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Microsoft Connect Test"))
	})

	log.Printf("Starting server on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

// HandleFunc registers a new route to the server's multiplexer
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(pattern, handler)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket: %v", err)
		return
	}

	s.mu.Lock()
	if s.peerConn != nil {
		s.peerConn.Close()
	}
	s.peerConn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.peerConn == conn {
			s.peerConn = nil
		}
		s.mu.Unlock()
		conn.Close()
	}()

	log.Println("New client connected via WebSocket")

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
		s.msgChannel <- message
	}
}

// SendMessage sends a message to the connected peer
func (s *Server) SendMessage(msg interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.peerConn == nil {
		return nil // No client connected
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.peerConn.WriteMessage(websocket.TextMessage, data)
}

// Messages returns the channel for incoming signaling messages
func (s *Server) Messages() <-chan []byte {
	return s.msgChannel
}
