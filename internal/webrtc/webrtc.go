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

package webrtc

import (
	"encoding/json"
	"log"

	"github.com/pion/webrtc/v3"
)

type WebRTCManager struct {
	peerConnection *webrtc.PeerConnection
	videoTrack     *webrtc.TrackLocalStaticSample
	onICECandidate func(candidate *webrtc.ICECandidate)
}

func NewWebRTCManager(onICECandidate func(candidate *webrtc.ICECandidate)) (*WebRTCManager, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Create a video track
	// H264 payload type is usually 96, but Pion handles dynamic assignment
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	if err != nil {
		return nil, err
	}

	_, err = pc.AddTrack(videoTrack)
	if err != nil {
		return nil, err
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			log.Printf("New local ICE candidate: %s", c.String())
			if onICECandidate != nil {
				onICECandidate(c)
			}
		}
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("WebRTC Connection State has changed: %s\n", s.String())
	})

	pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		log.Printf("WebRTC ICE Connection State has changed: %s\n", s.String())
	})

	return &WebRTCManager{
		peerConnection: pc,
		videoTrack:     videoTrack,
		onICECandidate: onICECandidate,
	}, nil
}

func (m *WebRTCManager) HandleOffer(sdp string) (string, error) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	if err := m.peerConnection.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	answer, err := m.peerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	if err := m.peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	return answer.SDP, nil
}

func (m *WebRTCManager) AddICECandidate(candidateJSON string) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateJSON), &candidate); err != nil {
		return err
	}

	return m.peerConnection.AddICECandidate(candidate)
}

func (m *WebRTCManager) VideoTrack() *webrtc.TrackLocalStaticSample {
	return m.videoTrack
}

func (m *WebRTCManager) Close() error {
	return m.peerConnection.Close()
}
