package session

// layout.go opens configured workspace sessions, launches body windows, and applies initial layout state.

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

// Layout opens and arranges per-workspace sessions defined in config.
type Layout struct {
	hypr        *hypr.Client
	state       *state.State
	skipBrowser map[string]bool
}

// SkipBrowser marks sessions whose browser was already restored (e.g. by batch exact restore during init).
// Layout will claim the existing Firefox window instead of launching a new one.
func (l *Layout) SkipBrowser(names ...string) {
	if l.skipBrowser == nil {
		l.skipBrowser = make(map[string]bool)
	}
	for _, n := range names {
		l.skipBrowser[n] = true
	}
}

func NewLayout(h *hypr.Client, s *state.State) *Layout {
	return &Layout{hypr: h, state: s}
}

// Execute dispatches: "list", "set <ws> <name>", a workspace number, or a session name.
func (l *Layout) Execute(arg string) (string, error) {
	cfg := l.state.GetConfig()
	sessions := cfg.Sessions

	parts := strings.Fields(arg)
	if len(parts) == 0 || arg == "--list" || arg == "-l" || arg == "list" {
		return l.listByWorkspace(sessions), nil
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

func (l *Layout) setActive(args []string, sessions config.SessionsConfig) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: layout set <workspace> <session>")
	}
	ws, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", args[0])
	}
	name := args[1]
	session, ok := sessions[name]
	if !ok {
		return "", fmt.Errorf("unknown session: %s", name)
	}
	if session.Workspace != ws {
		return "", fmt.Errorf("session %q belongs to ws%d, not ws%d", name, session.Workspace, ws)
	}
	l.state.SetActiveSession(ws, name)
	return fmt.Sprintf("ws%d active: %s", ws, name), nil
}

func (l *Layout) openByWorkspace(ws int, sessions config.SessionsConfig) (string, error) {
	name := l.state.GetActiveSession(ws)
	if name == "" {
		return "", fmt.Errorf("no active session for ws%d (use 'layout set %d <session>')", ws, ws)
	}
	session, ok := sessions[name]
	if !ok {
		return "", fmt.Errorf("active session %q for ws%d not found in config", name, ws)
	}
	if session.Workspace != ws {
		return "", fmt.Errorf("active session %q belongs to ws%d, not ws%d", name, session.Workspace, ws)
	}
	return l.openSession(session)
}

func (l *Layout) listByWorkspace(sessions config.SessionsConfig) string {
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
		activeName := l.state.GetActiveSession(ws)
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
	for _, c := range clients {
		if c.Workspace.ID == s.Workspace && !c.Pinned && !windows.IsIgnored(c.Class) {
			l.hypr.Dispatch(fmt.Sprintf("closewindow address:%s", c.Address))
		}
	}
	time.Sleep(300 * time.Millisecond)

	homeDir, _ := os.UserHomeDir()
	if s.Project != "" {
		l.state.SetProjectPath(s.Workspace, fmt.Sprintf("%s/%s", homeDir, s.Project))
	}

	if s.Command != "" {
		l.hypr.Dispatch(fmt.Sprintf("exec %s", s.Command))

		if s.Browser.Snapshot != "" || len(s.Browser.AllURLs()) > 0 {
			time.Sleep(500 * time.Millisecond)
			if err := l.launchSessionBrowser(s); err != nil {
				return "", err
			}
		}

		if s.Monocle {
			time.Sleep(1500 * time.Millisecond)
			l.applyMonocle(s.Workspace)
		}
		return fmt.Sprintf("opened session: %s on ws%d", s.Name, s.Workspace), nil
	}
	if len(s.Body) == 0 {
		return "", fmt.Errorf("session %q has no body or command", s.Name)
	}

	for _, name := range s.Body {
		tbw, ok := config.ThreeBody[name]
		if !ok {
			return "", fmt.Errorf("session %q references unknown three-body window %q", s.Name, name)
		}
		if name == "browser" || strings.Contains(strings.ToLower(tbw.Class), "firefox") {
			if err := l.launchSessionBrowser(s); err != nil {
				return "", err
			}
			continue
		}

		cmd := l.withSessionLaunchEnv(s, name, tbw.Command, homeDir)
		l.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(1500 * time.Millisecond)
	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", cfg.Windows.Split.Default))

	if first, ok := config.ThreeBody[s.Body[0]]; ok {
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

func (l *Layout) launchSessionBrowser(s config.Session) error {
	b := browser.NewBrowser(l.hypr, l.state)

	if l.skipBrowser[s.Name] && s.Browser.Snapshot != "" {
		if err := b.ClaimWindow(s.Browser.Snapshot, s.Workspace); err != nil {
			fmt.Fprintf(os.Stderr, "layout: claim browser window for %s: %v\n", s.Name, err)
		}
		return nil
	}

	if b.UsesExactRestore(s.Browser) {
		if _, err := b.RestoreConfiguredSnapshot(s.Browser, false); err != nil {
			return err
		}
		time.Sleep(1500 * time.Millisecond)
		return nil
	}

	tbw := config.ThreeBody["browser"]

	browserCfg, err := b.ResolveLaunchConfig(s.Browser)
	if err != nil {
		return err
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
	return nil
}

func browserLaunchCmd(cmd, mode, url string) string {
	if url == "" {
		url = "about:blank"
	}
	return fmt.Sprintf("%s --%s %q", cmd, mode, url)
}
