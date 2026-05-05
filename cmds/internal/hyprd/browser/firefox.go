package browser

// firefox.go handles Firefox process checks, profile detection, and workspace-local URL-open targeting.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

const (
	firefoxBinary = "firefox-developer-edition"
	firefoxNewtab = "http://localhost:42069"
)

var (
	// firefoxTitleSuffixes are stripped to normalize Hypr window titles for session-store comparison.
	firefoxTitleSuffixes = []string{
		" — Firefox Developer Edition",
		" — Mozilla Firefox",
	}
	// trivialBrowserURLs are ignored by snapshot heuristics when picking "interesting" windows.
	trivialBrowserURLs = map[string]struct{}{
		"":                        {},
		"about:blank":             {},
		"about:home":              {},
		"about:newtab":            {},
		"http://localhost:42069/": {},
	}
)

type firefoxOpenTarget struct {
	Window             hypr.Window
	Profile            firefoxProfile
	WorkspaceID        int
	NeedsThreeBodySwap bool
}

func firefoxRunningPIDs() ([]int, error) {
	cmd := exec.Command("pgrep", "-f", "/usr/lib/firefox-developer-edition/firefox|firefox-developer-edition")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var pids []int
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// FirefoxRunning reports whether Firefox Developer Edition appears to be running.
func FirefoxRunning() bool {
	pids, err := firefoxRunningPIDs()
	return err == nil && len(pids) > 0
}

func firefoxProfilePIDs(profile firefoxProfile) ([]int, error) {
	pids, err := firefoxRunningPIDs()
	if err != nil {
		return nil, err
	}
	var matched []int
	for _, pid := range pids {
		args, err := processArgs(pid)
		if err != nil {
			continue
		}
		if argsUseProfile(args, profile.Root) {
			matched = append(matched, pid)
		}
	}
	return matched, nil
}

func processArgs(pid int) ([]string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimRight(string(data), "\x00")
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\x00"), nil
}

func argsUseProfile(args []string, profileRoot string) bool {
	profileArg, ok := profileArgFromArgs(args)
	return ok && profileArgMatches(profileArg, profileRoot)
}

func profileArgFromArgs(args []string) (string, bool) {
	for i, arg := range args {
		switch {
		case arg == "--profile" || arg == "-profile" || arg == "-P":
			if i+1 < len(args) {
				return args[i+1], true
			}
		case strings.HasPrefix(arg, "--profile="):
			return strings.TrimPrefix(arg, "--profile="), true
		}
	}
	return "", false
}

func profileArgMatches(raw, profileRoot string) bool {
	return filepath.Clean(config.ExpandPath(raw)) == profileRoot
}

func processTreeUsesProfile(pid int, profileRoot string) bool {
	for range 32 {
		args, err := processArgs(pid)
		if err == nil && argsUseProfile(args, profileRoot) {
			return true
		}
		parent, err := processParent(pid)
		if err != nil || parent <= 1 || parent == pid {
			return false
		}
		pid = parent
	}
	return false
}

func processParent(pid int) (int, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		value, ok := strings.CutPrefix(line, "PPid:")
		if !ok {
			continue
		}
		return strconv.Atoi(strings.TrimSpace(value))
	}
	return 0, fmt.Errorf("no parent pid for %d", pid)
}

// stopFirefox ensures no Firefox is running; with force it SIGTERMs and polls up to 15s.
func stopFirefox(force bool) error {
	pids, err := firefoxRunningPIDs()
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("firefox is running; rerun with --force for exact restore")
	}

	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		pids, err = firefoxRunningPIDs()
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("firefox did not exit cleanly after SIGTERM")
}

func stopFirefoxProfile(profile firefoxProfile, force bool) error {
	pids, err := firefoxProfilePIDs(profile)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("firefox profile %s is running; rerun with --force for exact restore", profile.Root)
	}

	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		pids, err = firefoxProfilePIDs(profile)
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("firefox profile %s did not exit cleanly after SIGTERM", profile.Root)
}

func (b *Browser) launchFirefoxProfile(profile firefoxProfile) error {
	cmd := append(slices.Clone(b.browserCommandParts()), "--new-instance", "--profile", profile.Root)
	// Dispatch through Hyprland so Firefox inherits Hyprland's env (Wayland, Qt, cursor vars).
	if b.hypr != nil {
		return b.hypr.Dispatch(fmt.Sprintf("exec %s", shellQuoteCommand(cmd)))
	}
	return exec.Command(cmd[0], cmd[1:]...).Start()
}

