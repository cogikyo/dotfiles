package config

// Config is the combined configuration shape used by callers that still want
// one object for all daemons.
type Config struct {
	Eww    EwwConfig    `yaml:"eww"`
	Hypr   HyprConfig   `yaml:"hypr"`
	Newtab NewtabConfig `yaml:"newtab"`
}

// Default returns the full config populated with per-daemon defaults.
func Default() *Config {
	return &Config{
		Eww:    DefaultEww(),
		Hypr:   DefaultHypr(),
		Newtab: DefaultNewtab(),
	}
}
