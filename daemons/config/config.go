// Package config provides unified YAML-based configuration for all daemons.
//
// Configuration is loaded from ~/dotfiles/daemons/config.yaml. If the file does
// not exist or contains errors, sensible defaults are used. The Config struct
// contains top-level sections for each daemon (eww, hypr, newtab).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const configPath = "dotfiles/daemons/config.yaml"

// ═══════════════════════════════════════════════════════════════════════════
// Root
// ═══════════════════════════════════════════════════════════════════════════

// Config is the root configuration loaded from ~/dotfiles/daemons/config.yaml.
type Config struct {
	Eww    EwwConfig    `yaml:"eww"`
	Hypr   HyprConfig   `yaml:"hypr"`
	Newtab NewtabConfig `yaml:"newtab"`
}

// ═══════════════════════════════════════════════════════════════════════════
// Eww
// ═══════════════════════════════════════════════════════════════════════════

// EwwConfig defines all provider-specific settings for the ewwd daemon.
type EwwConfig struct {
	Weather    WeatherConfig    `yaml:"weather"`
	Timer      TimerConfig      `yaml:"timer"`
	Audio      AudioConfig      `yaml:"audio"`
	Brightness BrightnessConfig `yaml:"brightness"`
	Date       DateConfig       `yaml:"date"`
	GPU        GPUConfig        `yaml:"gpu"`
	Network    NetworkConfig    `yaml:"network"`
}

// WeatherConfig configures OpenWeatherMap API integration for live weather updates.
// Location is auto-detected via IP geolocation; ~/.local/.location is used as fallback.
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

// ═══════════════════════════════════════════════════════════════════════════
// Hypr
// ═══════════════════════════════════════════════════════════════════════════

// HyprConfig is the configuration for hyprd, controlling monitor geometry,
// monocle mode, split ratios, visual styling, window management, and sessions.
type HyprConfig struct {
	Monitor  MonitorConfig      `yaml:"monitor"`
	Monocle  MonocleConfig      `yaml:"monocle"`
	Split    SplitConfig        `yaml:"split"`
	Style    StyleConfig        `yaml:"style"`
	Windows  WindowsConfig      `yaml:"windows"`
	Sessions map[string]Session `yaml:"sessions"`
}

// MonitorConfig defines physical monitor dimensions and reserved screen areas.
type MonitorConfig struct {
	Width    int            `yaml:"width"`
	Height   int            `yaml:"height"`
	Reserved ReservedConfig `yaml:"reserved"`
}

// ReservedConfig specifies screen edges to exclude from window layout calculations.
type ReservedConfig struct {
	Top    int `yaml:"top"`
	Bottom int `yaml:"bottom"`
	Left   int `yaml:"left"`
}

// MonocleConfig controls single-window fullscreen mode.
type MonocleConfig struct {
	Workspace   int     `yaml:"workspace"`
	WidthRatio  float64 `yaml:"width_ratio"`
	HeightRatio float64 `yaml:"height_ratio"`
}

// SplitConfig defines predefined master-slave split ratios.
type SplitConfig struct {
	XS      string `yaml:"xs"`
	Default string `yaml:"default"`
	LG      string `yaml:"lg"`
}

// StyleConfig defines visual styling for borders and shadows.
type StyleConfig struct {
	Border BorderColors `yaml:"border"`
	Shadow ShadowColors `yaml:"shadow"`
}

// BorderColors specifies border colors using Hyprland color formats.
type BorderColors struct {
	Default string `yaml:"default"`
	Monocle string `yaml:"monocle"`
}

// ShadowColors specifies shadow colors using Hyprland color formats.
type ShadowColors struct {
	Default string `yaml:"default"`
	Monocle string `yaml:"monocle"`
}

// WindowsConfig controls which windows hyprd manages and where to hide slave windows.
type WindowsConfig struct {
	IgnoredClasses  []string `yaml:"ignored_classes"`
	HiddenWorkspace string   `yaml:"hidden_workspace"`
}

