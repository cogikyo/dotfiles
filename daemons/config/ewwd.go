package config

import "time"

// EwwConfig defines all provider-specific settings for the ewwd daemon.
type EwwConfig struct {
	Windows    []string         `yaml:"windows"`
	Weather    WeatherConfig    `yaml:"weather"`
	Timer      TimerConfig      `yaml:"timer"`
	Audio      AudioConfig      `yaml:"audio"`
	Brightness BrightnessConfig `yaml:"brightness"`
	Date       DateConfig       `yaml:"date"`
	GPU        GPUConfig        `yaml:"gpu"`
	Network    NetworkConfig    `yaml:"network"`
}

// WeatherConfig configures OpenWeatherMap API integration for live weather updates.
type WeatherConfig struct {
	APIKeyFile   string        `yaml:"api_key_file"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// TimerConfig defines default durations and constraints for timer and alarm operations.
type TimerConfig struct {
	DefaultMinutes    int `yaml:"default_minutes"`
	DefaultAlarmHours int `yaml:"default_alarm_hours"`
	MinAlarmHours     int `yaml:"min_alarm_hours"`
}

// AudioConfig controls PulseAudio source/sink volumes with custom limits and aliases.
type AudioConfig struct {
	SourceOffset        int               `yaml:"source_offset"`
	SourceMax           int               `yaml:"source_max"`
	SinkMax             int               `yaml:"sink_max"`
	VolumeStep          int               `yaml:"volume_step"`
	PollInterval        time.Duration     `yaml:"poll_interval"`
	DefaultSinkVolume   int               `yaml:"default_sink_volume"`
	DefaultSourceVolume int               `yaml:"default_source_volume"`
	NameMappings        map[string]string `yaml:"name_mappings"`
	BluetoothNames      []string          `yaml:"bluetooth_names"`
}

// BrightnessConfig defines preset brightness levels for common scenarios.
type BrightnessConfig struct {
	Min     int `yaml:"min"`
	Max     int `yaml:"max"`
	Night   int `yaml:"night"`
	Default int `yaml:"default"`
}

// DateConfig provides reference dates for date-based calculations.
type DateConfig struct {
	BirthDate string `yaml:"birth_date"`
}

// GPUConfig enables AMD GPU monitoring via sysfs device nodes.
type GPUConfig struct {
	DevicePath   string        `yaml:"device_path"`
	PollInterval time.Duration `yaml:"poll_interval"`
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
