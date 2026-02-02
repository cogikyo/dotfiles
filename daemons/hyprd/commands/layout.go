package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dotfiles/daemons/hyprd/hypr"

	"gopkg.in/yaml.v3"
)

// SessionConfig is the root structure for sessions.yaml.
type SessionConfig struct {
	Sessions map[string]Session `yaml:"sessions"` // keyed by session name
}

// Session defines a workspace layout for automated window spawning and arrangement.
type Session struct {
	Name      string         `yaml:"name" json:"name"`           // display name
	Workspace int            `yaml:"workspace" json:"workspace"` // target workspace ID
	Project   string         `yaml:"project" json:"project"`     // sets PROJECT_PATH for kitty sessions
	URLs      []string       `yaml:"urls" json:"urls"`           // opened in firefox
	Windows   []WindowConfig `yaml:"windows" json:"windows"`     // spawned and arranged by role
}

// WindowConfig defines a window to spawn and its position in the master/slave layout.
type WindowConfig struct {
	Command string `yaml:"command"` // shell command to execute (empty means no spawn)
	Title   string `yaml:"title"`   // used to identify window for arrangement
	Role    string `yaml:"role"`    // "master" or "slave"
}

// DefaultSessions provides fallback configurations when ~/.config/hyprd/sessions.yaml is missing or invalid.
var DefaultSessions = map[string]Session{
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
		Project:   "acr",
		URLs:      []string{"localhost:3002"},
		Windows: []WindowConfig{
			{Command: "kitty --title terminal", Title: "terminal", Role: "master"},
			{Title: "firefox", Role: "slave"},
			{Command: "kitty --title claude", Title: "claude", Role: "slave"},
		},
	},
	"nosvagor": {
		Name:      "nosvagor",
		Workspace: 3,
		Project:   "nosvagor.com",
		URLs:      []string{"localhost:3000"},
		Windows: []WindowConfig{
			{Command: "kitty --title terminal", Title: "terminal", Role: "master"},
			{Title: "firefox", Role: "slave"},
			{Command: "kitty --title claude", Title: "claude", Role: "slave"},
		},
	},
}

// Layout manages session-based workspace layouts with automatic window spawning and arrangement.
type Layout struct {
	hypr  *hypr.Client  // Hyprland IPC client
	state StateManager  // shared daemon state
}

// NewLayout creates a Layout command handler.
func NewLayout(h *hypr.Client, s StateManager) *Layout {
	return &Layout{hypr: h, state: s}
}

// Execute opens the named session or lists available sessions (when arg is empty, "--list", or "-l").
func (l *Layout) Execute(arg string) (string, error) {
	sessions := LoadSessions()

	if arg == "" || arg == "--list" || arg == "-l" {
		return listSessions(sessions), nil
	}

	session, ok := sessions[arg]
	if !ok {
		return "", fmt.Errorf("unknown session: %s (use --list to see available)", arg)
	}

	return l.openSession(session)
}

// LoadSessions reads ~/.config/hyprd/sessions.yaml, falling back to DefaultSessions on error.
func LoadSessions() map[string]Session {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultSessions
	}

	configPath := filepath.Join(homeDir, ".config", "hyprd", "sessions.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultSessions
	}

	var config SessionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: failed to parse sessions.yaml: %v\n", err)
		return DefaultSessions
	}

	if len(config.Sessions) == 0 {
		return DefaultSessions
	}

	return config.Sessions
}

// listSessions formats session names with workspace IDs for display.
func listSessions(sessions map[string]Session) string {
	var names []string
	for name, s := range sessions {
		names = append(names, fmt.Sprintf("%s (ws%d)", name, s.Workspace))
	}
	sort.Strings(names)
	return "sessions: " + strings.Join(names, ", ")
}

// openSession spawns windows, opens URLs, and arranges the layout on the target workspace.
func (l *Layout) openSession(s Session) (string, error) {
	// Switch to workspace
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", s.Workspace))

	// Check if workspace already has windows
	clients, err := l.hypr.Clients()
	if err != nil {
		return "", err
	}

	var wsWindows []hypr.Window
	for _, c := range clients {
		if c.Workspace.ID == s.Workspace && !c.Pinned && c.Class != "GLava" {
			wsWindows = append(wsWindows, c)
		}
	}

	if len(wsWindows) > 0 {
		return fmt.Sprintf("ws%d already has %d windows", s.Workspace, len(wsWindows)), nil
	}

	homeDir, _ := os.UserHomeDir()

	// Spawn windows defined in config
	for _, w := range s.Windows {
		if w.Command == "" {
			continue
		}
		cmd := w.Command
		if s.Project != "" && strings.Contains(cmd, "kitty") {
			// Set PROJECT_PATH for kitty sessions
			cmd = fmt.Sprintf("env PROJECT_PATH=%s/%s %s", homeDir, s.Project, cmd)
		}
		l.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(500 * time.Millisecond)
	}

	// Open browser with URLs
	if len(s.URLs) > 0 {
		l.hypr.Dispatch(fmt.Sprintf("exec firefox-developer-edition --new-window '%s'", s.URLs[0]))
		time.Sleep(500 * time.Millisecond)

		for _, url := range s.URLs[1:] {
			l.hypr.Dispatch(fmt.Sprintf("exec firefox-developer-edition '%s'", url))
			time.Sleep(300 * time.Millisecond)
		}
	}

	// Wait for windows to appear then arrange
	time.Sleep(1500 * time.Millisecond)
	l.arrangeLayout(s.Workspace, s)

	return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
}

// arrangeLayout moves terminal to master and orders firefox/claude in the slave stack.
func (l *Layout) arrangeLayout(wsID int, session Session) {
	clients, err := l.hypr.Clients()
	if err != nil {
		return
	}

	// Find windows by title/class
	var terminal, firefox, claude *hypr.Window
	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID != wsID || c.Floating || c.Class == "GLava" {
			continue
		}

		// Match by title first
		switch c.Title {
		case "terminal":
			terminal = c
		case "claude":
			claude = c
		}

		// Firefox matched by class (has dynamic titles)
		if strings.Contains(strings.ToLower(c.Class), "firefox") {
			firefox = c
		}
	}

	// Swap terminal to master position
	if terminal != nil {
		l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", terminal.Address))
		l.hypr.Dispatch("layoutmsg swapwithmaster")
		time.Sleep(100 * time.Millisecond)
	}

	// Ensure firefox is above claude in slave stack
	if firefox != nil && claude != nil {
		// Re-query positions
		clients, err = l.hypr.Clients()
		if err != nil {
			return
		}
		for i := range clients {
			c := &clients[i]
			if c.Address == firefox.Address {
				firefox = c
			}
			if c.Address == claude.Address {
				claude = c
			}
		}

		if claude.At[1] < firefox.At[1] {
			l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", claude.Address))
			l.hypr.Dispatch("layoutmsg swapnext")
		}
	}

	// Focus master
	if terminal != nil {
		l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", terminal.Address))
	}

	// Apply default split
	cfg := l.state.GetConfig()
	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", cfg.Split.Default))
}
