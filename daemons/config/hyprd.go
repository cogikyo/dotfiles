package config

import (
	"slices"
	"time"
)

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ root config                                                               │
// ╰───────────────────────────────────────────────────────────────────────────╯

// HyprConfig configures hyprd window and session behavior.
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
	ActiveSessions map[int]ActiveSession      `yaml:"active_sessions"`
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ boot / environment                                                        │
// ╰───────────────────────────────────────────────────────────────────────────╯

// InitConfig controls the one-time boot sequence that runs when hyprd starts.
//
// Sessions opened on boot come from ActiveSessions entries with init: true;
// this config only covers the environment setup that precedes them.
type InitConfig struct {
	Execs          []string      `yaml:"execs"`           // commands run once at hyprd startup
	Workspace      int           `yaml:"workspace"`       // workspace to focus after boot
	Lock           bool          `yaml:"lock"`            // lock the session after boot completes
	LockDelay      time.Duration `yaml:"lock_delay"`      // delay before locking (gives sessions time to settle)
	NetworkTimeout int           `yaml:"network_timeout"` // seconds to wait for network before proceeding
}

// BackgroundConfig controls the mpvpaper video wallpaper.
type BackgroundConfig struct {
	Display   string    `yaml:"display"`    // monitor name (e.g. "HDMI-A-1")
	VideoPath string    `yaml:"video_path"` // directory containing wallpaper videos
	Socket    string    `yaml:"socket"`     // mpv IPC socket path
	Wallpaper Wallpaper `yaml:"wallpaper"`
}

