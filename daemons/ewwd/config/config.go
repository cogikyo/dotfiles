// Package config provides YAML-based configuration for ewwd.
//
// Configuration is loaded from ~/.config/ewwd/config.yaml. If the file does not
// exist or contains errors, sensible defaults are used. The Config struct
// contains subsections for each ewwd provider (weather, timer, audio, etc.).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config defines all provider-specific settings for ewwd daemon services.
type Config struct {
	Weather    WeatherConfig    `yaml:"weather"`    // OpenWeatherMap API integration
	Timer      TimerConfig      `yaml:"timer"`      // Timer and alarm defaults
	Audio      AudioConfig      `yaml:"audio"`      // PulseAudio volume control
	Brightness BrightnessConfig `yaml:"brightness"` // Screen brightness levels
	Date       DateConfig       `yaml:"date"`       // Date calculations (e.g., age)
	GPU        GPUConfig        `yaml:"gpu"`        // AMD GPU monitoring via sysfs
	Network    NetworkConfig    `yaml:"network"`    // Network interface monitoring
}

// WeatherConfig configures OpenWeatherMap API integration for live weather updates.
type WeatherConfig struct {
	APIKeyFile   string        `yaml:"api_key_file"`   // Path to file containing OWM API key
	LocationFile string        `yaml:"location_file"`  // Path to file containing lat,lon coordinates
	PollInterval time.Duration `yaml:"poll_interval"`  // How often to fetch weather data
}

// TimerConfig defines default durations and constraints for timer and alarm operations.
type TimerConfig struct {
	DefaultMinutes    int `yaml:"default_minutes"`      // Default timer duration in minutes
	DefaultAlarmHours int `yaml:"default_alarm_hours"`  // Default alarm time (hours from midnight)
	MinAlarmHours     int `yaml:"min_alarm_hours"`      // Minimum alarm time (hours from midnight)
}

// AudioConfig controls PulseAudio source/sink volumes with custom limits and aliases.
type AudioConfig struct {
	SourceOffset        int               `yaml:"source_offset"`         // Base offset added to source volume calculations
	SourceMax           int               `yaml:"source_max"`            // Maximum microphone volume percentage
	SinkMax             int               `yaml:"sink_max"`              // Maximum speaker volume percentage
	VolumeStep          int               `yaml:"volume_step"`           // Increment for volume adjustments
	PollInterval        time.Duration     `yaml:"poll_interval"`         // How often to poll PulseAudio state
	DefaultSinkVolume   int               `yaml:"default_sink_volume"`   // Initial speaker volume on reset
	DefaultSourceVolume int               `yaml:"default_source_volume"` // Initial microphone volume on reset
	NameMappings        map[string]string `yaml:"name_mappings"`         // Friendly aliases for device names
}

// BrightnessConfig defines preset brightness levels for common scenarios.
type BrightnessConfig struct {
	Min     int `yaml:"min"`     // Minimum allowed brightness
	Max     int `yaml:"max"`     // Maximum allowed brightness
	Night   int `yaml:"night"`   // Night mode brightness
	Default int `yaml:"default"` // Default startup brightness
}

// DateConfig provides reference dates for date-based calculations (e.g., age).
type DateConfig struct {
	BirthDate string `yaml:"birth_date"` // ISO 8601 date (YYYY-MM-DD) for age calculation
}

// GPUConfig enables AMD GPU monitoring via sysfs device nodes.
type GPUConfig struct {
	DevicePath   string        `yaml:"device_path"`   // Path to DRM device in sysfs (e.g., /sys/class/drm/card0/device)
	PollInterval time.Duration `yaml:"poll_interval"` // How often to read GPU metrics
}

// NetworkConfig controls polling frequency for network interface statistics.
type NetworkConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"` // How often to read network interface stats
}

// Default returns a config with sensible defaults for all providers.
func Default() *Config {
	return &Config{
		Weather: WeatherConfig{
			APIKeyFile:   "~/.local/.owm_api_key",
			LocationFile: "~/.local/.location",
			PollInterval: 60 * time.Second,
		},
		Timer: TimerConfig{
			DefaultMinutes:    90,
			DefaultAlarmHours: 6,
			MinAlarmHours:     3,
		},
		Audio: AudioConfig{
			SourceOffset:        50,
			SourceMax:           150,
			SinkMax:             100,
			VolumeStep:          10,
			PollInterval:        2 * time.Second,
			DefaultSinkVolume:   69,
			DefaultSourceVolume: 150,
			NameMappings:        map[string]string{"cullyn": "pixel buds"},
		},
		Brightness: BrightnessConfig{
			Min:     2,
			Max:     10,
			Night:   4,
			Default: 10,
		},
		Date: DateConfig{
			BirthDate: "1996-02-26",
		},
		GPU: GPUConfig{
			DevicePath:   "/sys/class/drm/card0/device",
			PollInterval: 500 * time.Millisecond,
		},
		Network: NetworkConfig{
			PollInterval: 1 * time.Second,
		},
	}
}

// Load reads config from ~/.config/ewwd/config.yaml, falling back to defaults on any error.
func Load() *Config {
	cfg := Default()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".config", "ewwd", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: config parse error: %v\n", err)
		return Default()
	}

	return cfg
}

// ExpandPath converts ~/ prefix to absolute home directory path, leaving other paths unchanged.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
