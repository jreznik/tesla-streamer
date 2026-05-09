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

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"tesla-streamer/internal/capture"
	"tesla-streamer/internal/config"
	"tesla-streamer/internal/server"
	webrtc_manager "tesla-streamer/internal/webrtc"

	"github.com/pion/webrtc/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	addr string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "tesla-streamer",
		Short: "Tesla Streamer - High performance screen streaming for Tesla browsers",
		Run:   run,
	}

	rootCmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "address to listen on")
	rootCmd.Flags().String("profile", "latency", "streaming profile (latency, quality, balanced)")
	rootCmd.Flags().String("resolution", "", "override resolution (e.g. 1280x720)")
	rootCmd.Flags().Int("bitrate", 0, "override bitrate in kbps")
	rootCmd.Flags().String("encoder", "x264", "encoder to use (x264, vaapi)")

	viper.BindPFlags(rootCmd.Flags())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	fmt.Println("!!! BACKEND KICKSTART SUCCESSFUL !!!")
	os.Stdout.Sync()

	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv := server.NewServer(addr)

	// Start DNS Spoofer for offline mode
	// Note: requires root to bind port 53. If it fails, we log it and continue.
	dns := server.NewDNSSpoofer("10.42.0.1")
	if err := dns.Start(); err != nil {
		log.Printf("Warning: Could not start DNS Spoofer (port 53): %v. Offline mode may be limited.", err)
	}

	// Create WebRTC manager
	rtc, err := webrtc_manager.NewWebRTCManager(func(candidate *webrtc.ICECandidate) {
		srv.SendMessage(map[string]interface{}{
			"type":      "candidate",
			"candidate": candidate.ToJSON(),
		})
	})
	if err != nil {
		log.Fatalf("Failed to create WebRTC manager: %v", err)
	}

	captureMgr := capture.NewCaptureManager(rtc, conf)

	// Control API
	srv.HandleFunc("/api/reselect", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Control API: Reselect requested")
		captureMgr.Reselect()
		w.WriteHeader(http.StatusOK)
	})

	srv.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "running",
		})
	})

	srv.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Profile    string `json:"profile"`
			Resolution string `json:"resolution"`
			Bitrate    int    `json:"bitrate"`
			Display    string `json:"display"`
			ShowStats  bool   `json:"show_stats"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update backend capture config
		newConf := capture.Config{
			Profile:    req.Profile,
			Resolution: req.Resolution,
			Bitrate:    req.Bitrate,
			Encoder:    viper.GetString("encoder"),
		}
		
		// Apply defaults for the chosen profile (handles 10000 bitrate for quality)
		config.ApplyProfileDefaults(&newConf)
		
		// Manual overrides if sent from GUI (non-zero/empty)
		if req.Resolution != "" {
			newConf.Resolution = req.Resolution
		}
		if req.Bitrate != 0 {
			newConf.Bitrate = req.Bitrate
		}

		captureMgr.UpdateConfig(newConf)

		// Forward display and stats commands to web app via WebSocket
		if req.Display != "" {
			srv.SendMessage(map[string]interface{}{
				"type": "display_config",
				"mode": req.Display,
			})
		}
		
		srv.SendMessage(map[string]interface{}{
			"type": "stats_config",
			"show": req.ShowStats,
		})

		w.WriteHeader(http.StatusOK)
	})

	// Start signaling server in background
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Handle signaling messages
	go func() {
		for msg := range srv.Messages() {
			var m map[string]interface{}
			if err := json.Unmarshal(msg, &m); err != nil {
				log.Printf("Failed to unmarshal signaling message: %v", err)
				continue
			}

			switch m["type"] {
			case "offer":
				log.Println("Received WebRTC offer from client")
				
				// Reset capture to ensure fresh headers for the new connection
				captureMgr.Reset()

				sdp := m["sdp"].(string)
				answerSDP, err := rtc.HandleOffer(sdp)
				if err != nil {
					log.Printf("Failed to handle WebRTC offer: %v", err)
					continue
				}
				log.Println("Sending WebRTC answer to client")
				srv.SendMessage(map[string]interface{}{
					"type": "answer",
					"sdp":  answerSDP,
				})
			case "candidate":
				log.Println("Received ICE candidate from client")
				candidateJSON, _ := json.Marshal(m["candidate"])
				if err := rtc.AddICECandidate(string(candidateJSON)); err != nil {
					log.Printf("Failed to add ICE candidate: %v", err)
				}
			default:
				log.Printf("Unknown signaling message type: %s", m["type"])
			}
		}
	}()

	// Graceful shutdown on signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	
	// Start capture initially
	captureMgr.Start()

	log.Println("Server is running. Control API active on /api/reselect")
	<-sig
	log.Println("Shutting down...")
	captureMgr.Stop()
	rtc.Close()
	os.Exit(0)
}
