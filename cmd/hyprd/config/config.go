// Package config provides YAML-based configuration for hyprd.
//
// Configuration is loaded from ~/.config/hyprd/config.yaml. If the file does
// not exist or contains errors, sensible defaults are used. The configuration
// file uses YAML format with sections for monitor geometry, monocle mode,
// split ratios, visual styling, and window management settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Config represents the root configuration for hyprd.
type Config struct {
	Monitor MonitorConfig `yaml:"monitor"` // Monitor defines the monitor geometry and reserved screen areas.
	Monocle MonocleConfig `yaml:"monocle"` // Monocle defines monocle mode settings including workspace and sizing.
	Split   SplitConfig   `yaml:"split"`   // Split defines master-slave split ratio values.
	Style   StyleConfig   `yaml:"style"`   // Style defines visual styling for borders and shadows.
	Windows WindowsConfig `yaml:"windows"` // Windows defines window management settings.
}

// MonitorConfig holds monitor geometry settings.
type MonitorConfig struct {
	// Width is the monitor width in pixels.
	Width int `yaml:"width"`
	// Height is the monitor height in pixels.
	Height int `yaml:"height"`
	// Reserved defines screen areas reserved for bars and gaps.
	Reserved ReservedConfig `yaml:"reserved"`
}

// ReservedConfig holds reserved screen areas for bars and gaps.
type ReservedConfig struct {
	// Top is the reserved space at the top of the screen in pixels.
	Top int `yaml:"top"`
	// Bottom is the reserved space at the bottom of the screen in pixels.
	Bottom int `yaml:"bottom"`
	// Left is the reserved space on the left side of the screen in pixels.
	Left int `yaml:"left"`
}

// MonocleConfig holds monocle mode settings.
type MonocleConfig struct {
	// Workspace is the workspace number used for monocle mode.
	Workspace int `yaml:"workspace"`
	// WidthRatio is the fraction of monitor width used for monocle windows.
	WidthRatio float64 `yaml:"width_ratio"`
	// HeightRatio is the fraction of usable height used for monocle windows.
	HeightRatio float64 `yaml:"height_ratio"`
}

// SplitConfig holds master-slave split ratio values.
type SplitConfig struct {
	// XS is the extra-small split ratio string.
	XS string `yaml:"xs"`
	// Default is the default split ratio string.
	Default string `yaml:"default"`
	// LG is the large split ratio string.
	LG string `yaml:"lg"`
}

// StyleConfig holds visual styling settings.
type StyleConfig struct {
	// Border defines border color settings for different modes.
	Border BorderColors `yaml:"border"`
	// Shadow defines shadow color settings for different modes.
	Shadow ShadowColors `yaml:"shadow"`
}

// BorderColors holds border color settings.
type BorderColors struct {
	// Default is the border color for normal windows.
	Default string `yaml:"default"`
	// Monocle is the border color for monocle mode windows.
	Monocle string `yaml:"monocle"`
}

// ShadowColors holds shadow color settings.
type ShadowColors struct {
	// Default is the shadow color for normal windows.
	Default string `yaml:"default"`
	// Monocle is the shadow color for monocle mode windows.
	Monocle string `yaml:"monocle"`
}

// WindowsConfig holds window management settings.
type WindowsConfig struct {
	// IgnoredClasses lists window classes that should be ignored by hyprd.
	IgnoredClasses []string `yaml:"ignored_classes"`
	// HiddenWorkspace is the special workspace used to hide slave windows.
	HiddenWorkspace string `yaml:"hidden_workspace"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Monitor: MonitorConfig{
			Width:  3840,
			Height: 2160,
			Reserved: ReservedConfig{
				Top:    86,
				Bottom: 32,
				Left:   0,
			},
		},
		Monocle: MonocleConfig{
			Workspace:   6,
			WidthRatio:  0.83,
			HeightRatio: 0.94,
		},
		Split: SplitConfig{
			XS:      "0.37",
			Default: "0.4942",
			LG:      "0.77",
		},
		Style: StyleConfig{
			Border: BorderColors{
				Default: "rgb(f2a170)",
				Monocle: "rgb(5aba6d)",
			},
			Shadow: ShadowColors{
				Default: "rgba(e56b2c32)",
				Monocle: "rgba(2d9a4342)",
			},
		},
		Windows: WindowsConfig{
			IgnoredClasses:  []string{"GLava"},
			HiddenWorkspace: "special:hiddenSlaves",
		},
	}
}

// Load reads configuration from ~/.config/hyprd/config.yaml and returns it.
// If the file does not exist or contains parse errors, Load returns the
// default configuration.
func Load() *Config {
	cfg := Default()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".config", "hyprd", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg // File doesn't exist, use defaults
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: config parse error: %v\n", err)
		return Default()
	}

	return cfg
}

// UsableHeight returns the usable screen height after reserved areas.
func (c *Config) UsableHeight() int {
	return c.Monitor.Height - c.Monitor.Reserved.Top - c.Monitor.Reserved.Bottom
}

// MonocleWidth returns the calculated monocle window width.
func (c *Config) MonocleWidth() int {
	return int(float64(c.Monitor.Width) * c.Monocle.WidthRatio)
}

// MonocleHeight returns the calculated monocle window height.
func (c *Config) MonocleHeight() int {
	return int(float64(c.UsableHeight()) * c.Monocle.HeightRatio)
}

// IsIgnored returns true if the window class should be ignored.
func (c *Config) IsIgnored(class string) bool {
	return slices.Contains(c.Windows.IgnoredClasses, class)
}
