package kitty

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

// Manager owns tab initialization, refresh, and host-switch commands.
type Manager struct {
	hypr         *hypr.Client
	state        *state.State
	hostMu       sync.Mutex
	hostSwitches map[hostOrigin]bool
}

func NewManager(h *hypr.Client, state *state.State) *Manager {
	return &Manager{hypr: h, state: state, hostSwitches: make(map[hostOrigin]bool)}
}

// Execute dispatches tab initialization, refresh, and host-switch commands.
func (t *Manager) Execute(args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) < 1 {
		return "", fmt.Errorf("usage: tabs init <profile> <pid> | tabs refresh <position|name|current|all> [pid] | tabs host <alias> [--kitty-pid <pid> --os-window <id>]")
	}

	switch parts[0] {
	case "init":
		return t.init(parts[1:])
	case "refresh":
		return t.refresh(parts[1:])
	case "host":
		return t.host(parts[1:])
	default:
		return "", fmt.Errorf("unknown subcommand: %s", parts[0])
	}
}

func (t *Manager) init(args []string) (string, error) {
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

	kitty := NewClient(pid)
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
		if err := t.launchLocalTab(kitty, profile, tab, windowID, cwd); err != nil {
			return "", err
		}
		created++
	}

	t.closeLauncherTab(kitty, windows[0])
	focusID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, profile.Focus)
	kitty.FocusTab(focusID)

	return fmt.Sprintf("tabs init: %s (%d tabs)", profileName, created), nil
}

func (t *Manager) refresh(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: tabs refresh <position|name|current|all> [pid]")
	}

	nameOrAlias := args[0]
	pid, err := t.refreshPID(args)
	if err != nil {
		return "", err
	}

	kitty := NewClient(pid)
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
	controller, err := controllerHost()
	if err != nil {
		return "", err
	}
	host, err := sourceHost(windows[0], controller)
	if err != nil {
		return "", err
	}

	if nameOrAlias == "all" {
		logical, err := t.refreshLogicalCWD(windows[0], profile, "", config.TabDef{}, controller, host)
		if err != nil {
			return "", err
		}
		return t.refreshAll(kitty, profile, windowID, host, logical)
	}

	tabName := resolveTabAlias(t.state.GetConfig(), nameOrAlias, profileName)
	if nameOrAlias == "current" {
		tabName = activeProfileTabName(t.state.GetConfig(), profileName, windows[0])
	}

	var tabDef *config.TabDef
	position, positionErr := strconv.Atoi(nameOrAlias)
	if positionErr == nil {
		if position < 1 || position > len(profile.Tabs) {
			return "", fmt.Errorf("tab position %d out of range for profile %s (1-%d)", position, profileName, len(profile.Tabs))
		}
		tabDef = &profile.Tabs[position-1]
		tabName = tabDef.Name
	} else {
		if len(nameOrAlias) > 0 && ((nameOrAlias[0] >= '0' && nameOrAlias[0] <= '9') || nameOrAlias[0] == '+' || nameOrAlias[0] == '-') {
			return "", fmt.Errorf("invalid tab position %q for profile %s (expected 1-%d)", nameOrAlias, profileName, len(profile.Tabs))
		}
		tabDef = t.findTab(profile, tabName)
		if tabDef == nil {
			if resolved := pickSemanticTab(profile, baseTabName(nameOrAlias), "", "", ""); resolved != "" {
				tabName = resolved
				tabDef = t.findTab(profile, tabName)
			}
		}
	}
	if tabDef == nil {
		return "", fmt.Errorf("tab %q not in profile %s", tabName, profileName)
	}
	tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tabDef.Name)
	logical, err := t.refreshLogicalCWD(windows[0], profile, tabID, *tabDef, controller, host)
	if err != nil {
		return "", err
	}
	return t.refreshSingle(kitty, profile, *tabDef, windowID, host, logical)
}

func (t *Manager) refreshPID(args []string) (int, error) {
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

func (t *Manager) activeKittyPID() (int, error) {
	win, err := t.hypr.ActiveWindow()
	if err != nil {
		return 0, err
	}
	if win == nil || win.Pid == 0 || win.Class != "kitty" {
		return 0, fmt.Errorf("usage: tabs refresh <position|name|current|all> [pid]")
	}
	return win.Pid, nil
}

func (t *Manager) refreshAll(kitty *Client, profile *config.TabProfile, windowID int, host, defaultLogical string) (string, error) {
	type refreshTab struct {
		definition config.TabDef
		logical    string
	}
	var launch []refreshTab
	for _, tab := range profile.Tabs {
		logical, err := configuredLogicalCWD(tab.CWD, defaultLogical)
		if err != nil {
			return "", fmt.Errorf("tab %s: %w", tab.Name, err)
		}
		cwd, err := hostPath(host, logical)
		if err != nil {
			return "", err
		}
		satisfied, err := hostRequiresSatisfied(host, tab.Requires, cwd)
		if err != nil {
			return "", err
		}
		if satisfied {
			launch = append(launch, refreshTab{definition: tab, logical: logical})
		}
	}

	for _, tab := range profile.Tabs {
		tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
		kitty.closeTab(tabID)
	}

	for _, tab := range launch {
		if err := t.launchManagedTab(kitty, profile, tab.definition, windowID, host, tab.logical); err != nil {
			return "", err
		}
	}

	focusID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, profile.Focus)
	kitty.FocusTab(focusID)
	return fmt.Sprintf("tabs refresh: all (%d tabs)", len(launch)), nil
}

