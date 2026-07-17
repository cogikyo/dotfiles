package session

// tabs.go initializes and refreshes kitty tab layouts from configured tab profiles.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

// Tabs initializes and refreshes the tab layout of a kitty editor window per its config profile.
type Tabs struct {
	hypr  *hypr.Client
	state *state.State
}

func NewTabs(h *hypr.Client, state *state.State) *Tabs {
	return &Tabs{hypr: h, state: state}
}

// Execute dispatches "init <profile> <pid>" or "refresh <name|all> [pid]".
func (t *Tabs) Execute(args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "", fmt.Errorf("usage: tabs init <profile> <pid> | tabs refresh <name|current|all> [pid]")
	}

	switch parts[0] {
	case "init":
		return t.init(parts[1:])
	case "refresh":
		return t.refresh(parts[1:])
	default:
		return "", fmt.Errorf("unknown subcommand: %s", parts[0])
	}
}

func (t *Tabs) init(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: tabs init <profile> <pid>")
	}

	profileName := args[0]
	pid, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid pid: %s", args[1])
	}

	profile, err := t.getProfile(profileName)
	if err != nil {
		return "", err
	}

	kitty := NewKittyClient(pid)
	windows, err := kitty.FullState()
	if err != nil {
		return "", fmt.Errorf("kitty state: %w", err)
	}
	if len(windows) == 0 {
		return "", fmt.Errorf("no kitty windows")
	}

	windowID := windows[0].ID
	defaultCWD := t.resolveDefaultCWD(windows[0])
	if len(args) > 2 && args[2] != "" {
		defaultCWD = config.ExpandPath(args[2])
	}

	created := 0
	for _, tab := range profile.Tabs {
		cwd := t.resolveCWD(tab, defaultCWD)
		if !t.checkRequires(tab.Requires, cwd) {
			continue
		}
		if err := t.launchTab(kitty, profile, tab, windowID, cwd); err != nil {
			return "", err
		}
		created++
	}

	t.closeLauncherTab(kitty, windows[0])
	focusID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, profile.Focus)
	kitty.FocusTab(focusID)

	return fmt.Sprintf("tabs init: %s (%d tabs)", profileName, created), nil
}

func (t *Tabs) refresh(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: tabs refresh <name|current|all> [pid]")
	}

	nameOrAlias := args[0]
	pid, err := t.refreshPID(args)
	if err != nil {
		return "", err
	}

	kitty := NewKittyClient(pid)
	windows, err := kitty.FullState()
	if err != nil {
		return "", fmt.Errorf("kitty state: %w", err)
	}
	if len(windows) == 0 {
		return "", fmt.Errorf("no kitty windows")
	}

	windowID := windows[0].ID
	profileName := detectTabProfile(t.state.GetConfig(), windows[0])
	profile, err := t.getProfile(profileName)
	if err != nil {
		return "", err
	}

	if nameOrAlias == "all" {
		return t.refreshAll(kitty, profile, windowID)
	}

	tabName := resolveTabAlias(t.state.GetConfig(), nameOrAlias, profileName)
	if nameOrAlias == "current" {
		tabName = activeProfileTabName(t.state.GetConfig(), profileName, windows[0])
	}
	tabDef := t.findTab(profile, tabName)
	if tabDef == nil {
		if resolved := pickSemanticTab(profile, normalizeTabAction(nameOrAlias), "", "", ""); resolved != "" {
			tabName = resolved
			tabDef = t.findTab(profile, tabName)
		}
	}
	if tabDef == nil {
		return "", fmt.Errorf("tab %q not in profile %s", tabName, profileName)
	}
	return t.refreshSingle(kitty, profile, *tabDef, windowID)
}

func (t *Tabs) refreshPID(args []string) (int, error) {
	if len(args) >= 2 {
		pid, err := strconv.Atoi(args[1])
		if err != nil {
			return 0, fmt.Errorf("invalid pid: %s", args[1])
		}
		if pid == 0 {
			return t.activeKittyPID()
		}
		return pid, nil
	}

	return t.activeKittyPID()
}

func (t *Tabs) activeKittyPID() (int, error) {
	win, err := t.hypr.ActiveWindow()
	if err != nil {
		return 0, err
	}
	if win == nil || win.Pid == 0 || win.Class != "kitty" {
		return 0, fmt.Errorf("usage: tabs refresh <name|current|all> [pid]")
	}
	return win.Pid, nil
}

