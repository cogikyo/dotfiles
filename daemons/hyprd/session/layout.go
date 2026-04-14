package session

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/browser"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

type Layout struct {
	hypr  *hypr.Client
	state *state.State
}

func NewLayout(h *hypr.Client, s *state.State) *Layout {
	return &Layout{hypr: h, state: s}
}

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
	if parts[0] == "set" {
		return l.setActive(parts[1:], sessions)
	}
	if ws, err := strconv.Atoi(parts[0]); err == nil {
		return l.openByWorkspace(ws, sessions)
	}
	session, ok := sessions[parts[0]]
	if !ok {
		return "", fmt.Errorf("unknown session: %s (use 'layout list')", parts[0])
	}
	return l.openSession(session)
}

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

func (l *Layout) openByWorkspace(ws int, sessions map[string]config.Session) (string, error) {
	name := l.state.GetActiveSession(ws)
	if name == "" {
		return "", fmt.Errorf("no active session for ws%d (use 'layout set %d <session>')", ws, ws)
	}
	session, ok := sessions[name]
	if !ok {
		return "", fmt.Errorf("active session %q for ws%d not found in config", name, ws)
	}
	session.Workspace = ws
	return l.openSession(session)
}

func (l *Layout) listByWorkspace(sessions map[string]config.Session, active map[int]config.ActiveSession) string {
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
		activeName := active[ws].Session
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

func (l *Layout) openSession(s config.Session) (string, error) {
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", s.Workspace))
	l.state.SetActiveSession(s.Workspace, s.Name)

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
	if s.Project != "" {
		l.state.SetProjectPath(s.Workspace, fmt.Sprintf("%s/%s", homeDir, s.Project))
	}

	if s.Command != "" {
		l.hypr.Dispatch(fmt.Sprintf("exec %s", s.Command))
		if s.Monocle {
			time.Sleep(1500 * time.Millisecond)
			l.applyMonocle(s.Workspace)
		}
		return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
	}
	if len(s.Body) == 0 {
		return "", fmt.Errorf("session %q has no body or command", s.Name)
	}

	b := browser.NewBrowser(l.hypr, l.state)
	for _, name := range s.Body {
		tbw, ok := cfg.ThreeBody[name]
		if !ok {
			return "", fmt.Errorf("session %q references unknown three-body window %q", s.Name, name)
		}
		if name == "browser" || strings.Contains(strings.ToLower(tbw.Class), "firefox") {
			if b.UsesExactRestore(s.Browser) {
				if _, err := b.RestoreConfiguredSnapshot(s.Browser, false); err != nil {
					return "", err
				}
				time.Sleep(1500 * time.Millisecond)
			} else {
				browserCfg, err := b.ResolveLaunchConfig(s.Browser)
				if err != nil {
					return "", err
				}
				urls := browserCfg.AllURLs()
				if len(urls) > 0 {
					l.hypr.Dispatch(fmt.Sprintf("exec %s", browserLaunchCmd(tbw.Command, "new-window", urls[0])))
					time.Sleep(500 * time.Millisecond)
					for _, url := range urls[1:] {
						l.hypr.Dispatch(fmt.Sprintf("exec %s", browserLaunchCmd(tbw.Command, "new-tab", url)))
						time.Sleep(300 * time.Millisecond)
					}
				} else {
					l.hypr.Dispatch(fmt.Sprintf("exec %s", browserLaunchCmd(tbw.Command, "new-window", "about:blank")))
					time.Sleep(500 * time.Millisecond)
				}
			}
			continue
		}

		cmd := l.withSessionLaunchEnv(s, name, tbw.Command, homeDir)
		l.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(1500 * time.Millisecond)
	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", cfg.Split.Default))

	if first, ok := cfg.ThreeBody[s.Body[0]]; ok {
		clients, _ = l.hypr.Clients()
		for i := range clients {
			c := &clients[i]
			if c.Workspace.ID == s.Workspace && windows.MatchesTarget(c, first.Class, first.Title) {
				l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
				break
			}
		}
	}

	if s.Monocle {
		l.applyMonocle(s.Workspace)
	}

	return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
}

// applyMonocle floats + resizes the active window on wsID to the configured monocle
// dimensions and tracks the state so `hyprd monocle` can toggle it off later.
func (l *Layout) applyMonocle(wsID int) {
	active, err := l.hypr.ActiveWindow()
	if err != nil || active == nil {
		return
	}

	cfg := l.state.GetConfig()
	w, h := cfg.MonocleSize()
	ox, oy := cfg.MonocleOffset()
	batch := fmt.Sprintf(
		"dispatch togglefloating; dispatch resizeactive exact %d %d; dispatch centerwindow; dispatch moveactive %d %d",
		w, h, ox, oy,
	)
	l.hypr.Request("[[BATCH]]" + batch)
	windows.CenterCursor(l.hypr)

	l.state.SetMonocle(wsID, &state.MonocleState{
		Focused: active.Address,
		Master:  active.Address,
	})
}

func (l *Layout) withSessionLaunchEnv(s config.Session, bodyName, cmd, homeDir string) string {
	if !strings.Contains(cmd, "kitty") || !strings.Contains(cmd, "--session") {
		return cmd
	}

	var env []string
	if s.Project != "" {
		env = append(env, fmt.Sprintf("PROJECT_PATH=%s/%s", homeDir, s.Project))
	}
	if profile := s.Tabs[bodyName]; profile != "" {
		env = append(env, "HYPRD_TAB_PROFILE="+profile)
	}
	if len(env) == 0 {
		return cmd
	}

	return fmt.Sprintf("env %s %s", strings.Join(env, " "), cmd)
}

func browserLaunchCmd(cmd, mode, url string) string {
	if url == "" {
		url = "about:blank"
	}
	return fmt.Sprintf("%s --%s %q", cmd, mode, url)
}
