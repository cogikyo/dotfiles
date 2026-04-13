package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"dotfiles/daemons/config"
)

// Tabs manages kitty tab lifecycle — creating and refreshing tab sets from config profiles.
type Tabs struct {
	hypr  any // unused, kept for command pattern consistency
	state StateManager
}

// NewTabs creates a Tabs command handler.
func NewTabs(state StateManager) *Tabs {
	return &Tabs{state: state}
}

// Execute routes tabs subcommands: init <profile> <pid>, refresh <name|all> <pid>.
func (t *Tabs) Execute(args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "", fmt.Errorf("usage: tabs {init|refresh} <profile|name> <pid>")
	}

	sub := parts[0]
	switch sub {
	case "init":
		return t.init(parts[1:])
	case "refresh":
		return t.refresh(parts[1:])
	default:
		return "", fmt.Errorf("unknown subcommand: %s", sub)
	}
}

// init creates all tabs for a profile in the kitty instance identified by PID.
// Args: <profile> <pid>
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

	created := 0
	for _, tab := range profile.Tabs {
		cwd := t.resolveCWD(tab, defaultCWD)

		if !t.checkRequires(tab.Requires, cwd) {
			continue
		}

		tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
		launchArgs := t.buildLaunchArgs(tab, tabID, cwd)

		if err := kitty.Launch(launchArgs...); err != nil {
			return "", fmt.Errorf("launch tab %s: %w", tab.Name, err)
		}
		created++
	}

	// Close the launcher tab (the tab with no KITTY_TAB_ID)
	t.closeLauncherTab(kitty, windows[0])

	// Focus the default tab
	focusID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, profile.Focus)
	kitty.FocusTab(focusID)

	return fmt.Sprintf("tabs init: %s (%d tabs)", profileName, created), nil
}

// refresh closes and recreates tabs. Supports single tab, "all", or colon-separated aliases.
// Args: <name|all|alias> <pid>
func (t *Tabs) refresh(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: tabs refresh <name|all> <pid>")
	}

	nameOrAlias := args[0]
	pid, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid pid: %s", args[1])
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

	// Auto-detect profile from existing tabs
	profileName := t.detectProfile(windows[0])
	profile, err := t.getProfile(profileName)
	if err != nil {
		return "", err
	}

	if nameOrAlias == "all" {
		return t.refreshAll(kitty, profile, windowID)
	}

	// Resolve colon-separated alias (e.g., "nvim:master:fe-nvim")
	tabName := t.resolveAlias(nameOrAlias, profileName)

	// Verify tab exists in profile
	tabDef := t.findTab(profile, tabName)
	if tabDef == nil {
		return "", fmt.Errorf("tab %q not in profile %s", tabName, profileName)
	}

	return t.refreshSingle(kitty, profile, *tabDef, windowID)
}

// refreshAll closes all profile tabs and re-creates them.
func (t *Tabs) refreshAll(kitty *KittyClient, profile *config.TabProfile, windowID int) (string, error) {
	// Close all tabs belonging to this profile
	for _, tab := range profile.Tabs {
		tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
		kitty.CloseTab(tabID)
	}

	// Re-read state for default CWD after closing
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

		tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
		launchArgs := t.buildLaunchArgs(tab, tabID, cwd)
		if err := kitty.Launch(launchArgs...); err != nil {
			return "", fmt.Errorf("launch tab %s: %w", tab.Name, err)
		}
		created++
	}

	focusID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, profile.Focus)
	kitty.FocusTab(focusID)

	return fmt.Sprintf("tabs refresh: all (%d tabs)", created), nil
}

