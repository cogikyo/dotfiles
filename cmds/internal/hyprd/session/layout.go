package session

// layout.go opens configured workspace sessions, launches body windows, and applies initial layout state.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/browser"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

const (
	sessionWindowTimeout       = 5 * time.Second
	sessionBrowserClaimTimeout = 5 * time.Second
)

// Layout opens and arranges per-workspace sessions defined in config.
type Layout struct {
	hypr  *hypr.Client
	state *state.State
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
		l.execOnWorkspace(s.Workspace, s.Command)
		roles := []string{s.Name}

		if s.Browser.Snapshot != "" || len(s.Browser.AllURLs()) > 0 {
			if err := l.launchSessionBrowser(s); err != nil {
				return "", err
			}
			roles = append(roles, "browser")
		}

		windowsByRole := l.waitForSessionRoles(s, roles, sessionWindowTimeout)
		if commandWindow := windowsByRole[s.Name]; commandWindow != nil {
			l.arrangePair(s, commandWindow, windowsByRole["browser"])
		}

		if s.Monocle {
			l.applyMonocle(s.Workspace)
		}
		return l.sessionResult(s, roles, windowsByRole), nil
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
		l.execOnWorkspace(s.Workspace, cmd)
	}
	windowsByRole := l.waitForSessionRoles(s, s.Body, sessionWindowTimeout)

	l.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", cfg.Windows.Split.Default))
	l.arrangeThreeBody(s, windowsByRole)

	if s.Monocle {
		l.applyMonocle(s.Workspace)
	}

	return l.sessionResult(s, s.Body, windowsByRole), nil
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

	if b.UsesExactRestore(s.Browser) {
		// Exact restores use a per-session Firefox profile, so existing windows can be claimed across restarts.
		if err := l.claimManagedBrowserWindow(b, s); err == nil {
			return nil
		}
		if _, err := b.RestoreConfiguredSnapshotForSession(s.Name, s.Browser, false); err != nil {
			return err
		}
		if err := l.claimManagedBrowserWindow(b, s); err != nil {
			fmt.Fprintf(os.Stderr, "layout: claim browser window for %s: %v\n", s.Name, err)
		}
		return nil
	}
	return l.launchSessionBrowserURLs(b, s)
}

