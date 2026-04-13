package commands

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
)

// Layout manages session-based workspace layouts with automatic window spawning and arrangement.
type Layout struct {
	hypr  *hypr.Client // Hyprland IPC client
	state StateManager // shared daemon state
}

// NewLayout creates a Layout command handler.
func NewLayout(h *hypr.Client, s StateManager) *Layout {
	return &Layout{hypr: h, state: s}
}

// Execute routes layout subcommands:
//
//	hyprd layout 4          — open the active session for workspace 4
//	hyprd layout set 4 cog  — change ws4's active session to "cog"
//	hyprd layout list       — show all sessions grouped by workspace
//	hyprd layout dotfiles   — open a session by name (legacy/explicit)
func (l *Layout) Execute(arg string) (string, error) {
	cfg := l.state.GetConfig()
	sessions := cfg.Sessions
	if len(sessions) == 0 {
		sessions = config.Default().Hypr.Sessions
	}

	parts := strings.Fields(arg)
	if len(parts) == 0 || arg == "--list" || arg == "-l" || arg == "list" {
		return l.listByWorkspace(sessions, cfg.ActiveSessions), nil
	}

	// "layout set <ws> <session>"
	if parts[0] == "set" {
		return l.setActive(parts[1:], sessions)
	}

	// "layout <number>" — open active session for that workspace
	if ws, err := strconv.Atoi(parts[0]); err == nil {
		return l.openByWorkspace(ws, sessions)
	}

	// "layout <name>" — open session by name (legacy/explicit)
	session, ok := sessions[parts[0]]
	if !ok {
		return "", fmt.Errorf("unknown session: %s (use 'layout list')", parts[0])
	}
	return l.openSession(session)
}

// setActive changes the active session for a workspace.
func (l *Layout) setActive(args []string, sessions map[string]config.Session) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: layout set <workspace> <session>")
	}
	ws, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", args[0])
	}
	name := args[1]
	if _, ok := sessions[name]; !ok {
		return "", fmt.Errorf("unknown session: %s", name)
	}
	l.state.SetActiveSession(ws, name)
	return fmt.Sprintf("ws%d active: %s", ws, name), nil
}

// openByWorkspace resolves the active session for a workspace and opens it.
func (l *Layout) openByWorkspace(ws int, sessions map[string]config.Session) (string, error) {
	name := l.state.GetActiveSession(ws)
	if name == "" {
		return "", fmt.Errorf("no active session for ws%d (use 'layout set %d <session>')", ws, ws)
	}
	session, ok := sessions[name]
	if !ok {
		return "", fmt.Errorf("active session %q for ws%d not found in config", name, ws)
	}
	// Ensure the session targets the right workspace
	session.Workspace = ws
	return l.openSession(session)
}

// listByWorkspace shows sessions grouped by workspace with the active one marked.
func (l *Layout) listByWorkspace(sessions map[string]config.Session, active map[int]string) string {
	// Group sessions by workspace
	byWS := make(map[int][]string)
	for name, s := range sessions {
		byWS[s.Workspace] = append(byWS[s.Workspace], name)
	}

	var lines []string
	var wsNums []int
	for ws := range byWS {
		wsNums = append(wsNums, ws)
	}
	sort.Ints(wsNums)

	for _, ws := range wsNums {
		names := byWS[ws]
		sort.Strings(names)
		activeName := active[ws]
		// Check runtime override
		if rt := l.state.GetActiveSession(ws); rt != "" {
			activeName = rt
		}
		var formatted []string
		for _, n := range names {
			if n == activeName {
				formatted = append(formatted, "*"+n)
			} else {
				formatted = append(formatted, n)
			}
		}
		lines = append(lines, fmt.Sprintf("ws%d: %s", ws, strings.Join(formatted, ", ")))
	}
	return strings.Join(lines, "\n")
}

// openSession spawns windows and arranges the layout on the target workspace.
// Three-body sessions (Body field set) spawn from ThreeBody config and let
// auto-enrollment handle master/slave arrangement. Simple sessions spawn a
// single command.
func (l *Layout) openSession(s config.Session) (string, error) {
	// Switch to workspace
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", s.Workspace))

	// Check if workspace already has windows
	clients, err := l.hypr.Clients()
	if err != nil {
		return "", err
	}

	cfg := l.state.GetConfig()
	var wsWindows []hypr.Window
	for _, c := range clients {
		if c.Workspace.ID == s.Workspace && !c.Pinned && !cfg.IsIgnored(c.Class) {
			wsWindows = append(wsWindows, c)
		}
	}

	if len(wsWindows) > 0 {
		return fmt.Sprintf("ws%d already has %d windows", s.Workspace, len(wsWindows)), nil
	}

	homeDir, _ := os.UserHomeDir()

	// Store project path in state for this workspace
	if s.Project != "" {
		l.state.SetProjectPath(s.Workspace, fmt.Sprintf("%s/%s", homeDir, s.Project))
	}

	// Simple single-command session (slack, tableplus, etc.)
	if s.Command != "" {
		l.hypr.Dispatch(fmt.Sprintf("exec %s", s.Command))
		return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
	}

	// Three-body session — spawn from ThreeBody config
	if len(s.Body) == 0 {
		return "", fmt.Errorf("session %q has no body or command", s.Name)
	}

	urls := s.Browser.AllURLs()

	for _, name := range s.Body {
		tbw, ok := cfg.ThreeBody[name]
		if !ok {
			return "", fmt.Errorf("session %q references unknown three-body window %q", s.Name, name)
		}

		// Browser body member: open URLs instead of (or before) launching
		if name == "browser" || strings.Contains(strings.ToLower(tbw.Class), "firefox") {
			if len(urls) > 0 {
				// First URL opens a new window
				l.hypr.Dispatch(fmt.Sprintf("exec %s '%s'", tbw.Command, urls[0]))
				time.Sleep(500 * time.Millisecond)
				// Remaining URLs open as tabs in that window
				for _, url := range urls[1:] {
					l.hypr.Dispatch(fmt.Sprintf("exec %s '%s'", tbw.Command, url))
					time.Sleep(300 * time.Millisecond)
				}
			} else {
				l.hypr.Dispatch(fmt.Sprintf("exec %s", tbw.Command))
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		// Kitty body members: inject PROJECT_PATH and tab profile
		cmd := tbw.Command
		if s.Project != "" && strings.Contains(cmd, "kitty") {
			cmd = fmt.Sprintf("env PROJECT_PATH=%s/%s %s", homeDir, s.Project, cmd)
		}
		l.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for windows to settle, then apply default split
	time.Sleep(1500 * time.Millisecond)
	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", cfg.Split.Default))

	// Focus the master (first body member, which is first spawned → leftmost)
	if first, ok := cfg.ThreeBody[s.Body[0]]; ok {
		clients, _ = l.hypr.Clients()
		for i := range clients {
			c := &clients[i]
			if c.Workspace.ID == s.Workspace && matchesTarget(c, first.Class, first.Title) {
				l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
				break
			}
		}
	}

	return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
}
