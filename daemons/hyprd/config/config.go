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

// Config is the root configuration for hyprd, loaded from ~/.config/hyprd/config.yaml.
// It controls monitor geometry, monocle mode behavior, split ratios, visual styling, and window management.
type Config struct {
	Monitor MonitorConfig `yaml:"monitor"` // Geometry and reserved screen areas
	Monocle MonocleConfig `yaml:"monocle"` // Workspace and sizing for monocle mode
	Split   SplitConfig   `yaml:"split"`   // Master-slave split ratio values
	Style   StyleConfig   `yaml:"style"`   // Border and shadow colors
	Windows WindowsConfig `yaml:"windows"` // Ignored classes and hidden workspace
}

// MonitorConfig defines physical monitor dimensions and reserved screen areas for bars and gaps.
type MonitorConfig struct {
	Width    int             `yaml:"width"`    // Monitor width in pixels
	Height   int             `yaml:"height"`   // Monitor height in pixels
	Reserved ReservedConfig `yaml:"reserved"` // Screen areas reserved for bars and gaps
}

// ReservedConfig specifies screen edges to exclude from window layout calculations (e.g., for status bars).
type ReservedConfig struct {
	Top    int `yaml:"top"`    // Reserved pixels at top edge
	Bottom int `yaml:"bottom"` // Reserved pixels at bottom edge
	Left   int `yaml:"left"`   // Reserved pixels at left edge
}

// MonocleConfig controls single-window fullscreen mode, specifying which workspace to use and window size ratios.
type MonocleConfig struct {
	Workspace   int     `yaml:"workspace"`    // Workspace number for monocle mode
	WidthRatio  float64 `yaml:"width_ratio"`  // Fraction of monitor width (0.0-1.0)
	HeightRatio float64 `yaml:"height_ratio"` // Fraction of usable height after reserved areas (0.0-1.0)
}

// SplitConfig defines predefined master-slave split ratios (e.g., "0.5" for 50/50 split).
type SplitConfig struct {
	XS      string `yaml:"xs"`      // Extra-small split ratio
	Default string `yaml:"default"` // Default split ratio
	LG      string `yaml:"lg"`      // Large split ratio
}

// StyleConfig defines visual styling for borders and shadows in different window modes.
type StyleConfig struct {
	Border BorderColors `yaml:"border"` // Border colors for normal and monocle modes
	Shadow ShadowColors `yaml:"shadow"` // Shadow colors for normal and monocle modes
}

// BorderColors specifies border colors using Hyprland color formats (e.g., rgb(f2a170) or rgba(f2a17080)).
type BorderColors struct {
	Default string `yaml:"default"` // Border color for normal windows
	Monocle string `yaml:"monocle"` // Border color for monocle mode windows
}

// ShadowColors specifies shadow colors using Hyprland color formats (e.g., rgba(e56b2c32) with alpha for intensity).
type ShadowColors struct {
	Default string `yaml:"default"` // Shadow color for normal windows
	Monocle string `yaml:"monocle"` // Shadow color for monocle mode windows
}

// WindowsConfig controls which windows hyprd manages and where to hide slave windows in monocle mode.
type WindowsConfig struct {
	IgnoredClasses  []string `yaml:"ignored_classes"`  // Window classes to exclude from hyprd management
	HiddenWorkspace string   `yaml:"hidden_workspace"` // Special workspace for hidden slave windows
}

// Default returns a Config with sensible defaults for a 4K monitor (3840x2160) with catppuccin mocha colors.
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

// Load reads configuration from ~/.config/hyprd/config.yaml, falling back to Default() if the file
// doesn't exist or contains parse errors. Parse errors are printed to stderr.
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

// UsableHeight returns the screen height available for windows after subtracting top and bottom reserved areas.
func (c *Config) UsableHeight() int {
	return c.Monitor.Height - c.Monitor.Reserved.Top - c.Monitor.Reserved.Bottom
}

// MonocleWidth returns the monocle window width based on monitor width and width ratio.
func (c *Config) MonocleWidth() int {
	return int(float64(c.Monitor.Width) * c.Monocle.WidthRatio)
}

// MonocleHeight returns the monocle window height based on usable height and height ratio.
func (c *Config) MonocleHeight() int {
	return int(float64(c.UsableHeight()) * c.Monocle.HeightRatio)
}

// IsIgnored returns true if the given window class is in the IgnoredClasses list.
func (c *Config) IsIgnored(class string) bool {
	return slices.Contains(c.Windows.IgnoredClasses, class)
}
