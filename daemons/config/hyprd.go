package config

// hyprd.go declares hyprd's window, session, and notification configuration.

import (
	"slices"
	"time"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ root config                                                                  │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// HyprConfig configures hyprd window and session behavior.
type HyprConfig struct {
	Background     BackgroundConfig           `yaml:"background"`
	Init           InitConfig                 `yaml:"init"`
	Notify         NotifyConfig               `yaml:"notify"`
	Windows        WindowsConfig              `yaml:"windows"`
	Tabs           map[string]TabProfile      `yaml:"tabs"`
	ThreeBody      map[string]ThreeBodyWindow `yaml:"three_body"`
	Sessions       map[string]Session         `yaml:"sessions"`
	ActiveSessions map[int]ActiveSession      `yaml:"active_sessions"`
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ boot / environment                                                           │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// InitConfig controls the one-time boot sequence (env setup before sessions).
//
// Sessions opened on boot come from ActiveSessions entries with init: true.
type InitConfig struct {
	Execs          []string      `yaml:"execs"`           // commands run once at hyprd startup
	Workspace      int           `yaml:"workspace"`       // workspace to focus after boot
	Lock           bool          `yaml:"lock"`            // lock the session after boot completes
	LockDelay      time.Duration `yaml:"lock_delay"`      // delay before locking (gives sessions time to settle)
	NetworkTimeout int           `yaml:"network_timeout"` // seconds to wait for network before proceeding
}

// BackgroundConfig controls the mpvpaper video wallpaper.
type BackgroundConfig struct {
	Display   string    `yaml:"display"`    // monitor name, e.g. "HDMI-A-1"
	VideoPath string    `yaml:"video_path"` // directory containing wallpaper videos
	Socket    string    `yaml:"socket"`     // mpv IPC socket path
	Wallpaper Wallpaper `yaml:"wallpaper"`
}

// Wallpaper selects a video file and tunes mpv visual properties.
type Wallpaper struct {
	File       string `yaml:"file"` // filename relative to VideoPath
	Brightness int    `yaml:"brightness"`
	Contrast   int    `yaml:"contrast"`
	Saturation int    `yaml:"saturation"`
	Hue        int    `yaml:"hue"`
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ windows / layout                                                             │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// SplitConfig defines named master-slave split ratios (stringified floats 0-1).
type SplitConfig struct {
	XS      string `yaml:"xs"`
	Default string `yaml:"default"`
	LG      string `yaml:"lg"`
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ notifications                                                                │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// NotifyConfig defines notification routing, presentation, and sound policy.
type NotifyConfig struct {
	SoundsDir           string                       `yaml:"sounds_dir"`            // directory containing sound files
	WorkspaceIconsDir   string                       `yaml:"workspace_icons_dir"`   // directory containing workspace icon PNGs
	WorkspaceIcons      map[int]string               `yaml:"workspace_icons"`       // workspace number -> icon basename
	QuietVolume         int                          `yaml:"quiet_volume"`          // paplay volume (0-65536) for low-priority sounds
	LoudVolume          int                          `yaml:"loud_volume"`           // paplay volume (0-65536) for high-priority sounds
	Styles              map[string]NotifyStyle       `yaml:"styles"`                // notification class -> Dunst hint overrides
	UrgencySounds       map[string]string            `yaml:"urgency_sounds"`        // urgency -> sound name (or "none")
	AppSounds           map[string]string            `yaml:"app_sounds"`            // app name -> sound name; takes precedence over urgency
	FocusApps           map[string]string            `yaml:"focus_apps"`            // dunst app name -> Hyprland window class to focus on arrival
	ActionFocusApps     map[string]NotifyFocusTarget `yaml:"action_focus_apps"`     // dunst app/desktop-entry -> window target to focus on click
	SilentApps          []string                     `yaml:"silent_apps"`           // external app names that suppress sound entirely
	KittySilentPatterns []string                     `yaml:"kitty_silent_patterns"` // substrings in kitty notification content that suppress sound
}

// NotifyFocusTarget describes the window/workspace to focus when a notification action fires.
type NotifyFocusTarget struct {
	Workspace int    `yaml:"workspace"`
	Class     string `yaml:"class"`
	Title     string `yaml:"title"`
}

// NotifyStyle defines Dunst hint overrides for a notification class.
type NotifyStyle struct {
	Urgency    string `yaml:"urgency"`     // low, normal, critical
	Timeout    int    `yaml:"timeout"`     // ms; 0 means persistent
	Background string `yaml:"background"`  // hex color with optional alpha
	Foreground string `yaml:"foreground"`  // hex color
	Frame      string `yaml:"frame"`       // border color
	IconSuffix string `yaml:"icon_suffix"` // appended to the icon basename
	Persistent bool   `yaml:"persistent"`  // prevents Dunst timeout from closing
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ windows / workspaces                                                         │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// WindowsConfig controls tiling policy and special workspace naming.
type WindowsConfig struct {
	IgnoredClasses  []string      `yaml:"ignored_classes"`  // window classes hyprd ignores for tiling/events
	HiddenWorkspace string        `yaml:"hidden_workspace"` // Hyprland special workspace name for stashed windows
	ShadowWorkspace string        `yaml:"shadow_workspace"` // Hyprland special workspace name for shadow windows
	Split           SplitConfig   `yaml:"split"`
	Monocle         MonocleConfig `yaml:"monocle"`
}

// MonocleConfig controls single-window monocle mode sizing and offset (px).
type MonocleConfig struct {
	Width   int `yaml:"width"`
	Height  int `yaml:"height"`
	OffsetX int `yaml:"offset_x"`
	OffsetY int `yaml:"offset_y"`
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ kitty tab profiles                                                           │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// TabProfile defines a set of kitty tabs managed as a unit.
type TabProfile struct {
	Order  int      `yaml:"order"`  // positional index for colon-separated tab aliases
	Prefix string   `yaml:"prefix"` // disambiguates tab names across profiles
	Focus  string   `yaml:"focus"`  // tab to focus after spawn
	Tabs   []TabDef `yaml:"tabs"`
}

// TabDef defines a single kitty tab within a profile.
type TabDef struct {
	Name       string    `yaml:"name"`
	Title      string    `yaml:"title"`
	Command    string    `yaml:"command"`
	CWD        string    `yaml:"cwd"`         // may contain "~/"
	CWDResolve string    `yaml:"cwd_resolve"` // e.g. "git-root"
	Requires   string    `yaml:"requires"`    // must exist in CWD or tab is skipped
	Layout     string    `yaml:"layout"`      // e.g. "fat:bias=80"
	Panes      []TabPane `yaml:"panes"`
}

// TabPane defines an extra pane created inside a kitty tab.
type TabPane struct {
	Command    string `yaml:"command"`
	CWD        string `yaml:"cwd"`
	CWDResolve string `yaml:"cwd_resolve"`
	Location   string `yaml:"location"` // hsplit, vsplit, etc.
	Bias       int    `yaml:"bias"`     // % of parent
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ sessions / three-body                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// ThreeBodyWindow defines a window that participates in the three-body layout.
type ThreeBodyWindow struct {
	Class   string `yaml:"class"`
	Title   string `yaml:"title"`   // optional
	Command string `yaml:"command"` // spawn command when missing
}

// ActiveSession binds a session to a workspace.
type ActiveSession struct {
	Session string `yaml:"session"` // key into HyprConfig.Sessions
	Init    bool   `yaml:"init"`    // spawn on boot
}

// Session defines a workspace layout for automated window spawning and arrangement.
type Session struct {
	Name      string            `yaml:"name" json:"name"`
	Workspace int               `yaml:"workspace" json:"workspace"`
	Project   string            `yaml:"project" json:"project"`
	Body      []string          `yaml:"body" json:"body"` // three-body member order
	Browser   BrowserConfig     `yaml:"browser" json:"browser"`
	Tabs      map[string]string `yaml:"tabs" json:"tabs"`       // body member -> tab profile name
	Command   string            `yaml:"command" json:"command"` // single-window sessions (no three-body)
	Monocle   bool              `yaml:"monocle" json:"monocle"`
}

// BrowserConfig declares URLs and tab-group state for the browser body member.
type BrowserConfig struct {
	Snapshot string         `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
	Mode     string         `yaml:"mode,omitempty" json:"mode,omitempty"`
	Force    bool           `yaml:"force,omitempty" json:"force,omitempty"`
	Profile  string         `yaml:"profile,omitempty" json:"profile,omitempty"`
	Pinned   []string       `yaml:"pinned,omitempty" json:"pinned,omitempty"`
	Groups   []BrowserGroup `yaml:"groups,omitempty" json:"groups,omitempty"`
	URLs     []string       `yaml:"urls,omitempty" json:"urls,omitempty"`
}

// BrowserGroup defines a named set of URLs in a Firefox tab group.
type BrowserGroup struct {
	Name      string   `yaml:"name" json:"name"`
	Color     string   `yaml:"color,omitempty" json:"color,omitempty"`
	Collapsed bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
	URLs      []string `yaml:"urls" json:"urls"`
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ methods                                                                      │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// AllURLs returns every URL in open order: pinned, groups, then loose URLs.
func (b *BrowserConfig) AllURLs() []string {
	var urls []string
	urls = append(urls, b.Pinned...)
	for _, g := range b.Groups {
		urls = append(urls, g.URLs...)
	}
	urls = append(urls, b.URLs...)
	return urls
}

// MonocleSize returns the monocle window dimensions (px).
func (c *HyprConfig) MonocleSize() (w, h int) {
	return c.Windows.Monocle.Width, c.Windows.Monocle.Height
}

// MonocleOffset returns the x/y offset (px) from monitor center.
func (c *HyprConfig) MonocleOffset() (x, y int) {
	return c.Windows.Monocle.OffsetX, c.Windows.Monocle.OffsetY
}

// IsIgnored reports whether the window class is in IgnoredClasses.
func (c *HyprConfig) IsIgnored(class string) bool {
	return slices.Contains(c.Windows.IgnoredClasses, class)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ defaults                                                                     │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// DefaultHypr returns hyprd defaults.
func DefaultHypr() HyprConfig {
	return HyprConfig{
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
			FocusApps: map[string]string{
				"Slack":                     "slack",
				"firefox-developer-edition": "firefox-developer-edition",
			},
			ActionFocusApps: map[string]NotifyFocusTarget{
				"Firefox":                   {Workspace: 2, Class: "firefox-developer-edition"},
				"Firefox Developer Edition": {Workspace: 2, Class: "firefox-developer-edition"},
				"firefox-developer-edition": {Workspace: 2, Class: "firefox-developer-edition"},
			},
			SilentApps:          []string{"Spotify", "discord"},
			KittySilentPatterns: []string{"claude", "opencode", "approval"},
		},
		Windows: WindowsConfig{
			IgnoredClasses:  []string{"GLava"},
			HiddenWorkspace: "special:hiddenSlaves",
			ShadowWorkspace: "special:shadow",
			Split: SplitConfig{
				XS:      "0.37",
				Default: "0.4942",
				LG:      "0.77",
			},
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