func (t *Tabs) refreshAll(kitty *KittyClient, profile *config.TabProfile, windowID int) (string, error) {
	for _, tab := range profile.Tabs {
		tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
		kitty.CloseTab(tabID)
	}

	windows, err := kitty.FullState()
	if err != nil {
		return "", err
	}
	defaultCWD := t.resolveDefaultCWD(windows[0])

	created := 0
	for _, tab := range profile.Tabs {
		cwd := t.resolveCWD(tab, defaultCWD)
		if !t.checkRequires(tab.Requires, cwd) {
			continue
		}
		if err := t.launchTab(kitty, profile, tab, windowID, cwd); err != nil {
			return "", err
		}
		created++
	}

	focusID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, profile.Focus)
	kitty.FocusTab(focusID)
	return fmt.Sprintf("tabs refresh: all (%d tabs)", created), nil
}

func (t *Tabs) refreshSingle(kitty *KittyClient, profile *config.TabProfile, tab config.TabDef, windowID int) (string, error) {
	tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
	origIdx, _ := kitty.TabIndex(tabID)
	defaultCWD := t.resolveDefaultCWDForTab(kitty, tabID)
	kitty.FocusTab(tabID)
	kitty.CloseTab(tabID)

	cwd := t.resolveCWD(tab, defaultCWD)

	if !t.checkRequires(tab.Requires, cwd) {
		return fmt.Sprintf("tabs refresh: %s (skipped, requires %s)", tab.Name, tab.Requires), nil
	}

	if err := t.launchTab(kitty, profile, tab, windowID, cwd); err != nil {
		return "", err
	}

	if origIdx >= 0 {
		newIdx, _ := kitty.TabIndex(tabID)
		if newIdx > origIdx {
			for range newIdx - origIdx {
				kitty.MoveTabBackward()
			}
		}
	}

	return fmt.Sprintf("tabs refresh: %s", tab.Name), nil
}

func (t *Tabs) resolveDefaultCWDForTab(kitty *KittyClient, tabID string) string {
	windows, err := kitty.FullState()
	if err != nil || len(windows) == 0 {
		home, _ := os.UserHomeDir()
		return home
	}

	for _, tab := range windows[0].Tabs {
		if !tabHasID(tab, tabID) {
			continue
		}
		for _, pane := range tab.Windows {
			if paneSelected(pane) && pane.CWD != "" {
				return pane.CWD
			}
		}
		for _, pane := range tab.Windows {
			if pane.CWD != "" {
				return pane.CWD
			}
		}
	}

	return t.resolveDefaultCWD(windows[0])
}

func (t *Tabs) getProfile(name string) (*config.TabProfile, error) {
	cfg := t.state.GetConfig()
	if cfg.Tabs == nil {
		return nil, fmt.Errorf("no tab profiles configured")
	}
	profile, ok := cfg.Tabs[name]
	if !ok {
		return nil, fmt.Errorf("unknown profile: %s", name)
	}
	return &profile, nil
}

func (t *Tabs) resolveDefaultCWD(win KittyOSWindow) string {
	if project := os.Getenv("PROJECT_PATH"); project != "" {
		return config.ExpandPath(project)
	}

	for _, tab := range win.Tabs {
		if !tab.IsFocused {
			continue
		}
		for _, pane := range tab.Windows {
			if pane.IsFocused && pane.CWD != "" {
				home, _ := os.UserHomeDir()
				if pane.CWD != home {
					return pane.CWD
				}
			}
		}
	}

	if pwd := os.Getenv("PWD"); pwd != "" {
		return pwd
	}
	home, _ := os.UserHomeDir()
	return home
}

func (t *Tabs) resolveCWD(tab config.TabDef, defaultCWD string) string {
	return t.resolveBaseCWD(tab.CWD, tab.CWDResolve, defaultCWD)
}

func recentGitChild(parent string) string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}

	var best string
	var bestTime int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(parent, e.Name())
		out, err := exec.Command("git", "-C", child, "log", "-1", "--format=%ct").Output()
		if err != nil {
			continue
		}
		ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			continue
		}
		if ts > bestTime {
			bestTime = ts
			best = child
		}
	}
	return best
}

