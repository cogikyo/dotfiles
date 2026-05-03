package browser

// firefox.go handles Firefox process lifecycle checks and title matching against session-store window metadata.

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

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
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
	if b.hypr == nil {
		return firefoxProfile{}, false
	}
	monitor, err := b.hypr.FocusedMonitor()
	if err != nil || monitor == nil {
		return firefoxProfile{}, false
	}
	clients, err := b.hypr.Clients()
	if err != nil {
		return firefoxProfile{}, false
	}
	best := hypr.Window{FocusHistoryID: int(^uint(0) >> 1)}
	for _, client := range clients {
		if client.Workspace.ID != monitor.ActiveWS.ID {
			continue
		}
		if !strings.Contains(strings.ToLower(client.Class), "firefox") {
			continue
		}
		if client.FocusHistoryID < best.FocusHistoryID {
			best = client
		}
	}
	if best.Address == "" {
		return firefoxProfile{}, false
	}
	return firefoxProfileFromWindow(best)
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
