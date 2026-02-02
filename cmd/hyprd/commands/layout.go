package commands

// ================================================================================
// Session layout management with YAML configuration
// ================================================================================

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dotfiles/cmd/hyprd/hypr"

	"gopkg.in/yaml.v3"
)

// SessionConfig is the root of sessions.yaml.
type SessionConfig struct {
	Sessions map[string]Session `yaml:"sessions"`
}

// Session defines a workspace layout configuration.
type Session struct {
	Name      string         `yaml:"name" json:"name"`
	Workspace int            `yaml:"workspace" json:"workspace"`
	Project   string         `yaml:"project" json:"project"`
	URLs      []string       `yaml:"urls" json:"urls"`
	Windows   []WindowConfig `yaml:"windows" json:"windows"`
}

// WindowConfig defines a window to spawn and its role.
type WindowConfig struct {
	Command string `yaml:"command"` // e.g., "kitty --title terminal"
	Title   string `yaml:"title"`   // e.g., "terminal" (for detection)
	Role    string `yaml:"role"`    // "master" | "slave"
}

// DefaultSessions provides built-in session configurations.
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

// Layout handles the layout command execution.
type Layout struct {
	hypr  *hypr.Client
	state StateManager
}

// NewLayout creates a layout command handler.
func NewLayout(h *hypr.Client, s StateManager) *Layout {
	return &Layout{hypr: h, state: s}
}

// Execute runs the layout command.
// Args: session name, or "--list" to list sessions
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

// LoadSessions loads session config from YAML file, falling back to defaults.
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

// listSessions returns available session names.
func listSessions(sessions map[string]Session) string {
	var names []string
	for name, s := range sessions {
		names = append(names, fmt.Sprintf("%s (ws%d)", name, s.Workspace))
	}
	sort.Strings(names)
	return "sessions: " + strings.Join(names, ", ")
}

// openSession opens a session layout.
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

// arrangeLayout arranges windows: terminal → master, firefox/claude → slaves.
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
	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", SplitDefault))
}