func (t *Tabs) checkRequires(requires, cwd string) bool {
	switch requires {
	case "":
		return true
	case "justfile":
		_, err := os.Stat(filepath.Join(cwd, "justfile"))
		return err == nil
	case "git":
		return exec.Command("git", "-C", cwd, "rev-parse", "--git-dir").Run() == nil
	default:
		return true
	}
}

func (t *Tabs) launchTab(kitty *KittyClient, profile *config.TabProfile, tab config.TabDef, windowID int, cwd string) error {
	tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
	launchArgs := t.buildLaunchArgs(tab, tabID, cwd)
	if err := kitty.Launch(launchArgs...); err != nil {
		return fmt.Errorf("launch tab %s: %w", tab.Name, err)
	}

	if tab.Layout != "" {
		if err := kitty.GotoLayout(tabID, tab.Layout); err != nil {
			return fmt.Errorf("set layout for tab %s: %w", tab.Name, err)
		}
	}

	for _, pane := range tab.Panes {
		paneCWD := t.resolvePaneCWD(pane, cwd)
		launchArgs = t.buildPaneLaunchArgs(tabID, pane, paneCWD)
		if err := kitty.Launch(launchArgs...); err != nil {
			return fmt.Errorf("launch pane for tab %s: %w", tab.Name, err)
		}
	}

	if tab.Layout != "" && len(tab.Panes) > 0 {
		if err := kitty.GotoLayout(tabID, tab.Layout); err != nil {
			return fmt.Errorf("reapply layout for tab %s: %w", tab.Name, err)
		}
	}

	return nil
}

func (t *Tabs) buildLaunchArgs(tab config.TabDef, tabID, cwd string) []string {
	args := []string{
		"--type=tab",
		"--copy-env",
		"--env", "KITTY_TAB_ID=" + tabID,
		"--tab-title=" + tab.Title,
		"--cwd=" + cwd,
	}
	switch {
	case tab.Command == "xplr":
		args = append(args, "zsh", "-c", `cd "$(xplr --print-pwd-as-result)" 2>/dev/null; exec zsh -l`)
	case tab.Command != "":
		args = append(args, "--env", "HYPRD_LAUNCH_COMMAND="+withResolvedPWD(tab.Command, cwd))
		args = append(args, persistentZshCommand()...)
	}
	return args
}

func (t *Tabs) buildPaneLaunchArgs(tabID string, pane config.TabPane, cwd string) []string {
	args := []string{
		"--copy-env",
		"--match", "env:KITTY_TAB_ID=" + tabID,
		"--env", "KITTY_TAB_ID=" + tabID,
		"--cwd=" + cwd,
	}
	if pane.Location != "" {
		args = append(args, "--location="+pane.Location)
	}
	if pane.Bias != 0 {
		args = append(args, "--bias", strconv.Itoa(pane.Bias))
	}
	switch {
	case pane.Command != "":
		args = append(args, "--env", "HYPRD_LAUNCH_COMMAND="+withResolvedPWD(pane.Command, cwd))
		args = append(args, persistentZshCommand()...)
	}
	return args
}

func withResolvedPWD(command, cwd string) string {
	return strings.ReplaceAll(command, "$PWD", cwd)
}

func persistentZshCommand() []string {
	return []string{"zsh", "-l"}
}

func (t *Tabs) resolvePaneCWD(pane config.TabPane, defaultCWD string) string {
	return t.resolveBaseCWD(pane.CWD, pane.CWDResolve, defaultCWD)
}

func (t *Tabs) resolveBaseCWD(cwd, cwdResolve, defaultCWD string) string {
	base := defaultCWD
	if cwd != "" {
		base = config.ExpandPath(cwd)
	}
	if cwdResolve == "recent-git" {
		if resolved := recentGitChild(base); resolved != "" {
			return resolved
		}
	}
	return base
}

func (t *Tabs) findTab(profile *config.TabProfile, name string) *config.TabDef {
	for i := range profile.Tabs {
		if profile.Tabs[i].Name == name {
			return &profile.Tabs[i]
		}
	}
	return nil
}

// closeLauncherTab removes kitty's initial launcher tab (empty KITTY_TAB_ID).
func (t *Tabs) closeLauncherTab(kitty *KittyClient, win KittyOSWindow) {
	for _, tab := range win.Tabs {
		for _, pane := range tab.Windows {
			if pane.Env["KITTY_TAB_ID"] == "" {
				kitty.CloseTabByNumericID(tab.ID)
				return
			}
		}
	}
}
