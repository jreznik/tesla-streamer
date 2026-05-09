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

package config

import (
	"strings"

	"tesla-streamer/internal/capture"

	"github.com/spf13/viper"
)

func LoadConfig() (capture.Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("TESLA")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("profile", "latency")
	viper.SetDefault("encoder", "x264")
	viper.SetDefault("fps", 30)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return capture.Config{}, err
		}
	}

	conf := capture.Config{
		Profile: viper.GetString("profile"),
		Encoder: viper.GetString("encoder"),
		FPS:     viper.GetInt("fps"),
	}

	// Apply profile defaults
	ApplyProfileDefaults(&conf)

	// Manual Overrides
	if viper.IsSet("resolution") {
		conf.Resolution = viper.GetString("resolution")
	}
	if viper.IsSet("bitrate") {
		conf.Bitrate = viper.GetInt("bitrate")
	}

	return conf, nil
}

func ApplyProfileDefaults(conf *capture.Config) {
	switch strings.ToLower(conf.Profile) {
	case "latency":
		conf.Bitrate = 2000
		conf.Resolution = "1280x800"
	case "quality":
		conf.Bitrate = 10000 // High quality as requested
		conf.Resolution = ""    // Native
	case "balanced":
		conf.Bitrate = 5000
		conf.Resolution = "1280x800"
	default:
		// Default to latency
		conf.Bitrate = 2000
		conf.Resolution = "1280x800"
	}
}
