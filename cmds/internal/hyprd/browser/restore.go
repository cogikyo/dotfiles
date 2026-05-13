package browser

// restore.go implements URL replay and exact Firefox session injection with runtime backups.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (b *Browser) executeRestore(args []string) (string, error) {
	var (
		force      bool
		dryRun     bool
		positional []string
	)
	for i := range args {
		arg := args[i]
		switch {
		case arg == "--force":
			force = true
		case arg == "--dry-run":
			dryRun = true
		case len(arg) > 0 && arg[0] == '-':
			return "", fmt.Errorf(browserRestoreUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return "", fmt.Errorf(browserRestoreUsage)
	}

	name := positional[0]
	dir, _, err := b.loadSnapshotSession(name)
	if err != nil {
		return "", err
	}

	profile, err := profileForSnapshot(name, !dryRun)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(name, dir, profile, force, dryRun)
}

func (b *Browser) restoreSnapshotExact(name, snapshotDir string, profile firefoxProfile, force, dryRun bool) (string, error) {
	payload, err := buildSessionPayload(snapshotDir)
	if err != nil {
		return "", err
	}
	return b.injectAndLaunchWithStop(payload, profile, force, dryRun, stopFuncForProfile(profile))
}

func restoreBackupDir(profile firefoxProfile) string {
	return filepath.Join(profile.Root, "hyprd-restore-backups", time.Now().Format("20060102-150405"))
}

// injectAndLaunch stops Firefox, backs up session files inside the profile, injects the payload, and launches.
func (b *Browser) injectAndLaunch(payload []byte, profile firefoxProfile, force, dryRun bool) (string, error) {
	return b.injectAndLaunchWithStop(payload, profile, force, dryRun, stopFuncForProfile(profile))
}

func stopFuncForProfile(profile firefoxProfile) func(bool) error {
	if profile.Main {
		return stopFirefox
	}
	return func(force bool) error {
		return stopFirefoxProfile(profile, force)
	}
}

func (b *Browser) injectAndLaunchWithStop(payload []byte, profile firefoxProfile, force, dryRun bool, stop func(bool) error) (string, error) {
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")

	if dryRun {
		return fmt.Sprintf("would stop Firefox (force=%t)\nwould inject %d bytes into %s\nwould launch Firefox", force, len(payload), target), nil
	}

	if err := stop(force); err != nil {
		return "", err
	}
	backupDir, err := injectSessionPayload(profile, payload)
	if err != nil {
		return "", err
	}

	if err := b.launchFirefoxProfile(profile); err != nil {
		return "", err
	}
	return fmt.Sprintf("restored %d windows into %s\nbackup: %s", countPayloadWindows(payload), profile.Root, backupDir), nil
}

func injectSessionPayload(profile firefoxProfile, payload []byte) (string, error) {
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")
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
	return backupDir, nil
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
	backupDir := restoreBackupDir(profile)
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
