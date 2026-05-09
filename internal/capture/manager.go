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

package capture

import (
	"log"
	"sync"
	"tesla-streamer/internal/webrtc"
)

type CaptureManager struct {
	rtc        *webrtc.WebRTCManager
	conf       Config
	mu         sync.Mutex
	wayland    *WaylandCapturer
	pipeline   *GStreamerPipeline
	nodeID     uint32
	isStarting bool
}

func NewCaptureManager(rtc *webrtc.WebRTCManager, conf Config) *CaptureManager {
	return &CaptureManager{
		rtc:  rtc,
		conf: conf,
	}
}

// Start initiates the Wayland handshake (prompts user) and then starts the pipeline
func (m *CaptureManager) Start() {
	m.mu.Lock()
	if m.isStarting {
		m.mu.Unlock()
		return
	}
	m.isStarting = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.isStarting = false
			m.mu.Unlock()
		}()

		log.Println("Initializing Wayland capture handshake...")
		wayland, err := NewWaylandCapturer()
		if err != nil {
			log.Printf("Warning: Failed to initialize Wayland capturer: %v", err)
			m.startPipeline(0, nil)
			return
		}

		if err := wayland.Start(); err != nil {
			log.Printf("Warning: Failed to start Wayland screencast: %v", err)
			wayland.Stop()
			m.startPipeline(0, nil)
			return
		}

		m.startPipeline(wayland.NodeID(), wayland)
	}()
}

func (m *CaptureManager) startPipeline(nodeID uint32, wayland *WaylandCapturer) {
	log.Println("Initializing GStreamer pipeline...")
	pipeline, err := NewGStreamerPipeline(nodeID, m.rtc.VideoTrack(), m.conf)
	if err != nil {
		log.Printf("Error: Failed to create GStreamer pipeline: %v", err)
		if wayland != nil {
			wayland.Stop()
		}
		return
	}

	if err := pipeline.Start(); err != nil {
		log.Printf("Error: Failed to start GStreamer pipeline: %v", err)
		pipeline.Stop()
		if wayland != nil {
			wayland.Stop()
		}
		return
	}

	m.mu.Lock()
	// Clean up old session if any
	if m.pipeline != nil {
		m.pipeline.Stop()
	}
	if m.wayland != nil && m.wayland != wayland {
		m.wayland.Stop()
	}
	m.pipeline = pipeline
	m.wayland = wayland
	m.nodeID = nodeID
	m.mu.Unlock()

	log.Println("Streaming is ready.")
}

func (m *CaptureManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pipeline != nil {
		m.pipeline.Stop()
		m.pipeline = nil
	}
	if m.wayland != nil {
		m.wayland.Stop()
		m.wayland = nil
	}
}

func (m *CaptureManager) Reselect() {
	log.Println("Reselecting capture source...")
	m.Start()
}

func (m *CaptureManager) Reset() {
	m.mu.Lock()
	if m.isStarting || m.pipeline == nil {
		m.mu.Unlock()
		return
	}
	
	nodeID := m.nodeID
	wayland := m.wayland
	m.mu.Unlock()

	log.Println("Resetting stream pipeline for new peer (keeping current source)...")
	m.startPipeline(nodeID, wayland)
}

func (m *CaptureManager) UpdateConfig(conf Config) {
	m.mu.Lock()
	m.conf = conf
	nodeID := m.nodeID
	wayland := m.wayland
	m.mu.Unlock()

	log.Printf("Applying new configuration: %+v", conf)
	m.startPipeline(nodeID, wayland)
}
