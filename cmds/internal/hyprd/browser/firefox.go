package browser

// firefox.go handles Firefox processes and workspace-local URL-open targeting.

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
		args, err := firefoxProcessArgs(pid)
		if err != nil || !isFirefoxMainProcess(args) {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func firefoxProcessArgs(pid int) ([]string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(string(data), "\x00"), "\x00"), nil
}

func isFirefoxMainProcess(args []string) bool {
	if len(args) == 0 || filepath.Base(args[0]) != "firefox" {
		return false
	}
	return !slices.Contains(args, "-contentproc")
}

// FirefoxRunning reports whether Firefox Developer Edition appears to be running.
func FirefoxRunning() bool {
	pids, err := firefoxRunningPIDs()
	return err == nil && len(pids) > 0
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

func (b *Browser) launchFirefoxProfile(profile firefoxProfile) error {
	cmd := append(slices.Clone(b.browserCommandParts()), "--new-instance", "--profile", profile.Root)
	// Dispatch through Hyprland so Firefox inherits Hyprland's env (Wayland, Qt, cursor vars).
	if b.hypr != nil {
		return b.hypr.Exec(shellQuoteCommand(cmd))
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
	return firefoxOpenTarget{Window: window, WorkspaceID: workspaceID, NeedsThreeBodySwap: needsThreeBodySwap}, true
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
	return b.hypr.FocusWindow(target.Window.Address)
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
	if err := b.hypr.MoveWindowToWorkspace(activeAddress, windows.ShadowWorkspace, false); err != nil {
		return fmt.Errorf("move active window to shadow workspace: %w", err)
	}
	if err := b.hypr.MoveWindowToWorkspace(shadowAddress, strconv.Itoa(workspaceID), false); err != nil {
		return fmt.Errorf("move shadow window to workspace %d: %w", workspaceID, err)
	}
	if err := b.hypr.FocusWindow(shadowAddress); err != nil {
		return fmt.Errorf("focus shadow window: %w", err)
	}
	if b.state != nil && len(tiled) > 0 {
		b.state.SetThreeBody(workspaceID, &state.ThreeBodyState{Master: tiled[0].Address, Active: shadowAddress, Shadow: activeAddress})
	}
	return nil
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