func (l *Layout) launchSessionBrowserURLs(b *browser.Browser, s config.Session) error {
	tbw := config.ThreeBody["browser"]

	browserCfg, err := b.ResolveLaunchConfig(s.Browser)
	if err != nil {
		return err
	}
	urls := browserCfg.AllURLs()
	if len(urls) > 0 {
		l.execOnWorkspace(s.Workspace, browserLaunchCmd(tbw.Command, "new-window", urls[0]))
		time.Sleep(500 * time.Millisecond)
		for _, url := range urls[1:] {
			l.execOnWorkspace(s.Workspace, browserLaunchCmd(tbw.Command, "new-tab", url))
			time.Sleep(300 * time.Millisecond)
		}
	} else {
		l.execOnWorkspace(s.Workspace, browserLaunchCmd(tbw.Command, "new-window", "about:blank"))
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func (l *Layout) execOnWorkspace(workspace int, cmd string) {
	l.hypr.Dispatch(fmt.Sprintf("exec [workspace %d silent] %s", workspace, cmd))
}

func (l *Layout) claimBrowserWindow(b *browser.Browser, s config.Session) error {
	deadline := time.Now().Add(sessionBrowserClaimTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := b.ClaimWindow(s.Browser.Snapshot, s.Workspace); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return lastErr
}

func (l *Layout) claimManagedBrowserWindow(b *browser.Browser, s config.Session) error {
	deadline := time.Now().Add(sessionBrowserClaimTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := b.ClaimWindowForSession(s.Browser.Snapshot, s.Name, s.Browser, s.Workspace); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return lastErr
}

func (l *Layout) waitForSessionRoles(s config.Session, roles []string, timeout time.Duration) map[string]*hypr.Window {
	found := make(map[string]*hypr.Window)
	deadline := time.Now().Add(timeout)
	for {
		clients, err := l.hypr.Clients()
		if err == nil {
			for _, role := range roles {
				if found[role] != nil {
					continue
				}
				for i := range clients {
					c := &clients[i]
					if c.Workspace.ID != s.Workspace || c.Pinned || windows.IsIgnored(c.Class) {
						continue
					}
					if l.matchesRole(s, c, role) {
						found[role] = c
						break
					}
				}
			}
			if len(found) == len(roles) {
				return found
			}
		}
		if !time.Now().Before(deadline) {
			return found
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (l *Layout) matchesRole(s config.Session, w *hypr.Window, role string) bool {
	if role == "browser" {
		return strings.Contains(strings.ToLower(w.Class), "firefox")
	}
	if tbw, ok := config.ThreeBody[role]; ok {
		return windows.MatchesTarget(w, tbw.Class, tbw.Title)
	}
	return strings.EqualFold(w.Class, role) || strings.EqualFold(w.Class, commandName(s.Command))
}

func commandName(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}

func (l *Layout) sessionResult(s config.Session, roles []string, found map[string]*hypr.Window) string {
	missing := missingRoles(roles, found)
	if len(missing) == 0 {
		return fmt.Sprintf("opened session: %s on ws%d (%d/%d windows)", s.Name, s.Workspace, len(found), len(roles))
	}
	return fmt.Sprintf("opened session: %s on ws%d (%d/%d windows; missing %s)", s.Name, s.Workspace, len(found), len(roles), strings.Join(missing, ","))
}

func missingRoles(roles []string, found map[string]*hypr.Window) []string {
	var missing []string
	for _, role := range roles {
		if found[role] == nil {
			missing = append(missing, role)
		}
	}
	return missing
}

func (l *Layout) arrangePair(s config.Session, master, browserWindow *hypr.Window) {
	if master == nil || browserWindow == nil {
		return
	}
	l.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", s.Workspace, master.Address))
	l.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", s.Workspace, browserWindow.Address))
	l.ensureMaster(s.Workspace, master.Address)
}

func (l *Layout) arrangeThreeBody(s config.Session, windowsByRole map[string]*hypr.Window) {
	masterRole, slaveRole, shadowRole := l.initialRoles(s)
	master := windowsByRole[masterRole]
	slave := windowsByRole[slaveRole]
	shadow := windowsByRole[shadowRole]
	if master == nil || slave == nil {
		return
	}

	l.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", s.Workspace, master.Address))
	l.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", s.Workspace, slave.Address))

	if shadow != nil {
		l.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent special:shadow,address:%s", shadow.Address))
	}
	l.ensureMaster(s.Workspace, master.Address)
	l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", slave.Address))
	if shadow != nil {
		l.state.SetThreeBody(s.Workspace, &state.ThreeBodyState{Master: master.Address, Active: slave.Address, Shadow: shadow.Address})
	}
}

func (l *Layout) ensureMaster(workspace int, address string) {
	current, err := windows.GetMaster(l.hypr, workspace)
	if err != nil || current == nil || current.Address == address {
		return
	}
	if !l.focusWindow(address, 500*time.Millisecond) {
		return
	}
	l.hypr.Dispatch("layoutmsg swapwithmaster master")
}

func (l *Layout) focusWindow(address string, timeout time.Duration) bool {
	l.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", address))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		active, err := l.hypr.ActiveWindow()
		if err == nil && active != nil && active.Address == address {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func (l *Layout) initialRoles(s config.Session) (master, slave, shadow string) {
	master = s.Layout.Master
	slave = s.Layout.Slave
	shadow = s.Layout.Shadow
	if master == "" && len(s.Body) > 0 {
		master = s.Body[0]
	}
	if slave == "" && len(s.Body) > 1 {
		slave = s.Body[1]
	}
	if shadow == "" && len(s.Body) > 2 {
		shadow = s.Body[2]
	}
	return master, slave, shadow
}

func browserLaunchCmd(cmd, mode, url string) string {
	if url == "" {
		url = "about:blank"
	}
	return fmt.Sprintf("%s --%s %q", cmd, mode, url)
}
