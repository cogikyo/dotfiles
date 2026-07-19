package config

// ewwd.go declares ewwd provider settings and their default values.
import "time"

// EwwConfig holds settings for every ewwd provider.
type EwwConfig struct {
	Windows []string      `yaml:"windows"` // eww window names to open on startup
	Weather WeatherConfig `yaml:"weather"`
	Timer   TimerConfig   `yaml:"timer"`
	Audio   AudioConfig   `yaml:"audio"`
	Date    DateConfig    `yaml:"date"`
	Network NetworkConfig `yaml:"network"`
	Music   MusicConfig   `yaml:"music"`
}

// MusicConfig configures the music provider's Spotify Canvas integration.
type MusicConfig struct {
	SpDc string `yaml:"sp_dc"` // open.spotify.com session cookie for Canvas API auth
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

// AudioConfig controls WirePlumber volume with per-device aliases.
type AudioConfig struct {
	SourceMax           int               `yaml:"source_max"`            // maximum source volume %, may exceed 100 (boost)
	SinkMax             int               `yaml:"sink_max"`              // maximum sink volume %, typically <= 100
	VolumeStep          int               `yaml:"volume_step"`           // % per up/down action
	DefaultSinkVolume   int               `yaml:"default_sink_volume"`   // sink volume % applied by reset_volume
	DefaultSourceVolume int               `yaml:"default_source_volume"` // source volume % applied by reset_volume
	NameMappings        map[string]string `yaml:"name_mappings"`         // stable node.name -> display alias
}

// DateConfig provides reference dates for date-based widgets (age, countdowns).
type DateConfig struct {
	BirthDate string `yaml:"birth_date"` // ISO-8601 date
}

// NetworkConfig controls polling frequency for network interface statistics.
type NetworkConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
}

// DefaultEww returns ewwd defaults.
func DefaultEww() EwwConfig {
	return EwwConfig{
		Windows: []string{"today", "workspaces", "computer", "music", "player"},
		Weather: WeatherConfig{
			APIKeyFile:   "~/.local/.owm_api_key",
			PollInterval: time.Hour,
		},
		Timer: TimerConfig{
			DefaultMinutes:    90,
			DefaultAlarmHours: 6,
			MinAlarmHours:     3,
		},
		Audio: AudioConfig{
			SourceMax:           150,
			SinkMax:             100,
			VolumeStep:          10,
			DefaultSinkVolume:   69,
			DefaultSourceVolume: 150,
			NameMappings:        map[string]string{},
		},
		Date: DateConfig{
			BirthDate: "1996-02-26",
		},
		Network: NetworkConfig{
			PollInterval: 1 * time.Second,
		},
	}
}
