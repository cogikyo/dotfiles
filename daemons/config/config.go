// Package config provides unified YAML-based configuration for all daemons.
//
// Configuration is loaded from ~/dotfiles/daemons/daemons.yaml. If the file does
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

const configPath = "dotfiles/daemons/daemons.yaml"
const newtabLocalPath = "dotfiles/daemons/newtab/local.config.yaml"

// ═══════════════════════════════════════════════════════════════════════════
// Root
// ═══════════════════════════════════════════════════════════════════════════

// Config is the root configuration loaded from ~/dotfiles/daemons/daemons.yaml.
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

// ═══════════════════════════════════════════════════════════════════════════
// Hypr
// ═══════════════════════════════════════════════════════════════════════════

// HyprConfig is the configuration for hyprd, controlling monitor geometry,
// split ratios, visual styling, window management, and sessions.
type HyprConfig struct {
	Monitor        MonitorConfig              `yaml:"monitor"`
	Background     BackgroundConfig           `yaml:"background"`
	Init           InitConfig                 `yaml:"init"`
	Split          SplitConfig                `yaml:"split"`
	Style          StyleConfig                `yaml:"style"`
	Notify         NotifyConfig               `yaml:"notify"`
	Windows        WindowsConfig              `yaml:"windows"`
	Tabs           map[string]TabProfile      `yaml:"tabs"`
	ThreeBody      map[string]ThreeBodyWindow `yaml:"three_body"`
	Sessions       map[string]Session         `yaml:"sessions"`
	ActiveSessions map[int]string             `yaml:"active_sessions"`
}

// InitConfig controls the one-time boot sequence that runs when hyprd starts
// on a fresh Hyprland session (no existing windows).
type InitConfig struct {
	Sessions       []string `yaml:"sessions"`        // Layout sessions to open (by name)
	Execs          []string `yaml:"execs"`           // Commands to launch (via hyprctl dispatch exec)
	Workspace      int      `yaml:"workspace"`       // Workspace to focus after init
	Lock           bool     `yaml:"lock"`            // Run hyprlock after init
	Greeting       string   `yaml:"greeting"`        // Notification message on boot
	NetworkTimeout int      `yaml:"network_timeout"` // Seconds to wait for network (0 = skip)
}

// BackgroundConfig controls mpvpaper video wallpaper.
// A single mpvpaper process runs with an IPC socket for health checks.
type BackgroundConfig struct {
	Display   string    `yaml:"display"`
	VideoPath string    `yaml:"video_path"`
	Socket    string    `yaml:"socket"`
	Wallpaper Wallpaper `yaml:"wallpaper"`
}

// Wallpaper defines the video file and mpv visual properties for mpvpaper.
type Wallpaper struct {
	File       string `yaml:"file"`
	Brightness int    `yaml:"brightness"`
	Contrast   int    `yaml:"contrast"`
	Saturation int    `yaml:"saturation"`
	Hue        int    `yaml:"hue"`
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
}

// ShadowColors specifies shadow colors using Hyprland color formats.
type ShadowColors struct {
	Default string `yaml:"default"`
}

// NotifyConfig defines notification routing, Dunst presentation, and sound policy.
type NotifyConfig struct {
	SoundsDir           string                 `yaml:"sounds_dir"`
	WorkspaceIconsDir   string                 `yaml:"workspace_icons_dir"`
	WorkspaceIcons      map[int]string         `yaml:"workspace_icons"`
	QuietVolume         int                    `yaml:"quiet_volume"`
	LoudVolume          int                    `yaml:"loud_volume"`
	Styles              map[string]NotifyStyle `yaml:"styles"`
	UrgencySounds       map[string]string      `yaml:"urgency_sounds"`
	AppSounds           map[string]string      `yaml:"app_sounds"`
	SilentApps          []string               `yaml:"silent_apps"`
	KittySilentPatterns []string               `yaml:"kitty_silent_patterns"`
}

