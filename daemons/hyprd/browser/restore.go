package browser

// restore.go implements browser restore flows, including dry-runs, profile backup, and exact session injection.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func (b *Browser) executeRestore(args []string) (string, error) {
	var (
		mode       = "urls"
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
	if len(positional) < 1 || len(positional) > 2 {
		return "", fmt.Errorf(browserRestoreUsage)
	}
	if mode != "urls" && mode != "exact" {
		return "", fmt.Errorf(browserRestoreUsage)
	}

	name := positional[0]
	snapshotID := optionalArg(positional, 1)
	dir, store, err := b.loadSnapshotSession(name, snapshotID)
	if err != nil {
		return "", err
	}

	if mode == "urls" {
		return b.restoreSnapshotURLs(store, dryRun)
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(name, dir, profile, force, dryRun)
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
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")
	backupDir, err := restoreBackupDir()
	if err != nil {
		return "", err
	}

	if dryRun {
		lines := []string{
			fmt.Sprintf("would stop Firefox (force=%t)", force),
			fmt.Sprintf("would back up session files into %s", backupDir),
			fmt.Sprintf("would write %s", target),
			fmt.Sprintf("would write %s", filepath.Join(profile.Root, "sessionstore-backups", "recovery.jsonlz4")),
			fmt.Sprintf("would set resume_session_once=true in %s", filepath.Join(profile.Root, "prefs.js")),
			fmt.Sprintf("would launch %s", shellQuoteCommand(append(b.browserCommandParts(), "--new-instance", "--profile", profile.Root))),
		}
		return strings.Join(lines, "\n"), nil
	}

	if err := stopFirefox(force); err != nil {
		return "", err
	}
	backupDir, err = backupFirefoxSessionFiles(profile)
	if err != nil {
		return "", err
	}

	payload, err := os.ReadFile(filepath.Join(snapshotDir, "session.json"))
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
	return fmt.Sprintf("restored %s into %s\nbackup: %s", name, profile.Root, backupDir), nil
}

func restoreBackupDir() (string, error) {
	root, err := browserStateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "_restore-backups", time.Now().Format("20060102-150405")), nil
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
