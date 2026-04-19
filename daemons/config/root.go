// Package config defines typed configuration models and loaders used by local daemons.
//
// It:
//   - defines shared and per-daemon config structures
//   - provides compiled defaults and YAML-backed loaders
//   - resolves daemon config paths under the user's home directory
package config

// root.go defines the aggregate Config container and default constructor.

// Config aggregates every daemon's settings.
type Config struct {
	Eww    EwwConfig    `yaml:"eww"`
	Hypr   HyprConfig   `yaml:"hypr"`
	Newtab NewtabConfig `yaml:"newtab"`
}

// Default returns a Config with every daemon's compiled defaults.
func Default() *Config {
	return &Config{
		Eww:    DefaultEww(),
		Hypr:   DefaultHypr(),
		Newtab: DefaultNewtab(),
	}
}
