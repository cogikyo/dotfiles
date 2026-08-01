// Package browser provides Firefox workspace routing plus session snapshot and restore primitives for hyprd.
//
// External URL opens enter through `hyprd browser open`, usually from repo-managed desktop entries.
// Routing is workspace-local.
// The active window's workspace is used first; the focused monitor's workspace is the fallback.
// Three-body state owns browser selection and can swap a shadow Firefox into view before CLI remoting.
// No browser on another workspace is used as a fallback.
package browser

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	browserUsage         = "usage: browser {launch|open|windows|snapshot|show|hypr|restore}"
	browserLaunchUsage   = "usage: browser launch"
	browserOpenUsage     = "usage: browser open <url>"
	browserWindowsUsage  = "usage: browser windows [--all]"
	browserSnapshotUsage = "usage: browser snapshot <name> [active|largest|index]"
	browserShowUsage     = "usage: browser show <name>"
	browserHyprUsage     = "usage: browser hypr <name>"
	browserRestoreUsage  = "usage: browser restore <name> [--force] [--dry-run]"
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
	case "open":
		return b.executeOpen(parts[1:])
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

// executeOpen routes a URL to the Firefox instance owned by the current workspace.
//
// The active window chooses the workspace when possible; the focused monitor is only a fallback.
// Firefox CLI remoting runs after the target is focused or swapped into view.
// If no workspace-owned Firefox exists, this starts a new window instead of falling across workspaces.
func (b *Browser) executeOpen(args []string) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf(browserOpenUsage)
	}
	url := args[0]

	if target, ok := b.focusedWorkspaceFirefoxOpenTarget(); ok {
		if err := b.openURLInFirefoxTarget(target, url); err != nil {
			return "", err
		}
		return "opened browser URL", nil
	}

	cmd := append(b.browserCommandParts(), "--new-window", url)
	if err := exec.Command(cmd[0], cmd[1:]...).Start(); err != nil {
		return "", err
	}

	return "opened browser URL", nil
}

func (b *Browser) openURLInFirefoxTarget(target firefoxOpenTarget, url string) error {
	if err := b.focusFirefoxOpenTarget(target); err != nil {
		return err
	}
	time.Sleep(250 * time.Millisecond)

	cmd := b.browserCommandParts()
	cmd = append(cmd, "--new-tab", url)
	if err := exec.Command(cmd[0], cmd[1:]...).Start(); err != nil {
		return err
	}
	return nil
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

// UsesExactRestore reports whether cfg should restore through Firefox session files.
func (b *Browser) UsesExactRestore(cfg config.BrowserConfig) bool {
	return browserMode(cfg) == "exact"
}

// RestoreConfiguredSnapshot merges cfg.Snapshot as an exact window in the main Firefox profile.
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
	profile, err := discoverFirefoxProfile("")
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(cfg.Snapshot, dir, profile, browserForce(cfg), dryRun)
}

// RestoreConfiguredSnapshots restores multiple exact snapshot layouts into one shared Firefox profile.
func (b *Browser) RestoreConfiguredSnapshots(configs []config.BrowserConfig, dryRun bool) (string, error) {
	if len(configs) == 0 {
		return "", fmt.Errorf("no browser snapshots to restore")
	}

	dirs := make([]string, 0, len(configs))
	for _, cfg := range configs {
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
		dirs = append(dirs, dir)
	}

	profile, err := discoverFirefoxProfile("")
	if err != nil {
		return "", err
	}
	payload, err := buildCombinedSessionPayload(dirs)
	if err != nil {
		return "", err
	}
	return b.injectAndLaunch(payload, profile, true, dryRun)
}

// ClaimWindowForSnapshot finds the restored Firefox window for snapshot and moves it to workspace.
func (b *Browser) ClaimWindowForSnapshot(snapshot string, workspace int) error {
	return b.ClaimWindow(snapshot, workspace)
}

func (b *Browser) executeWindows(args []string) (string, error) {
	all := false
	for _, arg := range args {
		switch {
		case arg == "--all":
			all = true
		default:
			return "", fmt.Errorf(browserWindowsUsage)
		}
	}

	profile, err := discoverFirefoxProfile("")
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
	var positional []string
	for _, arg := range args {
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

	profile, err := b.snapshotProfile(selector)
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

	workspace := 0
	if selector == "active" {
		if active := b.activeFirefoxWindow(); active != nil && active.Workspace.ID > 0 {
			workspace = active.Workspace.ID
		}
	}
	return b.writeSnapshot(name, profile, windowIndex, workspace, store)
}

func (b *Browser) snapshotProfile(string) (firefoxProfile, error) {
	return discoverFirefoxProfile("")
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
	return cfg.Force
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
	if len(args) != 0 {
		return "", errors.New(browserLaunchUsage)
	}

	profile, err := discoverFirefoxProfile("")
	if err != nil {
		return "", err
	}
	if err := clearSessionStore(profile); err != nil {
		return "", err
	}

	cmd := append(b.browserCommandParts(), "--profile", profile.Root, "--new-window", firefoxNewtab)
	if b.hypr != nil {
		if err := b.hypr.Exec(shellQuoteCommand(cmd)); err != nil {
			return "", err
		}
	} else {
		if err := exec.Command(cmd[0], cmd[1:]...).Start(); err != nil {
			return "", err
		}
	}
	return "launched browser (session cleared)", nil
}