// NotifyStyle defines optional Dunst hint overrides for a notification class.
type NotifyStyle struct {
	Urgency    string `yaml:"urgency"`
	Timeout    int    `yaml:"timeout"`
	Background string `yaml:"background"`
	Foreground string `yaml:"foreground"`
	Frame      string `yaml:"frame"`
	IconSuffix string `yaml:"icon_suffix"`
	Persistent bool   `yaml:"persistent"`
}

// WindowsConfig controls which windows hyprd manages and where to hide slave windows.
type WindowsConfig struct {
	IgnoredClasses  []string      `yaml:"ignored_classes"`
	HiddenWorkspace string        `yaml:"hidden_workspace"`
	ShadowWorkspace string        `yaml:"shadow_workspace"`
	Monocle         MonocleConfig `yaml:"monocle"`
}

// MonocleConfig controls single-window monocle mode sizing and position.
type MonocleConfig struct {
	Width   int `yaml:"width"`
	Height  int `yaml:"height"`
	OffsetX int `yaml:"offset_x"`
	OffsetY int `yaml:"offset_y"`
}

// TabProfile defines a set of kitty tabs managed as a unit (e.g., editor, agents).
type TabProfile struct {
	Prefix string   `yaml:"prefix"`
	Focus  string   `yaml:"focus"`
	Tabs   []TabDef `yaml:"tabs"`
}

// TabDef defines a single kitty tab within a profile.
type TabDef struct {
	Name       string `yaml:"name"`
	Title      string `yaml:"title"`
	Command    string `yaml:"command"`     // empty = plain shell
	CWD        string `yaml:"cwd"`         // base directory, expands ~/
	CWDResolve string `yaml:"cwd_resolve"` // "recent-git": find child dir with latest commit
	Requires   string `yaml:"requires"`    // "justfile" or "git", skip tab if missing
}

// ThreeBodyWindow defines a window that participates in the three-body layout.
type ThreeBodyWindow struct {
	Class   string `yaml:"class"`
	Title   string `yaml:"title"`
	Command string `yaml:"command"`
}

// Session defines a workspace layout for automated window spawning and arrangement.
// Three-body sessions specify Body (references to ThreeBody keys) and optionally
// Browser config for URLs. Simple sessions use Command for a single window.
type Session struct {
	Name      string            `yaml:"name" json:"name"`
	Workspace int               `yaml:"workspace" json:"workspace"`
	Project   string            `yaml:"project" json:"project"`
	Body      []string          `yaml:"body" json:"body"`       // three-body window keys (e.g. ["editor", "browser", "agents"])
	Browser   BrowserConfig     `yaml:"browser" json:"browser"` // URL config for the browser body member
	Tabs      map[string]string `yaml:"tabs" json:"tabs"`       // per-body tab profile overrides (body key → tab profile name)
	Command   string            `yaml:"command" json:"command"` // simple single-window session (e.g. "slack")
}

// BrowserConfig declares URLs to open in the browser body member.
// Pinned tabs and tab groups are modeled for future WebExtension support;
// the current implementation flattens everything into sequential URL opens.
type BrowserConfig struct {
	Pinned []string       `yaml:"pinned" json:"pinned"` // URLs to pin (future: WebExtension)
	Groups []BrowserGroup `yaml:"groups" json:"groups"` // tab groups (future: WebExtension)
	URLs   []string       `yaml:"urls" json:"urls"`     // ungrouped tabs
}

// BrowserGroup defines a named set of URLs that belong together in a Firefox tab group.
type BrowserGroup struct {
	Name string   `yaml:"name" json:"name"`
	URLs []string `yaml:"urls" json:"urls"`
}

// AllURLs returns all URLs flattened in open order: pinned first, then groups, then ungrouped.
func (b *BrowserConfig) AllURLs() []string {
	var urls []string
	urls = append(urls, b.Pinned...)
	for _, g := range b.Groups {
		urls = append(urls, g.URLs...)
	}
	urls = append(urls, b.URLs...)
	return urls
}

