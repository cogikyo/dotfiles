// Package config defines typed configuration models and loaders used by local daemons.
//
// Responsibilities:
// - Define shared and per-daemon config structures.
// - Provide YAML-backed loaders (no compiled defaults for hyprd).
// - Resolve daemon config paths under the user's home directory.
package config

// root.go defines the aggregate Config container and default constructor.

// Config aggregates every daemon's settings.
type Config struct {
	Eww    EwwConfig    `yaml:"eww"`
	Hypr   HyprConfig   `yaml:"hypr"`
	Newtab NewtabConfig `yaml:"newtab"`
}
