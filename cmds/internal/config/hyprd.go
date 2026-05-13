package config

// hyprd.go declares hyprd's window, session, and notification configuration.

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ root config                                                                  │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// HyprConfig configures hyprd window and session behavior.
type HyprConfig struct {
	Background BackgroundConfig      `yaml:"background"`
	Bluetooth  BluetoothConfig       `yaml:"bluetooth"`
	Init       InitConfig            `yaml:"init"`
	Notify     NotifyConfig          `yaml:"notify"`
	VPN        VPNConfig             `yaml:"vpn"`
	Windows    WindowsConfig         `yaml:"windows"`
	Tabs       map[string]TabProfile `yaml:"tabs"`
	Sessions   SessionsConfig        `yaml:"sessions"`
}

// VPNConfig lists NetworkManager VPN profiles that can be loaded from secrets.
type VPNConfig struct {
	Connections map[string]VPNConnection `yaml:"connections"`
}

// VPNConnection describes a NetworkManager VPN profile managed by hyprd.
type VPNConnection struct {
	Profile string `yaml:"profile"` // staged NetworkManager keyfile under $HOME
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ boot / environment                                                           │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// InitConfig controls the one-time boot sequence (env setup before sessions).
//
// Sessions opened on boot are those with init: true in the session catalog.
type InitConfig struct {
	Workspace      int  `yaml:"workspace"`       // workspace to focus after boot
	Lock           bool `yaml:"lock"`            // lock the session after boot completes
	NetworkTimeout int  `yaml:"network_timeout"` // seconds to wait for network before proceeding
}

// BluetoothConfig controls automatic device connection at startup and unlock.
type BluetoothConfig struct {
	Enabled bool   `yaml:"enabled"`
	Device  string `yaml:"device"`
}

// BackgroundConfig controls the mpvpaper video wallpaper.
type BackgroundConfig struct {
	Display   string    `yaml:"display"`    // monitor name, or "auto" to use Hyprland's active output
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
	DefaultVolume       int                    `yaml:"default_volume"`        // percentage (100 = paplay 65536)
	Styles              map[string]VisualStyle `yaml:"styles"`                // named color themes for dunst hints
	AgentEvents         map[string]AgentEvent  `yaml:"agent_events"`          // event class -> style + sound + timing
	UrgencySounds       map[string]string      `yaml:"urgency_sounds"`        // urgency -> sound name (or "none")
	AppSounds           map[string]string      `yaml:"app_sounds"`            // app name -> sound name; takes precedence over urgency
	SilentApps          []string               `yaml:"silent_apps"`           // external app names that suppress sound entirely
	KittySilentPatterns []string               `yaml:"kitty_silent_patterns"` // substrings in kitty notification content that suppress sound
}

// VisualStyle defines dunst color hints for a notification theme.
type VisualStyle struct {
	Background string `yaml:"background"`
	Foreground string `yaml:"foreground"`
	Frame      string `yaml:"frame"`
}

// AgentEvent binds a visual style to sound and timing for an agent event class.
type AgentEvent struct {
	Style   string `yaml:"style"`
	Sound   string `yaml:"sound"`
	Volume  int    `yaml:"volume"`  // percentage (100 = paplay 65536); 0 defers to default_volume
	Timeout int    `yaml:"timeout"` // ms; 0 means persistent
}

// ResolvedStyle flattens an AgentEvent and its VisualStyle for notification dispatch.
type ResolvedStyle struct {
	Sound      string
	Volume     int
	Timeout    int
	Urgency    string
	Persistent bool
	IconSuffix string
	Background string
	Foreground string
	Frame      string
}

// ResolveEvent combines an AgentEvent with its VisualStyle into a dispatch-ready struct.
func (c *NotifyConfig) ResolveEvent(name string) ResolvedStyle {
	event, ok := c.AgentEvents[name]
	if !ok {
		return ResolvedStyle{}
	}
	visual := c.Styles[event.Style]

	urgency := "normal"
	if event.Timeout > 0 {
		urgency = "low"
	}

	return ResolvedStyle{
		Sound:      event.Sound,
		Volume:     event.Volume,
		Timeout:    event.Timeout,
		Urgency:    urgency,
		Persistent: event.Timeout == 0,
		IconSuffix: "-" + event.Style,
		Background: visual.Background,
		Foreground: visual.Foreground,
		Frame:      visual.Frame,
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ windows / workspaces                                                         │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// WindowsConfig controls tiling policy.
type WindowsConfig struct {
	Split   SplitConfig   `yaml:"split"`
	Monocle MonocleConfig `yaml:"monocle"`
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
	Name       string     `yaml:"name"`
	Title      string     `yaml:"title"`
	Command    string     `yaml:"command"`
	CWD        string     `yaml:"cwd"`         // may contain "~/"
	CWDResolve string     `yaml:"cwd_resolve"` // e.g. "git-root"
	Requires   string     `yaml:"requires"`    // must exist in CWD or tab is skipped
	Layout     string     `yaml:"layout"`      // e.g. "fat:bias=80"
	Actions    TabActions `yaml:"actions"`     // semantic actions this tab can satisfy
	Panes      []TabPane  `yaml:"panes"`
}

// TabActions maps semantic tab actions (nvim, git, term) to action-specific behavior.
type TabActions map[string]TabAction

// TabAction configures behavior when a semantic action resolves to a tab.
type TabAction struct {
	Pane int `yaml:"pane"` // zero-based kitty pane index to focus after focusing the tab
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
	Class   string
	Title   string
	Command string
}

// ThreeBody is the fixed set of window roles for three-body layouts.
var ThreeBody = map[string]ThreeBodyWindow{
	"editor":  {Class: "kitty", Title: "editor", Command: "kitty --title=editor --session ~/.config/kitty/sessions/editor.conf"},
	"agents":  {Class: "kitty", Title: "agents", Command: "kitty --title=agents --session ~/.config/kitty/sessions/agents.conf"},
	"browser": {Class: "firefox-developer-edition", Command: "hyprd browser launch"},
}

// SessionsConfig stores runtime-flat sessions keyed by name, while YAML groups them by workspace number first.
type SessionsConfig map[string]Session

// DefaultSession returns the init-marked session name for a workspace, or "" if none.
func (s SessionsConfig) DefaultSession(ws int) string {
	for _, session := range s {
		if session.Workspace == ws && session.Init {
			return session.Name
		}
	}
	return ""
}

// UnmarshalYAML decodes workspace-grouped session YAML into the flat runtime map.
func (s *SessionsConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == 0 || value.Tag == "!!null" {
		*s = nil
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("sessions must be a mapping of workspace -> sessions")
	}

	out := make(SessionsConfig)
	for i := 0; i < len(value.Content); i += 2 {
		wsNode := value.Content[i]
		groupNode := value.Content[i+1]

		ws, err := strconv.Atoi(strings.TrimSpace(wsNode.Value))
		if err != nil {
			return fmt.Errorf("sessions: invalid workspace %q", wsNode.Value)
		}
		if groupNode.Kind != yaml.MappingNode {
			return fmt.Errorf("sessions[%d] must be a mapping of session name -> config", ws)
		}

		for j := 0; j < len(groupNode.Content); j += 2 {
			nameNode := groupNode.Content[j]
			sessionNode := groupNode.Content[j+1]
			name := strings.TrimSpace(nameNode.Value)
			if name == "" {
				return fmt.Errorf("sessions[%d]: session name cannot be empty", ws)
			}
			if _, exists := out[name]; exists {
				return fmt.Errorf("sessions: duplicate session name %q", name)
			}

			var session Session
			if err := sessionNode.Decode(&session); err != nil {
				return fmt.Errorf("sessions[%d][%q]: %w", ws, name, err)
			}
			session.Name = name
			session.Workspace = ws
			out[name] = session
		}
	}

	initPerWS := make(map[int]string)
	for name, session := range out {
		if session.Init {
			if prev, dup := initPerWS[session.Workspace]; dup {
				return fmt.Errorf("sessions[%d]: multiple init sessions: %q and %q", session.Workspace, prev, name)
			}
			initPerWS[session.Workspace] = name
		}
	}

	*s = out
	return nil
}

// Session defines a workspace layout for automated window spawning and arrangement.
type Session struct {
	Name      string            `yaml:"-" json:"name"`      // derived from the session map key
	Workspace int               `yaml:"-" json:"workspace"` // derived from the enclosing workspace key
	Init      bool              `yaml:"init" json:"init"`   // spawn on boot; at most one per workspace
	Project   string            `yaml:"project" json:"project"`
	Body      []string          `yaml:"body" json:"body"` // three-body member order
	Layout    SessionLayout     `yaml:"layout" json:"layout"`
	Browser   BrowserConfig     `yaml:"browser" json:"browser"`
	Tabs      map[string]string `yaml:"tabs" json:"tabs"`       // body member -> tab profile name
	Command   string            `yaml:"command" json:"command"` // single-window sessions (no three-body)
	Class     string            `yaml:"class" json:"class"`     // explicit window class for single-command sessions
	Monocle   bool              `yaml:"monocle" json:"monocle"`
}

// SessionLayout optionally pins initial tiled roles after all session windows map.
type SessionLayout struct {
	Master string `yaml:"master" json:"master"`
	Slave  string `yaml:"slave" json:"slave"`
	Shadow string `yaml:"shadow" json:"shadow"`
}

// BrowserConfig declares a snapshot restore or URL/tab-group launch for the browser body member.
type BrowserConfig struct {
	Snapshot string         `yaml:"snapshot,omitempty" json:"snapshot,omitempty"`
	Mode     string         `yaml:"mode,omitempty" json:"mode,omitempty"`
	Force    bool           `yaml:"force,omitempty" json:"force,omitempty"`
	Pinned   []string       `yaml:"pinned,omitempty" json:"pinned,omitempty"`
	Groups   []BrowserGroup `yaml:"groups,omitempty" json:"groups,omitempty"`
	URLs     []string       `yaml:"urls,omitempty" json:"urls,omitempty"`
}

// UnmarshalYAML accepts `browser: snapshot-name` as the common exact-restore shorthand.
func (b *BrowserConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		b.Snapshot = strings.TrimSpace(value.Value)
		return nil
	}

	type rawBrowserConfig BrowserConfig
	var raw rawBrowserConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*b = BrowserConfig(raw)
	return nil
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