// MonocleSize returns the configured monocle window dimensions.
func (c *HyprConfig) MonocleSize() (w, h int) {
	return c.Windows.Monocle.Width, c.Windows.Monocle.Height
}

// MonocleOffset returns the x/y offset from center for monocle windows.
func (c *HyprConfig) MonocleOffset() (x, y int) {
	return c.Windows.Monocle.OffsetX, c.Windows.Monocle.OffsetY
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
			Background: BackgroundConfig{
				Display:   "HDMI-A-1",
				VideoPath: "~/dotfiles/share/videos",
				Socket:    "/tmp/mpvpaper.sock",
				Wallpaper: Wallpaper{
					File:       "dna.mp4",
					Brightness: 6,
					Contrast:   9,
					Saturation: -16,
					Hue:        -24,
				},
			},
			Init: InitConfig{
				Sessions: []string{"dotfiles"},
				Execs: []string{
					"glava -e bars_rc.glsl",
					"glava -e bars_r_rc.glsl",
					"glava -e radial_rc.glsl",
					"spotify-launcher",
					"bluetoothctl connect 14:3F:A6:10:A1:B5",
				},
				Workspace:      1,
				Lock:           true,
				Greeting:       "こんにちは",
				NetworkTimeout: 10,
			},
			Split: SplitConfig{
				XS:      "0.37",
				Default: "0.4942",
				LG:      "0.77",
			},
			Style: StyleConfig{
				Border: BorderColors{
					Default: "rgb(f2a170)",
				},
				Shadow: ShadowColors{
					Default: "rgba(e56b2c32)",
				},
			},
			Notify: NotifyConfig{
				SoundsDir:         "~/dotfiles/share/sounds",
				WorkspaceIconsDir: "~/dotfiles/config/eww/art/ws-icons",
				WorkspaceIcons: map[int]string{
					1: "network",
					2: "learn",
					3: "dna",
					4: "dotfiles",
					5: "music",
				},
				QuietVolume: 52428,
				LoudVolume:  72089,
				Styles: map[string]NotifyStyle{
					"start": {
						Urgency:    "low",
						Timeout:    2000,
						Background: "#25284180",
						Foreground: "#8aa4f3",
						Frame:      "#6380ec80",
					},
					"subagent": {
						Urgency:    "low",
						Timeout:    2000,
						Background: "#25284180",
						Foreground: "#8aa4f3",
						Frame:      "#6380ec80",
					},
					"complete": {
						Urgency:    "normal",
						Timeout:    0,
						Background: "#49353130",
						Foreground: "#f2a170",
						Frame:      "#ea834b",
						IconSuffix: "-active",
						Persistent: true,
					},
					"idle": {
						Urgency:    "normal",
						Timeout:    0,
						Background: "#2a2d4580",
						Foreground: "#b29ae8",
						Frame:      "#9376d880",
						IconSuffix: "-idle",
						Persistent: true,
					},
					"permission": {
						Urgency:    "normal",
						Timeout:    0,
						Background: "#2a3d2580",
						Foreground: "#95cb79",
						Frame:      "#73ad5a80",
						IconSuffix: "-monocle",
						Persistent: true,
					},
				},
				UrgencySounds: map[string]string{
					"low":      "none",
					"normal":   "moondrop",
					"critical": "gems",
				},
				AppSounds: map[string]string{
					"timer":     "jackpot",
					"attention": "discovery",
					"chat":      "reward",
					"work":      "fruit",
					"wf-start":  "discovery",
					"wf-end":    "reward",
					"Slack":     "knock",
				},
				SilentApps: []string{
					"Spotify",
					"discord",
					"claude",
					"codex",
					"claude-input",
					"",
					"󰯉",
					"",
					"",
					"",
				},
				KittySilentPatterns: []string{"claude", "codex", "approval"},
			},
			Windows: WindowsConfig{
				IgnoredClasses:  []string{"GLava"},
				HiddenWorkspace: "special:hiddenSlaves",
				ShadowWorkspace: "special:shadow",
				Monocle: MonocleConfig{
					Width:  3190,
					Height: 1920,
				},
			},
			ThreeBody: map[string]ThreeBodyWindow{
				"editor": {
					Class:   "kitty",
					Title:   "editor",
					Command: "kitty --title=editor --session ~/.config/kitty/sessions/editor.conf",
				},
				"agents": {
					Class:   "kitty",
					Title:   "agents",
					Command: "kitty --title=agents --session ~/.config/kitty/sessions/agents.conf",
				},
				"browser": {
					Class:   "firefox-developer-edition",
					Command: "firefox-developer-edition",
				},
			},
			Sessions: map[string]Session{
				"slack": {
					Name:      "slack",
					Workspace: 2,
					Command:   "slack",
				},
				"discord": {
					Name:      "discord",
					Workspace: 2,
					Command:   "discord",
				},
				"tableplus": {
					Name:      "tableplus",
					Workspace: 3,
					Command:   "tableplus",
				},
				"inkscape": {
					Name:      "inkscape",
					Workspace: 3,
					Command:   "inkscape",
				},
				"obs": {
					Name:      "obs",
					Workspace: 3,
					Command:   "obs",
				},
				"gimp": {
					Name:      "gimp",
					Workspace: 3,
					Command:   "gimp",
				},
				"leadpier": {
					Name:      "leadpier",
					Workspace: 4,
					Project:   "LeadPier",
					Body:      []string{"editor", "browser", "agents"},
					Browser: BrowserConfig{
						URLs: []string{"http://localhost:4000"},
					},
					Tabs: map[string]string{"editor": "leadpier"},
				},
				"dotfiles": {
					Name:      "dotfiles",
					Workspace: 5,
					Project:   "dotfiles",
					Body:      []string{"editor", "browser", "agents"},
					Browser: BrowserConfig{
						URLs: []string{"https://github.com/cogikyo/dotfiles"},
					},
				},
				"acr": {
					Name:      "acr",
					Workspace: 3,
					Project:   "cogikyo/acr",
					Body:      []string{"editor", "browser", "agents"},
					Browser: BrowserConfig{
						URLs: []string{"localhost:3002"},
					},
				},
				"cogikyo": {
					Name:      "cogikyo",
					Workspace: 4,
					Project:   "cogikyo/cogikyo.com",
					Body:      []string{"editor", "browser", "agents"},
					Browser: BrowserConfig{
						URLs: []string{"localhost:3000"},
					},
				},
			},
			ActiveSessions: map[int]string{
				2: "slack",
				3: "tableplus",
				4: "leadpier",
				5: "dotfiles",
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

// Load reads config from ~/dotfiles/daemons/daemons.yaml, falling back to defaults on any error.
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

	// Overlay newtab local config (machine-specific overrides, not tracked in git).
	localPath := filepath.Join(home, newtabLocalPath)
	if localData, err := os.ReadFile(localPath); err == nil {
		if err := yaml.Unmarshal(localData, &cfg.Newtab); err != nil {
			fmt.Fprintf(os.Stderr, "daemons: newtab local config parse error: %v\n", err)
		}
	}

	cfg.Hypr.Notify.UrgencySounds = lowercaseKeys(cfg.Hypr.Notify.UrgencySounds)
	cfg.Hypr.Notify.AppSounds = lowercaseKeys(cfg.Hypr.Notify.AppSounds)
	cfg.Hypr.Notify.SilentApps = lowercaseSlice(cfg.Hypr.Notify.SilentApps)

	return cfg
}

func lowercaseKeys(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

func lowercaseSlice(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = strings.ToLower(v)
	}
	return out
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
