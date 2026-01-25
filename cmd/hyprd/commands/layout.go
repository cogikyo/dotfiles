package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"hyprd/hypr"
)

// Session defines a workspace layout configuration.
type Session struct {
	Name      string   // Session name (e.g., "acr", "dotfiles")
	Workspace int      // Target workspace
	Project   string   // Project path (relative to $HOME)
	URLs      []string // Browser URLs to open
	Terminal  bool     // Whether to open terminal in project
	Editor    string   // Editor to use (default: "cursor")
}

// DefaultSessions provides built-in session configurations.
var DefaultSessions = map[string]Session{
	"acr": {
		Name:      "acr",
		Workspace: 3,
		Project:   "acr",
		URLs:      []string{"localhost:3002"},
		Terminal:  true,
		Editor:    "cursor",
	},
	"dotfiles": {
		Name:      "dotfiles",
		Workspace: 4,
		Project:   "dotfiles",
		URLs:      []string{"https://github.com/cogikyo/dotfiles"},
		Terminal:  true,
		Editor:    "cursor",
	},
	"nosvagor": {
		Name:      "nosvagor",
		Workspace: 3,
		Project:   "nosvagor.com",
		URLs:      []string{"localhost:3000"},
		Terminal:  true,
		Editor:    "cursor",
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
	if arg == "" || arg == "--list" || arg == "-l" {
		return l.listSessions(), nil
	}

	session, ok := DefaultSessions[arg]
	if !ok {
		return "", fmt.Errorf("unknown session: %s (use --list to see available)", arg)
	}

	return l.openSession(session)
}

// listSessions returns available session names.
func (l *Layout) listSessions() string {
	var names []string
	for name, s := range DefaultSessions {
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

	// Open editor
	if s.Editor != "" && s.Project != "" {
		l.hypr.Dispatch(fmt.Sprintf("exec %s $HOME/%s", s.Editor, s.Project))
		time.Sleep(500 * time.Millisecond)
	}

	// Open browser with URLs
	if len(s.URLs) > 0 {
		// First URL opens new window
		l.hypr.Dispatch(fmt.Sprintf("exec firefox-developer-edition --new-window '%s'", s.URLs[0]))
		time.Sleep(500 * time.Millisecond)

		// Remaining URLs open as tabs
		for _, url := range s.URLs[1:] {
			l.hypr.Dispatch(fmt.Sprintf("exec firefox-developer-edition '%s'", url))
			time.Sleep(300 * time.Millisecond)
		}
	}

	// Open terminal
	if s.Terminal && s.Project != "" {
		l.hypr.Dispatch(fmt.Sprintf("exec env PROJECT_PATH=$HOME/%s kitty --session ~/.config/kitty/sessions/default.conf", s.Project))
		time.Sleep(250 * time.Millisecond)
	}

	// Wait for windows to appear then arrange
	time.Sleep(1500 * time.Millisecond)
	l.arrangeLayout(s.Workspace)

	return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
}

// arrangeLayout arranges windows: editor → master, browser → slave1, terminal → slave2
func (l *Layout) arrangeLayout(wsID int) {
	clients, err := l.hypr.Clients()
	if err != nil {
		return
	}

	// Find windows on workspace
	var cursor, firefox, kitty *hypr.Window
	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID != wsID || c.Floating || c.Class == "GLava" {
			continue
		}
		switch {
		case strings.Contains(strings.ToLower(c.Class), "cursor"):
			cursor = c
		case strings.Contains(strings.ToLower(c.Class), "firefox"):
			firefox = c
		case c.Class == "kitty":
			kitty = c
		}
	}

	// Swap cursor to master
	if cursor != nil {
		l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", cursor.Address))
		l.hypr.Dispatch("layoutmsg swapwithmaster")
		time.Sleep(100 * time.Millisecond)
	}

	// Ensure firefox is above kitty in slave stack
	if firefox != nil && kitty != nil {
		// Re-query positions
		clients, _ = l.hypr.Clients()
		for i := range clients {
			c := &clients[i]
			if c.Address == firefox.Address {
				firefox = c
			}
			if c.Address == kitty.Address {
				kitty = c
			}
		}

		if kitty.At[1] < firefox.At[1] {
			l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", kitty.Address))
			l.hypr.Dispatch("layoutmsg swapnext")
		}
	}

	// Focus master
	if cursor != nil {
		l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", cursor.Address))
	}

	// Apply default split
	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", SplitDefault))
}