// clearSessionStore removes Firefox sessionstore files so normal browser launches don't inherit exact restores.
func clearSessionStore(profile firefoxProfile) error {
	if err := removeIfExists(filepath.Join(profile.Root, "sessionstore.jsonlz4")); err != nil {
		return err
	}

	backupsDir := filepath.Join(profile.Root, "sessionstore-backups")
	for _, name := range []string{"recovery.jsonlz4", "recovery.baklz4", "previous.jsonlz4"} {
		if err := removeIfExists(filepath.Join(backupsDir, name)); err != nil {
			return err
		}
	}
	upgrades, err := filepath.Glob(filepath.Join(backupsDir, "upgrade.jsonlz4-*"))
	if err != nil {
		return err
	}
	for _, path := range upgrades {
		if err := removeIfExists(path); err != nil {
			return err
		}
	}
	return nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove %s: %w", path, err)
}

func (b *Browser) browserCommandParts() []string {
	return []string{firefoxBinary}
}

func (b *Browser) currentFirefoxTitle() string {
	active := b.activeFirefoxWindow()
	if active == nil {
		return ""
	}
	return trimFirefoxTitle(active.Title)
}

func (b *Browser) activeFirefoxWindow() *hypr.Window {
	if b.hypr == nil {
		return nil
	}
	active, err := b.hypr.ActiveWindow()
	if err != nil || active == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(active.Class), "firefox") {
		return nil
	}
	return active
}

func (b *Browser) activeFirefoxProfile() (firefoxProfile, bool) {
	active := b.activeFirefoxWindow()
	if active == nil {
		return firefoxProfile{}, false
	}
	return firefoxProfileFromWindow(*active)
}

func (b *Browser) focusedWorkspaceFirefoxProfile() (firefoxProfile, bool) {
	target, ok := b.focusedWorkspaceFirefoxOpenTarget()
	if !ok {
		return firefoxProfile{}, false
	}
	return target.Profile, target.Profile.Root != ""
}

func (b *Browser) focusedWorkspaceFirefoxOpenTarget() (firefoxOpenTarget, bool) {
	if b.hypr == nil {
		return firefoxOpenTarget{}, false
	}
	workspaceID, err := b.currentWorkspaceID()
	if err != nil {
		return firefoxOpenTarget{}, false
	}
	clients, err := b.hypr.Clients()
	if err != nil {
		return firefoxOpenTarget{}, false
	}
	return firefoxOpenTargetForWorkspace(clients, workspaceID, b.threeBodyState(workspaceID))
}

// currentWorkspaceID returns the workspace that should own an external browser open.
//
// The active window wins because launchers can run while focus remains on the invoking app.
// The focused monitor is only a fallback for empty workspaces or missing active-window data.
func (b *Browser) currentWorkspaceID() (int, error) {
	if b.hypr == nil {
		return 0, fmt.Errorf("no hyprland client")
	}
	active, err := b.hypr.ActiveWindow()
	if err == nil && active != nil && active.Workspace.ID > 0 {
		return active.Workspace.ID, nil
	}
	monitor, err := b.hypr.FocusedMonitor()
	if err != nil {
		return 0, err
	}
	if monitor == nil {
		return 0, fmt.Errorf("no focused monitor")
	}
	return monitor.ActiveWS.ID, nil
}

func (b *Browser) threeBodyState(workspaceID int) *state.ThreeBodyState {
	if b.state == nil {
		return nil
	}
	return b.state.GetThreeBody(workspaceID)
}

// firefoxOpenTargetForWorkspace chooses the Firefox instance owned by workspaceID.
//
// Three-body state wins over stray visible Firefox windows.
// Shadow, active, and master addresses define workspace ownership.
// It never falls back to a Firefox window from another workspace.
func firefoxOpenTargetForWorkspace(clients []hypr.Window, workspaceID int, threeBody *state.ThreeBodyState) (firefoxOpenTarget, bool) {
	if target, ok := threeBodyFirefoxOpenTarget(clients, workspaceID, threeBody); ok {
		return target, true
	}
	if best, ok := visibleWorkspaceFirefoxWindow(clients, workspaceID); ok {
		return firefoxOpenTargetFromWindow(best, workspaceID, false)
	}
	return firefoxOpenTarget{}, false
}