// Wallpaper picks a video file and tunes mpv visual properties.
type Wallpaper struct {
	File       string `yaml:"file"` // filename relative to BackgroundConfig.VideoPath
	Brightness int    `yaml:"brightness"`
	Contrast   int    `yaml:"contrast"`
	Saturation int    `yaml:"saturation"`
	Hue        int    `yaml:"hue"`
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ layout / style                                                            │
// ╰───────────────────────────────────────────────────────────────────────────╯

// MonitorConfig defines physical monitor dimensions and reserved screen areas.
type MonitorConfig struct {
	Width    int            `yaml:"width"`  // px
	Height   int            `yaml:"height"` // px
	Reserved ReservedConfig `yaml:"reserved"`
}

// ReservedConfig specifies screen edges (px) to exclude from layout calculations.
type ReservedConfig struct {
	Top    int `yaml:"top"`
	Bottom int `yaml:"bottom"`
	Left   int `yaml:"left"`
}

// SplitConfig defines predefined master-slave split ratios (stringified floats 0-1).
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

// BorderColors specifies border colors in Hyprland's color format (e.g. "rgb(f2a170)").
type BorderColors struct {
	Default string `yaml:"default"`
}

// ShadowColors specifies shadow colors in Hyprland's color format (e.g. "rgba(e56b2c32)").
type ShadowColors struct {
	Default string `yaml:"default"`
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ notifications                                                             │
// ╰───────────────────────────────────────────────────────────────────────────╯

// NotifyConfig defines notification routing, presentation, and sound policy.
//
// LoadHypr lowercases UrgencySounds, AppSounds, and SilentApps, so YAML entries can be written in any case.
type NotifyConfig struct {
	SoundsDir           string                 `yaml:"sounds_dir"`            // directory containing sound files
	WorkspaceIconsDir   string                 `yaml:"workspace_icons_dir"`   // directory containing workspace icon PNGs
	WorkspaceIcons      map[int]string         `yaml:"workspace_icons"`       // workspace number -> icon basename
	QuietVolume         int                    `yaml:"quiet_volume"`          // paplay volume (0-65536) for low-priority sounds
	LoudVolume          int                    `yaml:"loud_volume"`           // paplay volume (0-65536) for high-priority sounds
	Styles              map[string]NotifyStyle `yaml:"styles"`                // notification class -> Dunst hint overrides
	UrgencySounds       map[string]string      `yaml:"urgency_sounds"`        // libnotify urgency -> sound name (or "none")
	AppSounds           map[string]string      `yaml:"app_sounds"`            // app name -> sound name, takes precedence over urgency
	SilentApps          []string               `yaml:"silent_apps"`           // app names that suppress sound entirely
	KittySilentPatterns []string               `yaml:"kitty_silent_patterns"` // substrings in kitty titles that suppress sound
}

// NotifyStyle defines optional Dunst hint overrides for a notification class.
type NotifyStyle struct {
	Urgency    string `yaml:"urgency"`     // low, normal, critical
	Timeout    int    `yaml:"timeout"`     // ms; 0 means persistent
	Background string `yaml:"background"`  // hex color with optional alpha
	Foreground string `yaml:"foreground"`  // hex color
	Frame      string `yaml:"frame"`       // border color
	IconSuffix string `yaml:"icon_suffix"` // appended to the icon basename
	Persistent bool   `yaml:"persistent"`  // prevents Dunst timeout from closing
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ windows / workspaces                                                      │
// ╰───────────────────────────────────────────────────────────────────────────╯

// WindowsConfig controls which windows hyprd manages and how special workspaces are named.
type WindowsConfig struct {
	IgnoredClasses  []string      `yaml:"ignored_classes"`  // window classes hyprd ignores for tiling/events
	HiddenWorkspace string        `yaml:"hidden_workspace"` // Hyprland special workspace name for stashed windows
	ShadowWorkspace string        `yaml:"shadow_workspace"` // Hyprland special workspace name for shadow windows
	Monocle         MonocleConfig `yaml:"monocle"`
}

// MonocleConfig controls single-window monocle mode sizing and offset (px).
type MonocleConfig struct {
	Width   int `yaml:"width"`
	Height  int `yaml:"height"`
	OffsetX int `yaml:"offset_x"` // offset from monitor center
	OffsetY int `yaml:"offset_y"` // offset from monitor center
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ kitty tab profiles                                                        │
// ╰───────────────────────────────────────────────────────────────────────────╯

// TabProfile defines a set of kitty tabs managed as a unit.
type TabProfile struct {
	Order  int      `yaml:"order"`  // positional index for colon-separated tab aliases
	Prefix string   `yaml:"prefix"` // applied to each tab name (disambiguates across profiles)
	Focus  string   `yaml:"focus"`  // name of the tab to focus after spawn
	Tabs   []TabDef `yaml:"tabs"`
}

// TabDef defines a single kitty tab within a profile.
type TabDef struct {
	Name       string    `yaml:"name"`
	Title      string    `yaml:"title"`       // displayed in the kitty tab bar
	Command    string    `yaml:"command"`     // command to run in the tab
	CWD        string    `yaml:"cwd"`         // starting directory, may contain "~/"
	CWDResolve string    `yaml:"cwd_resolve"` // optional strategy for resolving CWD (e.g. "git-root")
	Requires   string    `yaml:"requires"`    // file/marker that must exist in CWD; tab is skipped otherwise
	Layout     string    `yaml:"layout"`      // kitty layout name, e.g. "fat:bias=80"
	Panes      []TabPane `yaml:"panes"`       // extra panes created after the primary command
}

// TabPane defines an extra pane created inside a kitty tab after the primary pane.
type TabPane struct {
	Command    string `yaml:"command"`
	CWD        string `yaml:"cwd"`
	CWDResolve string `yaml:"cwd_resolve"`
	Location   string `yaml:"location"` // kitty launch --location value (hsplit, vsplit, etc.)
	Bias       int    `yaml:"bias"`     // split bias (% of parent occupied)
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ sessions / three-body                                                     │
// ╰───────────────────────────────────────────────────────────────────────────╯

// ThreeBodyWindow defines a window that participates in the three-body layout.
type ThreeBodyWindow struct {
	Class   string `yaml:"class"`   // Hyprland class to match
	Title   string `yaml:"title"`   // Hyprland title to match (optional)
	Command string `yaml:"command"` // command used to spawn the window when missing
}

// ActiveSession binds a session to a workspace and controls whether it opens on boot.
type ActiveSession struct {
	Session string `yaml:"session"` // key into HyprConfig.Sessions
	Init    bool   `yaml:"init"`    // spawn during InitConfig run
}

// Session defines a workspace layout for automated window spawning and arrangement.
type Session struct {
	Name      string        `yaml:"name" json:"name"`
	Workspace int           `yaml:"workspace" json:"workspace"`
	Project   string        `yaml:"project" json:"project"` // project directory or repo identifier
	Body      []string      `yaml:"body" json:"body"`       // three-body member order (editor, browser, agents, ...)
	Browser   BrowserConfig `yaml:"browser" json:"browser"`
	// Tabs maps three-body members (e.g. "editor") to kitty tab profile names (e.g. "leadpier").
	Tabs    map[string]string `yaml:"tabs" json:"tabs"`
	Command string            `yaml:"command" json:"command"` // single-window sessions that don't use three-body
	// Monocle applies monocle sizing to the spawned window after the session opens.
	Monocle bool `yaml:"monocle" json:"monocle"`
}

// BrowserConfig declares URLs and tab-group state to open in the browser body member.
type BrowserConfig struct {
	Snapshot string         `yaml:"snapshot,omitempty" json:"snapshot,omitempty"` // named snapshot to restore
	Mode     string         `yaml:"mode,omitempty" json:"mode,omitempty"`         // open strategy (new, existing, etc.)
	Force    bool           `yaml:"force,omitempty" json:"force,omitempty"`       // reopen even if matching tabs exist
	Profile  string         `yaml:"profile,omitempty" json:"profile,omitempty"`   // Firefox profile flag passthrough
	Pinned   []string       `yaml:"pinned,omitempty" json:"pinned,omitempty"`     // URLs opened as pinned tabs
	Groups   []BrowserGroup `yaml:"groups,omitempty" json:"groups,omitempty"`     // named tab groups
	URLs     []string       `yaml:"urls,omitempty" json:"urls,omitempty"`         // ungrouped URLs
}

// BrowserGroup defines a named set of URLs that belong together in a Firefox tab group.
type BrowserGroup struct {
	Name      string   `yaml:"name" json:"name"`
	Color     string   `yaml:"color,omitempty" json:"color,omitempty"`
	Collapsed bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
	URLs      []string `yaml:"urls" json:"urls"`
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ methods                                                                   │
// ╰───────────────────────────────────────────────────────────────────────────╯

// AllURLs returns every URL flattened in open order: pinned, then each group, then loose URLs.
func (b *BrowserConfig) AllURLs() []string {
	var urls []string
	urls = append(urls, b.Pinned...)
	for _, g := range b.Groups {
		urls = append(urls, g.URLs...)
	}
	urls = append(urls, b.URLs...)
	return urls
}

// MonocleSize returns the configured monocle window dimensions in pixels.
func (c *HyprConfig) MonocleSize() (w, h int) {
	return c.Windows.Monocle.Width, c.Windows.Monocle.Height
}

// MonocleOffset returns the x/y offset (px) from monitor center for monocle windows.
func (c *HyprConfig) MonocleOffset() (x, y int) {
	return c.Windows.Monocle.OffsetX, c.Windows.Monocle.OffsetY
}

// IsIgnored reports whether the given Hyprland window class is in IgnoredClasses.
func (c *HyprConfig) IsIgnored(class string) bool {
	return slices.Contains(c.Windows.IgnoredClasses, class)
}

// ╭───────────────────────────────────────────────────────────────────────────╮
// │ defaults                                                                  │
// ╰───────────────────────────────────────────────────────────────────────────╯

// DefaultHypr returns sensible defaults for hyprd.
func DefaultHypr() HyprConfig {
	return HyprConfig{
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
			Execs: []string{
				"glava -e bars_rc.glsl",
				"glava -e bars_r_rc.glsl",
				"glava -e radial_rc.glsl",
				"spotify-launcher",
				"bluetoothctl connect 14:3F:A6:10:A1:B5",
			},
			Workspace:      1,
			Lock:           true,
			NetworkTimeout: 10,
		},
		Split: SplitConfig{
			XS:      "0.37",
			Default: "0.4942",
			LG:      "0.77",
		},
		Style: StyleConfig{
			Border: BorderColors{Default: "rgb(f2a170)"},
			Shadow: ShadowColors{Default: "rgba(e56b2c32)"},
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
		Tabs:           map[string]TabProfile{},
		ThreeBody:      map[string]ThreeBodyWindow{},
		Sessions:       map[string]Session{},
		ActiveSessions: map[int]ActiveSession{},
	}
}
