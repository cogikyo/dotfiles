package config

import (
	"slices"
	"time"
)

// HyprConfig configures hyprd window/session behavior.
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

// ActiveSession binds a session to a workspace and controls whether it opens on boot.
type ActiveSession struct {
	Session string `yaml:"session"`
	Init    bool   `yaml:"init"`
}

// InitConfig controls the one-time boot sequence that runs when hyprd starts.
// Sessions to open on boot come from ActiveSessions entries with init: true.
type InitConfig struct {
	Execs          []string      `yaml:"execs"`
	Workspace      int           `yaml:"workspace"`
	Lock           bool          `yaml:"lock"`
	LockDelay      time.Duration `yaml:"lock_delay"`
	Greeting       string        `yaml:"greeting"`
	NetworkTimeout int           `yaml:"network_timeout"`
}

// BackgroundConfig controls mpvpaper video wallpaper.
type BackgroundConfig struct {
	Display   string    `yaml:"display"`
	VideoPath string    `yaml:"video_path"`
	Socket    string    `yaml:"socket"`
	Wallpaper Wallpaper `yaml:"wallpaper"`
}

// Wallpaper defines the video file and mpv visual properties.
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

// ReservedConfig specifies screen edges to exclude from layout calculations.
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

// NotifyConfig defines notification routing, presentation, and sound policy.
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

// WindowsConfig controls which windows hyprd manages.
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

// TabProfile defines a set of kitty tabs managed as a unit.
type TabProfile struct {
	Prefix string   `yaml:"prefix"`
	Focus  string   `yaml:"focus"`
	Tabs   []TabDef `yaml:"tabs"`
}

// TabDef defines a single kitty tab within a profile.
type TabDef struct {
	Name       string    `yaml:"name"`
	Title      string    `yaml:"title"`
	Command    string    `yaml:"command"`
	CWD        string    `yaml:"cwd"`
	CWDResolve string    `yaml:"cwd_resolve"`
	Requires   string    `yaml:"requires"`
	Layout     string    `yaml:"layout"`
	Panes      []TabPane `yaml:"panes"`
}

// TabPane defines an extra pane to create inside a kitty tab after the primary pane.
type TabPane struct {
	Command    string `yaml:"command"`
	CWD        string `yaml:"cwd"`
	CWDResolve string `yaml:"cwd_resolve"`
	Location   string `yaml:"location"`
	Bias       int    `yaml:"bias"`
}

// ThreeBodyWindow defines a window that participates in the three-body layout.
type ThreeBodyWindow struct {
	Class   string `yaml:"class"`
	Title   string `yaml:"title"`
	Command string `yaml:"command"`
}

// Session defines a workspace layout for automated window spawning and arrangement.
type Session struct {
	Name      string        `yaml:"name" json:"name"`
	Workspace int           `yaml:"workspace" json:"workspace"`
	Project   string        `yaml:"project" json:"project"`
	Body      []string      `yaml:"body" json:"body"`
	Browser   BrowserConfig `yaml:"browser" json:"browser"`
	// Tabs maps three-body members like "editor" to kitty tab profiles like "leadpier".
	Tabs    map[string]string `yaml:"tabs" json:"tabs"`
	Command string            `yaml:"command" json:"command"`
	// Monocle applies monocle sizing to the spawned window after session opens.
	Monocle bool `yaml:"monocle" json:"monocle"`
}

