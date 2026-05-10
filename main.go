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
	// Massive unbuffered banner to prove execution
	fmt.Println("##################################################")
	fmt.Println("!!! TESLA STREAMER BACKEND BOOTING UP !!!")
	fmt.Println("##################################################")
	os.Stdout.Sync()

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
		fmt.Printf("FATAL EXECUTION ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	fmt.Println("!!! RUNNING CORE ENGINE !!!")
	os.Stdout.Sync()

	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srv := server.NewServer(addr)

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

	// Keep DNS reference for mode updates
	dns := server.NewDNSSpoofer("10.42.0.1")
	if err := dns.Start(); err != nil {
		log.Printf("Warning: Could not start DNS Spoofer: %v", err)
	}

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
			Profile     string `json:"profile"`
			Resolution  string `json:"resolution"`
			Bitrate     int    `json:"bitrate"`
			Display     string `json:"display"`
			ShowStats   bool   `json:"show_stats"`
			OfflineMode bool   `json:"offline_mode"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update DNS mode
		dns.SetOffline(req.OfflineMode)

		newConf := capture.Config{
			Profile:    req.Profile,
			Resolution: req.Resolution,
			Bitrate:    req.Bitrate,
			Encoder:    viper.GetString("encoder"),
		}
		config.ApplyProfileDefaults(&newConf)
		
		if req.Resolution != "" {
			newConf.Resolution = req.Resolution
		}
		if req.Bitrate != 0 {
			newConf.Bitrate = req.Bitrate
		}

		captureMgr.UpdateConfig(newConf)

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
				continue
			}

			switch m["type"] {
			case "offer":
				captureMgr.Reset()
				sdp := m["sdp"].(string)
				answerSDP, err := rtc.HandleOffer(sdp)
				if err != nil {
					continue
				}
				srv.SendMessage(map[string]interface{}{
					"type": "answer",
					"sdp":  answerSDP,
				})
			case "candidate":
				candidateJSON, _ := json.Marshal(m["candidate"])
				rtc.AddICECandidate(string(candidateJSON))
			}
		}
	}()

	// Graceful shutdown on signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	
	// Start capture initially
	captureMgr.Start()

	<-sig
	log.Println("Shutting down...")
	captureMgr.Stop()
	rtc.Close()
	os.Exit(0)
}
