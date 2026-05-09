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
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

type GStreamerPipeline struct {
	pipeline *gst.Pipeline
	track    *webrtc.TrackLocalStaticSample
}

func NewGStreamerPipeline(nodeID uint32, track *webrtc.TrackLocalStaticSample, config Config) (*GStreamerPipeline, error) {
	gst.Init(nil)

	// Build pipeline string based on config
	encoder := "x264enc"
	encoderParams := fmt.Sprintf("tune=zerolatency bitrate=%d speed-preset=ultrafast key-int-max=30", config.Bitrate)
	
	if config.Encoder == "vaapi" {
		encoder = "vaapih264enc"
		encoderParams = fmt.Sprintf("bitrate=%d", config.Bitrate) 
	}

	scale := ""
	if config.Resolution != "" {
		res := config.Resolution
		// Parse width and height from "1280x800"
		parts := strings.Split(res, "x")
		if len(parts) == 2 {
			scale = fmt.Sprintf("! videoscale ! video/x-raw,width=%s,height=%s", parts[0], parts[1])
		}
	}

	source := fmt.Sprintf("pipewiresrc path=%d do-timestamp=true", nodeID)
	if nodeID == 0 {
		log.Println("No PipeWire Node ID provided, falling back to videotestsrc")
		source = "videotestsrc is-live=true ! video/x-raw,framerate=30/1"
	}

	// Optimized pipeline: source -> convert -> standardize to YUV -> scale -> force 30fps -> encode
	pipelineStr := fmt.Sprintf(
		"%s ! videoconvert ! video/x-raw,format=I420 %s ! videorate ! video/x-raw,framerate=30/1 ! videoconvert ! %s %s ! video/x-h264,profile=baseline,stream-format=byte-stream ! h264parse config-interval=-1 ! appsink name=sink",
		source, scale, encoder, encoderParams,
	)

	log.Printf("GStreamer pipeline: %s", pipelineStr)

	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return nil, err
	}

	// Add bus watch for errors
	pipeline.GetBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageError:
			err := msg.ParseError()
			log.Printf("GStreamer ERROR: %s", err.Error())
		case gst.MessageWarning:
			err := msg.ParseInfo()
			log.Printf("GStreamer WARNING: %s", err.Error())
		}
		return true
	})

	sinkElem, err := pipeline.GetElementByName("sink")
	if err != nil {
		return nil, err
	}
	sink := app.SinkFromElement(sinkElem)

	sampleCount := 0
	startTime := time.Now()
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(s *app.Sink) gst.FlowReturn {
			sample := s.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			// Push to WebRTC track
			data := buffer.Bytes()
			
			if sampleCount < 20 {
				log.Printf("SAMPLE #%d: Size=%d, T+%v", sampleCount, len(data), time.Since(startTime))
			}
			sampleCount++
			if sampleCount%100 == 0 {
				log.Printf("Pushed 100 samples to WebRTC track (last size: %d bytes)", len(data))
			}

			err := track.WriteSample(media.Sample{
				Data:     data,
				Duration: time.Duration(buffer.Duration()),
			})
			if err != nil {
				log.Printf("Failed to write sample to track: %v", err)
			}

			return gst.FlowOK
		},
	})

	return &GStreamerPipeline{
		pipeline: pipeline,
		track:    track,
	}, nil
}

func (p *GStreamerPipeline) Start() error {
	return p.pipeline.SetState(gst.StatePlaying)
}

func (p *GStreamerPipeline) Stop() error {
	return p.pipeline.SetState(gst.StateNull)
}