// BrowserConfig declares URLs to open in the browser body member.
type BrowserConfig struct {
	Snapshot string         `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
	Pinned   []string       `yaml:"pinned,omitempty" json:"pinned,omitempty"`
	Groups   []BrowserGroup `yaml:"groups,omitempty" json:"groups,omitempty"`
	URLs     []string       `yaml:"urls,omitempty" json:"urls,omitempty"`
}

// BrowserGroup defines a named set of URLs that belong together in a Firefox tab group.
type BrowserGroup struct {
	Name      string   `yaml:"name" json:"name"`
	Color     string   `yaml:"color,omitempty" json:"color,omitempty"`
	Collapsed bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
	URLs      []string `yaml:"urls" json:"urls"`
}

// AllURLs returns all URLs flattened in open order.
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
			Greeting:       "こんにちは",
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
		Tabs: map[string]TabProfile{
			"editor": {
				Prefix: "ed-",
				Focus:  "nvim",
				Tabs: []TabDef{
					{Name: "xplr", Title: "⟜ 󰭎  ⊸", Command: "xplr"},
					{Name: "nvim", Title: "⟜   ⊸", Command: "nvim"},
					{Name: "term", Title: "⟜   ⊸"},
					{Name: "build", Title: "⟜   ⊸", Command: "just watch", Requires: "justfile"},
					{Name: "git", Title: "⟜   ⊸", Command: "lazygit", Requires: "git"},
				},
			},
			"agents": {
				Prefix: "agents-",
				Focus:  "master",
				Tabs: []TabDef{
					{Name: "xplr", Title: "⊰   ⊱", Command: "codex"},
					{Name: "master", Title: "⊰ 󰯉  ⊱", Command: "claude"},
					{Name: "dev", Title: "⊰   ⊱", Command: "codex"},
					{Name: "plan", Title: "⊰   ⊱", Command: "claude"},
					{Name: "git", Title: "⊰   ⊱", Command: "claude"},
				},
			},
			"leadpier": {
				Prefix: "lp-",
				Focus:  "fe-nvim",
				Tabs: []TabDef{
					{Name: "fe-nvim", Title: "«   »", Command: "nvim", CWD: "~/LeadPier/frontend"},
					{Name: "be-nvim", Title: "«   »", Command: "nvim", CWD: "~/LeadPier/backend"},
					{Name: "nvim", Title: "« 󰠳  »", Command: "nvim", CWD: "~/LeadPier"},
					{Name: "fe-build", Title: "«   »", Command: "yarn dev", CWD: "~/LeadPier/frontend", Layout: "fat:bias=80", Panes: []TabPane{{CWD: "~/LeadPier/frontend"}}},
					{Name: "be-build", Title: "«   »", Command: "pier watch", CWD: "~/LeadPier/backend/core/runner", Layout: "fat:bias=80", Panes: []TabPane{{CWD: "~/LeadPier/backend/core/runner"}}},
				},
			},
		},
		ThreeBody: map[string]ThreeBodyWindow{
			"editor":  {Class: "kitty", Title: "editor", Command: "kitty --title=editor --session ~/.config/kitty/sessions/editor.conf"},
			"agents":  {Class: "kitty", Title: "agents", Command: "kitty --title=agents --session ~/.config/kitty/sessions/agents.conf"},
			"browser": {Class: "firefox-developer-edition", Command: "firefox-developer-edition"},
		},
		Sessions: map[string]Session{
			"slack":     {Name: "slack", Workspace: 2, Command: "slack", Monocle: true},
			"discord":   {Name: "discord", Workspace: 2, Command: "discord", Monocle: true},
			"tableplus": {Name: "tableplus", Workspace: 3, Command: "tableplus"},
			"inkscape":  {Name: "inkscape", Workspace: 3, Command: "inkscape"},
			"obs":       {Name: "obs", Workspace: 3, Command: "obs"},
			"gimp":      {Name: "gimp", Workspace: 3, Command: "gimp"},
			"leadpier": {
				Name:      "leadpier",
				Workspace: 4,
				Project:   "LeadPier",
				Body:      []string{"editor", "browser", "agents"},
				Browser:   BrowserConfig{URLs: []string{"http://localhost:4000"}},
				Tabs:      map[string]string{"editor": "leadpier"},
			},
			"dotfiles": {
				Name:      "dotfiles",
				Workspace: 5,
				Project:   "dotfiles",
				Body:      []string{"editor", "browser", "agents"},
				Browser:   BrowserConfig{URLs: []string{"https://github.com/cogikyo/dotfiles"}},
			},
			"acr": {
				Name:      "acr",
				Workspace: 3,
				Project:   "cogikyo/acr",
				Body:      []string{"editor", "browser", "agents"},
				Browser:   BrowserConfig{URLs: []string{"localhost:3002"}},
			},
			"cogikyo": {
				Name:      "cogikyo",
				Workspace: 4,
				Project:   "cogikyo/cogikyo.com",
				Body:      []string{"editor", "browser", "agents"},
				Browser:   BrowserConfig{URLs: []string{"localhost:3000"}},
			},
		},
		ActiveSessions: map[int]ActiveSession{
			2: {Session: "slack", Init: true},
			3: {Session: "tableplus", Init: true},
			4: {Session: "leadpier", Init: true},
			5: {Session: "dotfiles", Init: true},
		},
	}
}
