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

// Config is the root configuration for ewwd.
type Config struct {
	Weather    WeatherConfig    `yaml:"weather"`
	Timer      TimerConfig      `yaml:"timer"`
	Audio      AudioConfig      `yaml:"audio"`
	Brightness BrightnessConfig `yaml:"brightness"`
	Date       DateConfig       `yaml:"date"`
	GPU        GPUConfig        `yaml:"gpu"`
	Network    NetworkConfig    `yaml:"network"`
}

// WeatherConfig holds OpenWeatherMap settings.
type WeatherConfig struct {
	APIKeyFile   string        `yaml:"api_key_file"`
	LocationFile string        `yaml:"location_file"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// TimerConfig holds timer/alarm defaults.
type TimerConfig struct {
	DefaultMinutes    int `yaml:"default_minutes"`
	DefaultAlarmHours int `yaml:"default_alarm_hours"`
	MinAlarmHours     int `yaml:"min_alarm_hours"`
}

// AudioConfig holds PulseAudio settings.
type AudioConfig struct {
	SourceOffset        int               `yaml:"source_offset"`
	SourceMax           int               `yaml:"source_max"`
	SinkMax             int               `yaml:"sink_max"`
	VolumeStep          int               `yaml:"volume_step"`
	PollInterval        time.Duration     `yaml:"poll_interval"`
	DefaultSinkVolume   int               `yaml:"default_sink_volume"`
	DefaultSourceVolume int               `yaml:"default_source_volume"`
	NameMappings        map[string]string `yaml:"name_mappings"`
}

// BrightnessConfig holds screen brightness settings.
type BrightnessConfig struct {
	Min     int `yaml:"min"`
	Max     int `yaml:"max"`
	Night   int `yaml:"night"`
	Default int `yaml:"default"`
}

// DateConfig holds date provider settings.
type DateConfig struct {
	BirthDate string `yaml:"birth_date"`
}

// GPUConfig holds GPU monitoring settings.
type GPUConfig struct {
	DevicePath   string        `yaml:"device_path"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// NetworkConfig holds network monitoring settings.
type NetworkConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
}

// Default returns the default configuration.
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

// Load loads configuration from ~/.config/ewwd/config.yaml.
// Falls back to defaults if file doesn't exist or has errors.
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

// ExpandPath expands ~ to home directory.
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