// Session defines a workspace layout for automated window spawning and arrangement.
type Session struct {
	Name      string         `yaml:"name" json:"name"`
	Workspace int            `yaml:"workspace" json:"workspace"`
	Project   string         `yaml:"project" json:"project"`
	URLs      []string       `yaml:"urls" json:"urls"`
	Windows   []WindowConfig `yaml:"windows" json:"windows"`
}

// WindowConfig defines a window to spawn and its position in the master/slave layout.
type WindowConfig struct {
	Command string `yaml:"command"`
	Title   string `yaml:"title"`
	Role    string `yaml:"role"`
}

// UsableHeight returns the screen height available for windows after subtracting reserved areas.
func (c *HyprConfig) UsableHeight() int {
	return c.Monitor.Height - c.Monitor.Reserved.Top - c.Monitor.Reserved.Bottom
}

// MonocleWidth returns the monocle window width based on monitor width and width ratio.
func (c *HyprConfig) MonocleWidth() int {
	return int(float64(c.Monitor.Width) * c.Monocle.WidthRatio)
}

// MonocleHeight returns the monocle window height based on usable height and height ratio.
func (c *HyprConfig) MonocleHeight() int {
	return int(float64(c.UsableHeight()) * c.Monocle.HeightRatio)
}

// IsIgnored returns true if the given window class is in the IgnoredClasses list.
func (c *HyprConfig) IsIgnored(class string) bool {
	return slices.Contains(c.Windows.IgnoredClasses, class)
}

// ═══════════════════════════════════════════════════════════════════════════
// Newtab
// ═══════════════════════════════════════════════════════════════════════════

// NewtabConfig defines settings for the newtab HTTP server.
type NewtabConfig struct {
	Port         string `yaml:"port"`
	FirefoxDB    string `yaml:"firefox_db"`
	StaticDir    string `yaml:"static_dir"`
	HistoryLimit int    `yaml:"history_limit"`
}

// ═══════════════════════════════════════════════════════════════════════════
// Loading
// ═══════════════════════════════════════════════════════════════════════════

// Default returns a config with sensible defaults for all daemons.
func Default() *Config {
	return &Config{
		Eww: EwwConfig{
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
		},
		Hypr: HyprConfig{
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
			Sessions: map[string]Session{
				"dotfiles": {
					Name:      "dotfiles",
					Workspace: 4,
					Project:   "dotfiles",
					URLs:      []string{"https://github.com/cogikyo/dotfiles"},
					Windows: []WindowConfig{
						{Command: "kitty --title terminal", Title: "terminal", Role: "master"},
						{Title: "firefox", Role: "slave"},
						{Command: "kitty --title claude", Title: "claude", Role: "slave"},
					},
				},
				"acr": {
					Name:      "acr",
					Workspace: 3,
					Project:   "cogikyo/acr",
					URLs:      []string{"localhost:3002"},
					Windows: []WindowConfig{
						{Command: "kitty --title terminal", Title: "terminal", Role: "master"},
						{Title: "firefox", Role: "slave"},
						{Command: "kitty --title claude", Title: "claude", Role: "slave"},
					},
				},
				"cogikyo": {
					Name:      "cogikyo",
					Workspace: 3,
					Project:   "cogikyo/cogikyo.com",
					URLs:      []string{"localhost:3000"},
					Windows: []WindowConfig{
						{Command: "kitty --title terminal", Title: "terminal", Role: "master"},
						{Title: "firefox", Role: "slave"},
						{Command: "kitty --title claude", Title: "claude", Role: "slave"},
					},
				},
			},
		},
		Newtab: NewtabConfig{
			Port:         ":42069",
			FirefoxDB:    ".mozilla/firefox/sdfm8kqz.dev-edition-default/places.sqlite",
			StaticDir:    "dotfiles/daemons/newtab",
			HistoryLimit: 15,
		},
	}
}

// Load reads config from ~/dotfiles/daemons/config.yaml, falling back to defaults on any error.
func Load() *Config {
	cfg := Default()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "daemons: config parse error: %v\n", err)
		return Default()
	}

	return cfg
}

// ExpandPath converts ~/ prefix to absolute home directory path, leaving other paths unchanged.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
