package browser

// restore.go implements browser restore flows, including dry-runs, profile backup, and exact session injection.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dotfiles/daemons/config"
)

func (b *Browser) executeRestore(args []string) (string, error) {
	var (
		mode       = "exact"
		profileArg string
		force      bool
		dryRun     bool
		positional []string
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force":
			force = true
		case arg == "--dry-run":
			dryRun = true
		case arg == "--mode":
			if i+1 >= len(args) {
				return "", fmt.Errorf(browserRestoreUsage)
			}
			i++
			mode = args[i]
		case strings.HasPrefix(arg, "--mode="):
			mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", fmt.Errorf(browserRestoreUsage)
			}
			i++
			profileArg = args[i]
		case strings.HasPrefix(arg, "--profile="):
			profileArg = strings.TrimPrefix(arg, "--profile=")
		case strings.HasPrefix(arg, "--"):
			return "", fmt.Errorf(browserRestoreUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return "", fmt.Errorf(browserRestoreUsage)
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "urls" && mode != "exact" {
		return "", fmt.Errorf(browserRestoreUsage)
	}

	name := positional[0]
	dir, store, err := b.loadSnapshotSession(name)
	if err != nil {
		return "", err
	}

	if mode == "urls" {
		return b.restoreSnapshotURLs(store, dryRun)
	}

	profile, err := restoreProfileForSnapshot(dir, profileArg)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(name, dir, profile, force || mode == "exact", dryRun)
}

func (b *Browser) restoreSnapshotURLs(store *firefoxSessionStore, dryRun bool) (string, error) {
	if len(store.Windows) == 0 {
		return "", fmt.Errorf("snapshot has no windows")
	}

	var tabs []browserTabSummary
	for _, tab := range summarizeFirefoxWindow(store.Windows[0]).Tabs {
		if !tab.Hidden && tab.URL != "" {
			tabs = append(tabs, tab)
		}
	}
	if len(tabs) == 0 {
		return "", fmt.Errorf("snapshot has no visible tabs to restore")
	}

	commands := make([][]string, 0, len(tabs))
	first := append(slices.Clone(b.browserCommandParts()), "--new-window", tabs[0].URL)
	commands = append(commands, first)
	for _, tab := range tabs[1:] {
		commands = append(commands, append(slices.Clone(b.browserCommandParts()), "--new-tab", tab.URL))
	}

	if dryRun {
		var lines []string
		for _, cmd := range commands {
			lines = append(lines, shellQuoteCommand(cmd))
		}
		return strings.Join(lines, "\n"), nil
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return "", err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Sprintf("restored urls: %d tabs", len(tabs)), nil
}

func (b *Browser) restoreSnapshotExact(name, snapshotDir string, profile firefoxProfile, force, dryRun bool) (string, error) {
	payload, err := buildSessionPayload(snapshotDir)
	if err != nil {
		return "", err
	}
	return b.injectAndLaunch(payload, profile, force, dryRun)
}

func restoreBackupDir() (string, error) {
	root, err := browserStateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "_restore-backups", time.Now().Format("20060102-150405")), nil
}

// BatchExactEntry pairs a browser config with its target workspace for batch restore.
type BatchExactEntry struct {
	Config    config.BrowserConfig
	Workspace int
}

// RestoreBatchExact merges windows from multiple snapshots into one Firefox session and restores once.
func (b *Browser) RestoreBatchExact(entries []BatchExactEntry, dryRun bool) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("no snapshots to restore")
	}

	var dirs []string
	for _, e := range entries {
		dir, err := resolveSnapshotDir(e.Config.Snapshot)
		if err != nil {
			return "", fmt.Errorf("snapshot %q: %w", e.Config.Snapshot, err)
		}
		dirs = append(dirs, dir)
	}

	var profile firefoxProfile
	payload, err := mergeSnapshotPayloads(dirs)
	if err != nil {
		return "", err
	}
	for i, dir := range dirs {
		candidate, err := restoreProfileForSnapshot(dir, entries[i].Config.Profile)
		if err != nil {
			return "", err
		}
		if i == 0 {
			profile = candidate
			continue
		}
		if filepath.Clean(candidate.Root) != filepath.Clean(profile.Root) {
			return "", fmt.Errorf("cannot batch restore snapshots with different profiles: %s and %s", profile.Root, candidate.Root)
		}
	}

	force := false
	for _, e := range entries {
		if browserForce(e.Config) {
			force = true
			break
		}
	}

	return b.injectAndLaunch(payload, profile, force, dryRun)
}

