// Package config defines the shared configuration schema for every daemon.
//
// Each daemon has its own typed view (HyprConfig, EwwConfig, NewtabConfig) loaded from YAML under
// ~/dotfiles/daemons/configs/, with Default* fallbacks when files are missing or malformed.
// Callers typically use the per-daemon loader; Load aggregates all three for callers that want one object.
package config

// Config aggregates every daemon's settings. Most callers use LoadHypr/LoadEww/LoadNewtab directly.
type Config struct {
	Eww    EwwConfig    `yaml:"eww"`
	Hypr   HyprConfig   `yaml:"hypr"`
	Newtab NewtabConfig `yaml:"newtab"`
}

// Default returns a Config populated with every daemon's defaults.
func Default() *Config {
	return &Config{
		Eww:    DefaultEww(),
		Hypr:   DefaultHypr(),
		Newtab: DefaultNewtab(),
	}
}