// refreshSingle closes and recreates a single tab, preserving its position.
func (t *Tabs) refreshSingle(kitty *KittyClient, profile *config.TabProfile, tab config.TabDef, windowID int) (string, error) {
	tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)

	// Record original tab index
	origIdx, _ := kitty.TabIndex(tabID)

	// Focus the tab before closing (so new tab opens adjacent)
	kitty.FocusTab(tabID)

	// Close the old tab
	kitty.CloseTab(tabID)

	// Resolve CWD
	windows, err := kitty.FullState()
	if err != nil {
		return "", err
	}
	defaultCWD := t.resolveDefaultCWD(windows[0])
	cwd := t.resolveCWD(tab, defaultCWD)

	if !t.checkRequires(tab.Requires, cwd) {
		return fmt.Sprintf("tabs refresh: %s (skipped, requires %s)", tab.Name, tab.Requires), nil
	}

	// Reopen
	launchArgs := t.buildLaunchArgs(tab, tabID, cwd)
	if err := kitty.Launch(launchArgs...); err != nil {
		return "", fmt.Errorf("launch tab %s: %w", tab.Name, err)
	}

	// Restore position
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

// getProfile looks up a tab profile by name from config.
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

// detectProfile determines the active profile by examining KITTY_TAB_ID env vars.
func (t *Tabs) detectProfile(win KittyOSWindow) string {
	cfg := t.state.GetConfig()
	windowID := win.ID
	prefix := fmt.Sprintf("%d-", windowID)

	for _, tab := range win.Tabs {
		for _, pane := range tab.Windows {
			id := pane.Env["KITTY_TAB_ID"]
			if id == "" || !strings.HasPrefix(id, prefix) {
				continue
			}
			suffix := id[len(prefix):]
			// Check all configured profiles for matching prefix
			for name, profile := range cfg.Tabs {
				if profile.Prefix != "" && strings.HasPrefix(suffix, profile.Prefix) {
					return name
				}
			}
		}
	}

	return "editor" // default
}

// resolveAlias handles colon-separated tab name aliases (e.g., "nvim:master:fe-nvim").
// The profile order is: editor:agents:leadpier.
func (t *Tabs) resolveAlias(alias, profileName string) string {
	if !strings.Contains(alias, ":") {
		return alias
	}

	parts := strings.Split(alias, ":")
	profileOrder := map[string]int{
		"editor":   0,
		"agents":   1,
		"leadpier": 2,
	}

	idx, ok := profileOrder[profileName]
	if !ok || idx >= len(parts) {
		return parts[0]
	}

	name := parts[idx]
	if name == "" {
		return parts[0]
	}
	return name
}

// resolveDefaultCWD extracts the CWD from the focused tab's focused pane.
func (t *Tabs) resolveDefaultCWD(win KittyOSWindow) string {
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

	// Fall back to PWD or HOME
	if pwd := os.Getenv("PWD"); pwd != "" {
		return pwd
	}
	home, _ := os.UserHomeDir()
	return home
}

// resolveCWD determines the working directory for a tab.
func (t *Tabs) resolveCWD(tab config.TabDef, defaultCWD string) string {
	base := defaultCWD
	if tab.CWD != "" {
		base = config.ExpandPath(tab.CWD)
	}

	if tab.CWDResolve == "recent-git" {
		if resolved := recentGitChild(base); resolved != "" {
			return resolved
		}
	}

	return base
}

// recentGitChild finds the child directory of parent with the most recent git commit.
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

		// Check if it's a git repo (or inside one)
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

// checkRequires verifies that a tab's requirement is met in the given CWD.
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

// buildLaunchArgs constructs kitty @ launch arguments for a tab.
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
		args = append(args, "zsh", "-c",
			`cd "$(xplr --print-pwd-as-result)" 2>/dev/null; exec zsh`)
	case tab.Command != "":
		args = append(args, "--hold", tab.Command)
	}

	return args
}

// findTab looks up a tab definition by name within a profile.
func (t *Tabs) findTab(profile *config.TabProfile, name string) *config.TabDef {
	for i := range profile.Tabs {
		if profile.Tabs[i].Name == name {
			return &profile.Tabs[i]
		}
	}
	return nil
}

// closeLauncherTab closes the initial launcher tab (the one with no KITTY_TAB_ID).
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
