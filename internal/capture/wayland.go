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
	"math/rand"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalDest     = "org.freedesktop.portal.Desktop"
	portalPath     = "/org/freedesktop/portal/desktop"
	screenCastIface = "org.freedesktop.portal.ScreenCast"
)

type WaylandCapturer struct {
	conn   *dbus.Conn
	nodeID uint32
}

func NewWaylandCapturer() (*WaylandCapturer, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}
	return &WaylandCapturer{conn: conn}, nil
}

func (w *WaylandCapturer) Start() error {
	obj := w.conn.Object(portalDest, portalPath)

	// Pre-subscribe to signals to avoid race conditions
	c := make(chan *dbus.Signal, 10)
	w.conn.Signal(c)
	defer w.conn.RemoveSignal(c)

	// Create Session
	sessionToken := fmt.Sprintf("tesla_streamer_%d", rand.Intn(1000000))
	options := map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}

	var requestPath dbus.ObjectPath
	err := obj.Call(screenCastIface+".CreateSession", 0, options).Store(&requestPath)
	if err != nil {
		return fmt.Errorf("CreateSession failed: %w", err)
	}

	res, err := w.waitResponse(c, requestPath)
	if err != nil {
		return fmt.Errorf("CreateSession response failed: %w", err)
	}
	sessionHandle, ok := res["session_handle"].(string)
	if !ok {
		return fmt.Errorf("session_handle missing in CreateSession response")
	}
	sessionPath := dbus.ObjectPath(sessionHandle)

	// Select Sources
	// types: 1 = Monitor, 2 = Window. We use 3 (1|2) to allow both.
	options = map[string]dbus.Variant{
		"types": dbus.MakeVariant(uint32(3)), 
	}
	err = obj.Call(screenCastIface+".SelectSources", 0, sessionPath, options).Store(&requestPath)
	if err != nil {
		return fmt.Errorf("SelectSources failed: %w", err)
	}

	_, err = w.waitResponse(c, requestPath)
	if err != nil {
		return fmt.Errorf("SelectSources response failed: %w", err)
	}

	// Start
	options = map[string]dbus.Variant{}
	err = obj.Call(screenCastIface+".Start", 0, sessionPath, "", options).Store(&requestPath)
	if err != nil {
		return fmt.Errorf("Start failed: %w", err)
	}

	startRes, err := w.waitResponse(c, requestPath)
	if err != nil {
		return fmt.Errorf("Start response failed: %w", err)
	}

	log.Printf("Start response received: %+v", startRes)

	// Get Node ID from Start response
	var nodeID uint32

	streamsVal, ok := startRes["streams"]
	if !ok {
		return fmt.Errorf("streams key missing in Start response")
	}

	// Be extremely flexible with the type of 'streams'
	switch s := streamsVal.(type) {
	case [][]interface{}:
		if len(s) > 0 && len(s[0]) > 0 {
			nodeID, _ = s[0][0].(uint32)
		}
	case []interface{}:
		if len(s) > 0 {
			switch first := s[0].(type) {
			case []interface{}:
				if len(first) > 0 {
					nodeID, _ = first[0].(uint32)
				}
			case map[string]interface{}:
				// Handle some unusual decodings
				log.Printf("Notice: Stream 0 is a map: %+v", first)
			}
		}
	default:
		log.Printf("Notice: streams is unexpected type %T: %+v", s, s)
	}

	if nodeID == 0 {
		return fmt.Errorf("failed to extract PipeWire Node ID (streams type: %T)", streamsVal)
	}

	w.nodeID = nodeID
	log.Printf("Successfully started screencast. PipeWire Node ID: %d", w.nodeID)
	return nil
}

func (w *WaylandCapturer) waitResponse(c chan *dbus.Signal, path dbus.ObjectPath) (map[string]interface{}, error) {
	// Add match rule
	call := w.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		fmt.Sprintf("type='signal',sender='%s',path='%s',interface='org.freedesktop.portal.Request',member='Response'", portalDest, path))
	if call.Err != nil {
		return nil, call.Err
	}
	defer w.conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0,
		fmt.Sprintf("type='signal',sender='%s',path='%s',interface='org.freedesktop.portal.Request',member='Response'", portalDest, path))

	for {
		select {
		case sig := <-c:
			if sig.Path == path {
				// Response signal has two arguments: response code (uint32) and results (map[string]variant)
				code := sig.Body[0].(uint32)
				if code != 0 {
					return nil, fmt.Errorf("portal request failed with code %d", code)
				}
				results := sig.Body[1].(map[string]dbus.Variant)
				
				res := make(map[string]interface{})
				for k, v := range results {
					res[k] = v.Value()
				}
				return res, nil
			}
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("portal response timeout for path %s", path)
		}
	}
}

func (w *WaylandCapturer) Stop() error {
	if w.conn != nil {
		return w.conn.Close()
	}
	return nil
}

func (w *WaylandCapturer) NodeID() uint32 {
	return w.nodeID
}
