// Package browser provides Firefox session snapshot and restore primitives for hyprd.
//
// It translates Firefox profile and sessionstore data into reproducible launch state for Hyprland workflows.
//
// Responsibilities:
// - Discover Firefox profiles and load sessionstore payloads.
// - Create named snapshots from selected browser windows.
// - Restore snapshots by URL replay or exact session-file replacement.
package browser

// browser.go defines the Browser command surface and shared subcommand dispatch helpers.

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	browserUsage         = "usage: browser {launch|windows|snapshot|show|hypr|restore}"
	browserWindowsUsage  = "usage: browser windows [--all] [--profile <name|path>]"
	browserSnapshotUsage = "usage: browser snapshot <name> [active|largest|index] [--profile <name|path>]"
	browserShowUsage     = "usage: browser show <name>"
	browserHyprUsage     = "usage: browser hypr <name>"
	browserRestoreUsage  = "usage: browser restore <name> [--mode urls|exact] [--profile <name|path>] [--force] [--dry-run]"
)

// Browser exposes Firefox session commands backed by Hyprland and hyprd state.
type Browser struct {
	hypr  *hypr.Client
	state *state.State
}

// NewBrowser returns a Browser wired to the given Hyprland and state backends (either may be nil).
func NewBrowser(h *hypr.Client, s *state.State) *Browser {
	return &Browser{hypr: h, state: s}
}

// Execute dispatches a browser subcommand (e.g. "snapshot work active").
func (b *Browser) Execute(args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return "", fmt.Errorf(browserUsage)
	}

	switch parts[0] {
	case "launch":
		return b.executeLaunch(parts[1:])
	case "windows":
		return b.executeWindows(parts[1:])
	case "snapshot":
		return b.executeSnapshot(parts[1:])
	case "show":
		return b.executeShow(parts[1:])
	case "hypr":
		return b.executeHypr(parts[1:])
	case "restore":
		return b.executeRestore(parts[1:])
	default:
		return "", fmt.Errorf(browserUsage)
	}
}

// ResolveLaunchConfig populates cfg from the named snapshot, falling back to inline URLs if the snapshot is missing.
func (b *Browser) ResolveLaunchConfig(cfg config.BrowserConfig) (config.BrowserConfig, error) {
	if cfg.Snapshot == "" {
		return cfg, nil
	}

	snapshotCfg, err := b.SnapshotConfig(cfg.Snapshot)
	if err != nil {
		if len(cfg.AllURLs()) > 0 {
			return cfg, nil
		}
		return config.BrowserConfig{}, err
	}
	snapshotCfg.Snapshot = cfg.Snapshot
	return snapshotCfg, nil
}

// SnapshotConfig returns the BrowserConfig for the first window of the named snapshot.
func (b *Browser) SnapshotConfig(name string) (config.BrowserConfig, error) {
	_, store, err := b.loadSnapshotSession(name)
	if err != nil {
		return config.BrowserConfig{}, err
	}
	if len(store.Windows) == 0 {
		return config.BrowserConfig{}, fmt.Errorf("snapshot %q has no windows", name)
	}
	return summarizeFirefoxWindow(store.Windows[0]).Browser, nil
}

// UsesExactRestore reports whether cfg should restore by replacing Firefox session files.
func (b *Browser) UsesExactRestore(cfg config.BrowserConfig) bool {
	return browserMode(cfg) == "exact"
}

// RestoreConfiguredSnapshot performs an exact session-file restore using cfg.Snapshot.
func (b *Browser) RestoreConfiguredSnapshot(cfg config.BrowserConfig, dryRun bool) (string, error) {
	if !b.UsesExactRestore(cfg) {
		return "", fmt.Errorf("browser restore mode %q is not exact", browserMode(cfg))
	}
	if cfg.Snapshot == "" {
		return "", fmt.Errorf("browser exact restore requires snapshot")
	}

	dir, _, err := b.loadSnapshotSession(cfg.Snapshot)
	if err != nil {
		return "", err
	}
	profile, err := discoverFirefoxProfile(cfg.Profile)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(cfg.Snapshot, dir, profile, browserForce(cfg), dryRun)
}

// RestoreConfiguredSnapshotForSession exact-restores cfg.Snapshot into a hyprd-managed per-session profile.
func (b *Browser) RestoreConfiguredSnapshotForSession(sessionName string, cfg config.BrowserConfig, dryRun bool) (string, error) {
	if !b.UsesExactRestore(cfg) {
		return "", fmt.Errorf("browser restore mode %q is not exact", browserMode(cfg))
	}
	if cfg.Snapshot == "" {
		return "", fmt.Errorf("browser exact restore requires snapshot")
	}

	dir, _, err := b.loadSnapshotSession(cfg.Snapshot)
	if err != nil {
		return "", err
	}
	profile, err := ManagedProfileForSession(sessionName, cfg)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExactManaged(cfg.Snapshot, dir, profile, browserForce(cfg), dryRun)
}