func threeBodyFirefoxOpenTarget(clients []hypr.Window, workspaceID int, threeBody *state.ThreeBodyState) (firefoxOpenTarget, bool) {
	if threeBody == nil || threeBody.Shadow == "" {
		return firefoxOpenTarget{}, false
	}
	for _, address := range []string{threeBody.Active, threeBody.Master} {
		if address == "" {
			continue
		}
		for _, client := range clients {
			if client.Address == address && client.Workspace.ID == workspaceID && isFirefoxWindow(client) {
				return firefoxOpenTargetFromWindow(client, workspaceID, false)
			}
		}
	}
	for _, client := range clients {
		if client.Address != threeBody.Shadow || client.Workspace.Name != windows.ShadowWorkspace || !isFirefoxWindow(client) {
			continue
		}
		return firefoxOpenTargetFromWindow(client, workspaceID, true)
	}
	return firefoxOpenTarget{}, false
}

func visibleWorkspaceFirefoxWindow(clients []hypr.Window, workspaceID int) (hypr.Window, bool) {
	best := hypr.Window{FocusHistoryID: int(^uint(0) >> 1)}
	for _, client := range clients {
		if client.Workspace.ID != workspaceID || !isFirefoxWindow(client) {
			continue
		}
		if client.FocusHistoryID < best.FocusHistoryID {
			best = client
		}
	}
	return best, best.Address != ""
}

func firefoxOpenTargetFromWindow(window hypr.Window, workspaceID int, needsThreeBodySwap bool) (firefoxOpenTarget, bool) {
	profile, _ := firefoxProfileFromWindow(window)
	return firefoxOpenTarget{Window: window, Profile: profile, WorkspaceID: workspaceID, NeedsThreeBodySwap: needsThreeBodySwap}, true
}

func isFirefoxWindow(window hypr.Window) bool {
	return strings.Contains(strings.ToLower(window.Class), "firefox") || strings.Contains(strings.ToLower(window.InitialClass), "firefox")
}

func (b *Browser) focusFirefoxOpenTarget(target firefoxOpenTarget) error {
	if b.hypr == nil {
		return nil
	}
	if target.NeedsThreeBodySwap {
		return b.swapThreeBodyShadowIntoView(target.WorkspaceID, target.Window.Address)
	}
	return b.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", target.Window.Address))
}

// swapThreeBodyShadowIntoView makes the workspace-owned shadow Firefox active before CLI remoting.
//
// Firefox sends `--new-tab` to the active instance for that profile.
// Remoting before the swap can open on the wrong window.
func (b *Browser) swapThreeBodyShadowIntoView(workspaceID int, shadowAddress string) error {
	tiled, err := windows.GetTiledWindows(b.hypr, workspaceID)
	if err != nil {
		return err
	}
	slaves := windows.GetSlaves(tiled)
	if len(slaves) == 0 {
		return fmt.Errorf("no visible three-body slave on workspace %d", workspaceID)
	}
	activeAddress := slaves[0].Address
	batch := fmt.Sprintf("dispatch movetoworkspacesilent %s,address:%s; dispatch movetoworkspacesilent %d,address:%s; dispatch focuswindow address:%s", windows.ShadowWorkspace, activeAddress, workspaceID, shadowAddress, shadowAddress)
	if _, err := b.hypr.Request("[[BATCH]]" + batch); err != nil {
		return err
	}
	if b.state != nil && len(tiled) > 0 {
		b.state.SetThreeBody(workspaceID, &state.ThreeBodyState{Master: tiled[0].Address, Active: shadowAddress, Shadow: activeAddress})
	}
	return nil
}

func firefoxProfileFromWindow(window hypr.Window) (firefoxProfile, bool) {
	return firefoxProfileFromPID(window.Pid)
}

func firefoxProfileFromPID(pid int) (firefoxProfile, bool) {
	for pid > 1 {
		args, err := processArgs(pid)
		if err == nil {
			if raw, ok := profileArgFromArgs(args); ok {
				profile, err := discoverFirefoxProfile(raw)
				return profile, err == nil
			}
		}
		parent, err := processParent(pid)
		if err != nil || parent <= 1 || parent == pid {
			return firefoxProfile{}, false
		}
		pid = parent
	}
	return firefoxProfile{}, false
}

func trimFirefoxTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, suffix := range firefoxTitleSuffixes {
		title = strings.TrimSuffix(title, suffix)
	}
	return strings.TrimSpace(title)
}

// titlesMatch returns true on exact or prefix match (Firefox truncates long titles in the session store).
func titlesMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
