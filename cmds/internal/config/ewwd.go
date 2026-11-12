package config

import "time"

type EwwConfig struct {
	Windows WindowSets    `yaml:"windows"`
	Weather WeatherConfig `yaml:"weather"`
	Timer   TimerConfig   `yaml:"timer"`
	Audio   AudioConfig   `yaml:"audio"`
	Date    DateConfig    `yaml:"date"`
	Network NetworkConfig `yaml:"network"`
	Music   MusicConfig   `yaml:"music"`
}

type WindowSets struct {
	Full    []string `yaml:"full"`
	Compact []string `yaml:"compact"`
}

func (w WindowSets) For(layout Layout) (show, hide []string) {
	if layout == LayoutCompact {
		return w.Compact, w.Full
	}
	return w.Full, w.Compact
}

type MusicConfig struct {
	SpDc string `yaml:"sp_dc"` // open.spotify.com session cookie for Canvas API auth
}

type WeatherConfig struct {
	APIKeyFile   string        `yaml:"api_key_file"`  // path to plaintext file containing the OWM API key
	PollInterval time.Duration `yaml:"poll_interval"` // how often to refetch from the OWM API
}

type TimerConfig struct {
	DefaultMinutes    int `yaml:"default_minutes"`     // initial timer length in minutes
	DefaultAlarmHours int `yaml:"default_alarm_hours"` // initial alarm offset from now, in hours
	MinAlarmHours     int `yaml:"min_alarm_hours"`     // minimum allowed alarm offset, in hours
}

type AudioConfig struct {
	SourceMax           int               `yaml:"source_max"`            // maximum source volume %, may exceed 100 (boost)
	SinkMax             int               `yaml:"sink_max"`              // maximum sink volume %, typically <= 100
	VolumeStep          int               `yaml:"volume_step"`           // % per up/down action
	DefaultSinkVolume   int               `yaml:"default_sink_volume"`   // sink volume % applied by reset_volume
	DefaultSourceVolume int               `yaml:"default_source_volume"` // source volume % applied by reset_volume
	NameMappings        map[string]string `yaml:"name_mappings"`         // node.nick or node.name -> display alias
	FallbackSinks       []string          `yaml:"fallback_sinks"`
	HeadphonesSource    string            `yaml:"headphones_source"`
	FallbackSource      string            `yaml:"fallback_source"`
	Snapcast            SnapcastConfig    `yaml:"snapcast"`
}

type SnapcastConfig struct {
	Sink string `yaml:"sink"`
	RPC  string `yaml:"rpc"`
}

type DateConfig struct {
	BirthDate string `yaml:"birth_date"` // ISO-8601 date
}

type NetworkConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
}

func DefaultEww() EwwConfig {
	return EwwConfig{
		Windows: WindowSets{
			Full:    []string{"today", "workspaces", "computer", "music", "player"},
			Compact: []string{"bar"},
		},
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