func (b *Browser) executeWindows(args []string) (string, error) {
	profileArg, rest, err := parseProfileFlag(args, browserWindowsUsage)
	if err != nil {
		return "", err
	}
	all := false
	for _, arg := range rest {
		switch {
		case arg == "--all":
			all = true
		default:
			return "", fmt.Errorf(browserWindowsUsage)
		}
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	store, err := b.loadFirefoxSession(profile)
	if err != nil {
		return "", err
	}

	activeTitle := b.currentFirefoxTitle()
	var lines []string
	lines = append(lines, fmt.Sprintf("profile: %s (%s)", profile.Name, profile.Root))
	lines = append(lines, fmt.Sprintf("session: %s", store.Source))
	if activeTitle != "" {
		lines = append(lines, fmt.Sprintf("active_hypr_title: %s", activeTitle))
	}

	for i, window := range store.Windows {
		if !all && !windowIsInteresting(window) {
			continue
		}
		summary := summarizeFirefoxWindow(window)
		mark := " "
		if titlesMatch(activeTitle, summary.SelectedTitle) {
			mark = "*"
		}

		var tags []string
		if windowIsInteresting(window) {
			tags = append(tags, "interesting")
		}
		if summary.GroupCount > 0 {
			tags = append(tags, fmt.Sprintf("groups=%d", summary.GroupCount))
		}
		tagSuffix := ""
		if len(tags) > 0 {
			tagSuffix = " " + strings.Join(tags, " ")
		}

		lines = append(lines, fmt.Sprintf(
			"%s %02d tabs=%d selected=%d title=%q url=%q%s",
			mark, i+1, summary.TabCount, summary.SelectedTab, summary.SelectedTitle, summary.SelectedURL, tagSuffix,
		))
	}

	return strings.Join(lines, "\n"), nil
}

func (b *Browser) executeSnapshot(args []string) (string, error) {
	profileArg, rest, err := parseProfileFlag(args, browserSnapshotUsage)
	if err != nil {
		return "", err
	}
	var positional []string
	for _, arg := range rest {
		switch {
		case strings.HasPrefix(arg, "--"):
			return "", fmt.Errorf(browserSnapshotUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) < 1 || len(positional) > 2 {
		return "", fmt.Errorf(browserSnapshotUsage)
	}

	name := positional[0]
	selector := "active"
	if len(positional) == 2 {
		selector = positional[1]
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	store, err := b.loadFirefoxSession(profile)
	if err != nil {
		return "", err
	}

	windowIndex, err := b.resolveWindowIndex(store, selector)
	if err != nil {
		return "", err
	}
	return b.writeSnapshot(name, profile, windowIndex, store)
}

func (b *Browser) executeShow(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf(browserShowUsage)
	}
	dir, err := resolveSnapshotDir(args[0])
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "snapshot.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func (b *Browser) executeHypr(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf(browserHyprUsage)
	}
	cfg, err := b.SnapshotConfig(args[0])
	if err != nil {
		return "", err
	}
	doc := map[string]config.BrowserConfig{"browser": cfg}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func parseProfileFlag(args []string, usage string) (profile string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", nil, errors.New(usage)
			}
			i++
			profile = args[i]
		case strings.HasPrefix(arg, "--profile="):
			profile = strings.TrimPrefix(arg, "--profile=")
		default:
			rest = append(rest, arg)
		}
	}
	return profile, rest, nil
}

func browserMode(cfg config.BrowserConfig) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		if cfg.Snapshot != "" {
			return "exact"
		}
		return "urls"
	}
	return mode
}

func browserForce(cfg config.BrowserConfig) bool {
	return cfg.Force || (cfg.Snapshot != "" && browserMode(cfg) == "exact")
}

func shellQuoteCommand(parts []string) string {
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = strconv.Quote(part)
	}
	return strings.Join(quoted, " ")
}

// executeLaunch clears prior session state and launches Firefox cleanly via Hyprland.
// Three-body browser launches should not inherit artifacts from a previous exact restore.
func (b *Browser) executeLaunch(args []string) (string, error) {
	profileArg := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" && i+1 < len(args) {
			i++
			profileArg = args[i]
		}
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	if err := clearSessionStore(profile); err != nil {
		return "", err
	}

	cmd := append(b.browserCommandParts(), "--new-window", firefoxNewtab)
	if b.hypr != nil {
		if err := b.hypr.Dispatch(fmt.Sprintf("exec %s", strings.Join(cmd, " "))); err != nil {
			return "", err
		}
	} else {
		if err := exec.Command(cmd[0], cmd[1:]...).Start(); err != nil {
			return "", err
		}
	}
	return "launched browser (session cleared)", nil
}
