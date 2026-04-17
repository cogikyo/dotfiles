package config

import "time"

// EwwConfig holds all provider-specific settings for the ewwd daemon.
type EwwConfig struct {
	Windows    []string         `yaml:"windows"` // eww window names to open on startup
	Weather    WeatherConfig    `yaml:"weather"`
	Timer      TimerConfig      `yaml:"timer"`
	Audio      AudioConfig      `yaml:"audio"`
	Date       DateConfig       `yaml:"date"`
	GPU        GPUConfig        `yaml:"gpu"`
	Network    NetworkConfig    `yaml:"network"`
}

// WeatherConfig configures the OpenWeatherMap polling loop.
type WeatherConfig struct {
	APIKeyFile   string        `yaml:"api_key_file"`  // path to plaintext file containing the OWM API key
	PollInterval time.Duration `yaml:"poll_interval"` // how often to refetch from the OWM API
}

// TimerConfig defines default durations and constraints for timer/alarm widgets.
type TimerConfig struct {
	DefaultMinutes    int `yaml:"default_minutes"`     // initial timer length in minutes
	DefaultAlarmHours int `yaml:"default_alarm_hours"` // initial alarm offset from now, in hours
	MinAlarmHours     int `yaml:"min_alarm_hours"`     // minimum allowed alarm offset, in hours
}

// AudioConfig controls PulseAudio source/sink volumes with per-device aliases.
//
// SourceMax may exceed 100 because PulseAudio allows source boost beyond
// unity gain; SinkMax is typically capped at 100 to avoid clipping.
type AudioConfig struct {
	SourceOffset        int               `yaml:"source_offset"`         // % subtracted from source volume before display
	SourceMax           int               `yaml:"source_max"`            // maximum source volume %, may exceed 100 (boost)
	SinkMax             int               `yaml:"sink_max"`              // maximum sink volume %, typically <= 100
	VolumeStep          int               `yaml:"volume_step"`           // % per up/down action
	PollInterval        time.Duration     `yaml:"poll_interval"`         // how often to refresh volume state
	DefaultSinkVolume   int               `yaml:"default_sink_volume"`   // sink volume % applied on startup
	DefaultSourceVolume int               `yaml:"default_source_volume"` // source volume % applied on startup
	NameMappings        map[string]string `yaml:"name_mappings"`         // raw device name -> display alias
	BluetoothNames      []string          `yaml:"bluetooth_names"`       // substrings identifying bluetooth devices for icon selection
}

// DateConfig provides reference dates for date-based widgets (age, countdowns).
type DateConfig struct {
	BirthDate string `yaml:"birth_date"` // ISO-8601 date
}

// GPUConfig enables AMD GPU monitoring via sysfs.
type GPUConfig struct {
	DevicePath   string        `yaml:"device_path"`   // sysfs root, e.g. /sys/class/drm/card0/device
	PollInterval time.Duration `yaml:"poll_interval"` // how often to sample GPU stats
}

// NetworkConfig controls polling frequency for network interface statistics.
type NetworkConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
}

// DefaultEww returns sensible defaults for the ewwd daemon.
func DefaultEww() EwwConfig {
	return EwwConfig{
		Windows: []string{"today", "workspaces", "computer", "music", "player"},
		Weather: WeatherConfig{
			APIKeyFile:   "~/.local/.owm_api_key",
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
			BluetoothNames:      []string{"WH-1000XM4", "OpenFit", "pixel buds"},
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