func restoreProfileForSnapshot(snapshotDir, override string) (firefoxProfile, error) {
	if override != "" {
		return discoverFirefoxProfile(override)
	}

	meta, err := readSnapshotSummary(snapshotDir)
	if err != nil {
		return firefoxProfile{}, err
	}
	if meta.Profile.Path != "" && isDir(meta.Profile.Path) {
		return firefoxProfile{
			Root:       filepath.Clean(meta.Profile.Path),
			Name:       meta.Profile.Name,
			InstallKey: meta.Profile.InstallKey,
		}, nil
	}
	if meta.Profile.Name != "" {
		return discoverFirefoxProfile(meta.Profile.Name)
	}
	return discoverFirefoxProfile("")
}

// mergeSnapshotPayloads generates session JSON from each snapshot's metadata and combines their windows.
func mergeSnapshotPayloads(dirs []string) ([]byte, error) {
	var allWindows []json.RawMessage

	for _, dir := range dirs {
		payload, err := buildSessionPayload(dir)
		if err != nil {
			return nil, fmt.Errorf("build session for %s: %w", dir, err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(payload, &doc); err != nil {
			return nil, fmt.Errorf("unmarshal session from %s: %w", dir, err)
		}
		var windows []json.RawMessage
		if err := json.Unmarshal(doc["windows"], &windows); err != nil {
			return nil, fmt.Errorf("unmarshal windows from %s: %w", dir, err)
		}
		allWindows = append(allWindows, windows...)
	}

	merged, err := json.Marshal(allWindows)
	if err != nil {
		return nil, err
	}

	session := map[string]json.RawMessage{
		"version":        json.RawMessage(`["sessionrestore",1]`),
		"windows":        merged,
		"selectedWindow": json.RawMessage(`1`),
		"_closedWindows": json.RawMessage(`[]`),
	}
	return json.MarshalIndent(session, "", "  ")
}

// injectAndLaunch stops Firefox, backs up session files, injects the payload, and launches.
func (b *Browser) injectAndLaunch(payload []byte, profile firefoxProfile, force, dryRun bool) (string, error) {
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")

	if dryRun {
		return fmt.Sprintf("would stop Firefox (force=%t)\nwould inject %d bytes into %s\nwould launch Firefox", force, len(payload), target), nil
	}

	if err := stopFirefox(force); err != nil {
		return "", err
	}
	backupDir, err := backupFirefoxSessionFiles(profile)
	if err != nil {
		return "", err
	}

	if err := encodeMozillaLZ4File(target, payload); err != nil {
		return "", err
	}

	backupsDir := filepath.Join(profile.Root, "sessionstore-backups")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return "", err
	}
	for _, name := range []string{"recovery.jsonlz4", "recovery.baklz4"} {
		if err := encodeMozillaLZ4File(filepath.Join(backupsDir, name), payload); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(profile.Root, "sessionCheckpoints.json"), defaultSessionCheckpoints, 0o644); err != nil {
		return "", err
	}

	if err := setFirefoxPref(profile, "browser.sessionstore.resume_session_once", "true"); err != nil {
		return "", err
	}

	if err := b.launchFirefoxProfile(profile); err != nil {
		return "", err
	}
	return fmt.Sprintf("restored %d windows into %s\nbackup: %s", countPayloadWindows(payload), profile.Root, backupDir), nil
}

func countPayloadWindows(payload []byte) int {
	var doc struct {
		Windows []json.RawMessage `json:"windows"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return 0
	}
	return len(doc.Windows)
}

func backupFirefoxSessionFiles(profile firefoxProfile) (string, error) {
	backupDir, err := restoreBackupDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}

	for _, name := range []string{"sessionstore.jsonlz4", "sessionCheckpoints.json"} {
		source := filepath.Join(profile.Root, name)
		if fileExists(source) {
			if err := os.Rename(source, filepath.Join(backupDir, name)); err != nil {
				return "", err
			}
		}
	}

	backupsDir := filepath.Join(profile.Root, "sessionstore-backups")
	if isDir(backupsDir) {
		if err := os.Rename(backupsDir, filepath.Join(backupDir, "sessionstore-backups")); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return "", err
	}
	return backupDir, nil
}
