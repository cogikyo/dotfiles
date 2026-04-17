// Package browser exposes Firefox session inspection and restoration for hyprd.
//
// Reads Firefox's mozlz4-compressed sessionstore files, produces named snapshots, and restores a saved set of tabs
// either by URL (--mode urls) or by replacing the session file wholesale (--mode exact).
package browser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"

	"gopkg.in/yaml.v3"
)

const (
	browserUsage         = "usage: browser {windows|snapshot|show|hypr|restore} ..."
	browserWindowsUsage  = "usage: browser windows [--all] [--profile <name|path>]"
	browserSnapshotUsage = "usage: browser snapshot <name> [active|largest|index] [--profile <name|path>]"
	browserShowUsage     = "usage: browser show <name> [snapshot]"
	browserHyprUsage     = "usage: browser hypr <name> [snapshot]"
	browserRestoreUsage  = "usage: browser restore <name> [snapshot] [--mode urls|exact] [--profile <name|path>] [--force] [--dry-run]"
)

// Browser is the hyprd subcommand handler for Firefox session inspection, snapshotting, and restoration.
type Browser struct {
	hypr  *hypr.Client
	state *state.State
}

// NewBrowser returns a Browser using h for active-window queries and s for the configured browser command.
//
// Both may be nil; helpers then fall back to firefoxBinary and skip Hypr active-window detection.
func NewBrowser(h *hypr.Client, s *state.State) *Browser {
	return &Browser{hypr: h, state: s}
}

// Execute dispatches a browser subcommand.
//
// args is the raw argument string following "browser" (e.g. "snapshot work active").
func (b *Browser) Execute(args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return "", fmt.Errorf(browserUsage)
	}

	switch parts[0] {
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

// ResolveLaunchConfig materializes cfg by pulling tab/group data from the snapshot named in cfg.Snapshot.
//
// If the snapshot is missing but cfg already lists URLs, cfg is returned as-is so launch proceeds from inline config.
func (b *Browser) ResolveLaunchConfig(cfg config.BrowserConfig) (config.BrowserConfig, error) {
	if cfg.Snapshot == "" {
		return cfg, nil
	}

	snapshotCfg, err := b.SnapshotConfig(cfg.Snapshot, "")
	if err != nil {
		if len(cfg.AllURLs()) > 0 {
			return cfg, nil
		}
		return config.BrowserConfig{}, err
	}
	snapshotCfg.Snapshot = cfg.Snapshot
	return snapshotCfg, nil
}

// SnapshotConfig returns the BrowserConfig derived from the first window of the named snapshot.
//
// snapshotID may be "" for latest. Errors when the snapshot has no windows.
func (b *Browser) SnapshotConfig(name, snapshotID string) (config.BrowserConfig, error) {
	_, store, err := b.loadSnapshotSession(name, snapshotID)
	if err != nil {
		return config.BrowserConfig{}, err
	}
	if len(store.Windows) == 0 {
		return config.BrowserConfig{}, fmt.Errorf("snapshot %q has no windows", name)
	}
	return summarizeFirefoxWindow(store.Windows[0]).Browser, nil
}

// UsesExactRestore reports whether cfg opts into session-file replacement (mode "exact") over per-URL launch.
func (b *Browser) UsesExactRestore(cfg config.BrowserConfig) bool {
	return browserMode(cfg) == "exact"
}

// RestoreConfiguredSnapshot performs an exact restore driven by cfg.
//
// Requires cfg.Mode == "exact" and a non-empty cfg.Snapshot. dryRun prints the planned actions without touching disk.
func (b *Browser) RestoreConfiguredSnapshot(cfg config.BrowserConfig, dryRun bool) (string, error) {
	if !b.UsesExactRestore(cfg) {
		return "", fmt.Errorf("browser restore mode %q is not exact", browserMode(cfg))
	}
	if cfg.Snapshot == "" {
		return "", fmt.Errorf("browser exact restore requires snapshot")
	}

	dir, store, err := b.loadSnapshotSession(cfg.Snapshot, "")
	if err != nil {
		return "", err
	}
	profile, err := discoverFirefoxProfile(cfg.Profile)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(cfg.Snapshot, dir, store, profile, cfg.Force, dryRun)
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
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf(browserShowUsage)
	}
	dir, err := resolveSnapshotDir(args[0], optionalArg(args, 1))
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
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf(browserHyprUsage)
	}
	cfg, err := b.SnapshotConfig(args[0], optionalArg(args, 1))
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

// parseProfileFlag extracts --profile <val> or --profile=<val> from args.
//
// Returns the value, the remaining args with the flag stripped, and usage as an error when --profile lacks a value.
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
		return "urls"
	}
	return mode
}

func optionalArg(args []string, idx int) string {
	if idx >= len(args) {
		return ""
	}
	return args[idx]
}

func shellQuoteCommand(parts []string) string {
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = strconv.Quote(part)
	}
	return strings.Join(quoted, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
