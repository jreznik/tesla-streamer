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

type ScreenCapturer interface {
	Start() error
	Stop() error
	// For now, we'll let GStreamer handle the encoding and push directly to Pion,
	// so the interface might just need to manage the lifecycle.
}

type Config struct {
	Resolution string
	FPS        int
	Bitrate    int
	Encoder    string
	Profile    string
}