func (t *Manager) refreshSingle(kitty *Client, profile *config.TabProfile, tab config.TabDef, windowID int, host, defaultLogical string) (string, error) {
	tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
	origIdx, _ := kitty.tabIndex(tabID)
	logical, err := configuredLogicalCWD(tab.CWD, defaultLogical)
	if err != nil {
		return "", err
	}
	cwd, err := hostPath(host, logical)
	if err != nil {
		return "", err
	}

	satisfied, err := hostRequiresSatisfied(host, tab.Requires, cwd)
	if err != nil {
		return "", err
	}
	if !satisfied {
		return fmt.Sprintf("tabs refresh: %s (skipped, requires %s)", tab.Name, tab.Requires), nil
	}
	kitty.FocusTab(tabID)
	kitty.closeTab(tabID)

	if err := t.launchManagedTab(kitty, profile, tab, windowID, host, logical); err != nil {
		return "", err
	}

	if origIdx >= 0 {
		newIdx, _ := kitty.tabIndex(tabID)
		if newIdx > origIdx {
			for range newIdx - origIdx {
				kitty.moveTabBackward()
			}
		}
	}

	return fmt.Sprintf("tabs refresh: %s", tab.Name), nil
}

func (t *Manager) refreshLogicalCWD(win OSWindow, profile *config.TabProfile, tabID string, definition config.TabDef, controller, source string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("controller HOME unavailable: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("controller HOME unavailable")
	}
	for _, tab := range win.Tabs {
		if tabID != "" && !tabHasID(tab, tabID) {
			continue
		}
		if tabID == "" && !tabSelected(tab) {
			continue
		}
		if definition.Name == "" && len(tab.Windows) > 0 {
			name := profileTabNameFromID(win.ID, profile.Prefix, tab.Windows[0].Env["KITTY_TAB_ID"])
			if found := findProfileTab(profile, name); found != nil {
				definition = *found
			}
		}
		if definition.CWD != "" {
			return configuredLogicalCWD(definition.CWD, ".")
		}
		panes, err := orderedProfilePanes(tab.Windows)
		indexed := err == nil
		if err != nil {
			panes = tab.Windows
		}
		paneIndex := 0
		for index, pane := range panes {
			if paneSelected(pane) {
				paneIndex = index
				break
			}
		}
		configured := ""
		if indexed {
			configured = configuredPaneCWD(definition, paneIndex)
		}
		return sourceLogicalCWD(panes[paneIndex], configured, home, source == controller)
	}
	return "", fmt.Errorf("managed tab CWD is unavailable; refresh the window on its source host")
}

func configuredLogicalCWD(configured, fallback string) (string, error) {
	if configured == "" {
		return normalizeLogicalCWD(fallback)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("controller HOME unavailable: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("controller HOME unavailable")
	}
	return logicalCWDFromPath(config.ExpandPath(configured), home)
}

func hostPath(host, logical string) (string, error) {
	logical, err := normalizeLogicalCWD(logical)
	if err != nil {
		return "", err
	}
	home, err := hostHome(host)
	if err != nil {
		return "", fmt.Errorf("host %s HOME: %w", host, err)
	}
	return filepath.Join(home, filepath.FromSlash(logical)), nil
}

func (t *Manager) getProfile(name string) (*config.TabProfile, error) {
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

func (t *Manager) resolveDefaultCWD(win OSWindow) string {
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

func (t *Manager) resolveCWD(tab config.TabDef, defaultCWD string) string {
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

func (t *Manager) checkRequires(requires, cwd string) bool {
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

func (t *Manager) launchLocalTab(kitty *Client, profile *config.TabProfile, tab config.TabDef, windowID int, cwd string) error {
	controller, err := controllerHost()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if home == "" {
		return fmt.Errorf("controller HOME unavailable")
	}
	logical, err := logicalCWDFromPath(cwd, home)
	if err != nil {
		return err
	}
	return t.launchManagedTab(kitty, profile, tab, windowID, controller, logical)
}

func (t *Manager) launchManagedTab(kitty *Client, profile *config.TabProfile, tab config.TabDef, windowID int, host, logical string) error {
	controller, err := controllerHost()
	if err != nil {
		return err
	}
	tabID := fmt.Sprintf("%d-%s%s", windowID, profile.Prefix, tab.Name)
	cwd, err := hostPath(host, logical)
	if err != nil {
		return err
	}
	launchArgs := t.buildLaunchArgs(tab, tabID, controller, host, logical, cwd)
	paneIDs := make([]int, 1, len(tab.Panes)+1)
	paneIDs[0], err = kitty.launchID(launchArgs...)
	if err != nil {
		return fmt.Errorf("launch tab %s: %w", tab.Name, err)
	}

	if tab.Layout != "" {
		if err := kitty.gotoLayout(tabID, tab.Layout); err != nil {
			return fmt.Errorf("set layout for tab %s: %w", tab.Name, err)
		}
	}

	for paneOffset, pane := range tab.Panes {
		paneIndex := paneOffset + 1
		paneLogical, err := configuredLogicalCWD(pane.CWD, logical)
		if err != nil {
			return fmt.Errorf("tab %s pane %d: %w", tab.Name, paneIndex, err)
		}
		paneCWD, err := hostPath(host, paneLogical)
		if err != nil {
			return err
		}
		launchArgs = t.buildPaneLaunchArgs(tabID, pane, controller, host, paneLogical, paneCWD, paneIndex)
		paneID, err := kitty.launchID(launchArgs...)
		if err != nil {
			return fmt.Errorf("launch pane for tab %s: %w", tab.Name, err)
		}
		paneIDs = append(paneIDs, paneID)
	}

	if tab.Layout != "" && len(tab.Panes) > 0 {
		if err := kitty.gotoLayout(tabID, tab.Layout); err != nil {
			return fmt.Errorf("reapply layout for tab %s: %w", tab.Name, err)
		}
	}
	if tab.FocusPane < 0 || tab.FocusPane >= len(paneIDs) {
		return fmt.Errorf("tab %s focus_pane %d out of range", tab.Name, tab.FocusPane)
	}
	if err := kitty.FocusWindow(paneIDs[tab.FocusPane]); err != nil {
		return fmt.Errorf("focus pane for tab %s: %w", tab.Name, err)
	}

	return nil
}

func (t *Manager) buildLaunchArgs(tab config.TabDef, tabID, controller, host, logical, cwd string) []string {
	args := []string{
		"--type=tab",
		"--copy-env",
		"--env", "KITTY_TAB_ID=" + tabID,
		"--env", "HYPRD_LAUNCH_COMMAND=",
		"--env", "HYPRD_HOST_TXN=",
		"--env", "HYPRD_TAB_HOST=",
		"--tab-title=" + tab.Title,
	}
	args = append(args, paneMetadataArgs(host, logical, 0, "")...)
	args = append(args, hostLaunchCommand(controller, host, tab.Command, cwd)...)
	return args
}

func (t *Manager) buildPaneLaunchArgs(tabID string, pane config.TabPane, controller, host, logical, cwd string, paneIndex int) []string {
	args := []string{
		"--copy-env",
		"--match", "env:KITTY_TAB_ID=" + tabID,
		"--env", "KITTY_TAB_ID=" + tabID,
		"--env", "HYPRD_LAUNCH_COMMAND=",
		"--env", "HYPRD_HOST_TXN=",
		"--env", "HYPRD_TAB_HOST=",
	}
	args = append(args, paneMetadataArgs(host, logical, paneIndex, "")...)
	if pane.Location != "" {
		args = append(args, "--location="+pane.Location)
	}
	if pane.Bias != 0 {
		args = append(args, "--bias", strconv.Itoa(pane.Bias))
	}
	args = append(args, hostLaunchCommand(controller, host, pane.Command, cwd)...)
	return args
}

func withResolvedPWD(command, cwd string) string {
	return strings.ReplaceAll(command, "$PWD", cwd)
}

func persistentZshCommand() []string {
	return []string{"zsh", "-l"}
}

func (t *Manager) resolveBaseCWD(cwd, cwdResolve, defaultCWD string) string {
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

func (t *Manager) findTab(profile *config.TabProfile, name string) *config.TabDef {
	for i := range profile.Tabs {
		if profile.Tabs[i].Name == name {
			return &profile.Tabs[i]
		}
	}
	return nil
}

// closeLauncherTab removes kitty's initial launcher tab (empty KITTY_TAB_ID).
func (t *Manager) closeLauncherTab(kitty *Client, win OSWindow) {
	for _, tab := range win.Tabs {
		for _, pane := range tab.Windows {
			if pane.Env["KITTY_TAB_ID"] == "" {
				kitty.closeTabByNumericID(tab.ID)
				return
			}
		}
	}
}
