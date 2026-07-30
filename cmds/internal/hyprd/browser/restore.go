package browser

// restore.go merges snapshot windows into Firefox sessions and injects exact session payloads.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"dotfiles/cmds/internal/hyprd/hypr"
)

const (
	firefoxPlacementRestoreTimeout = 10 * time.Second
	firefoxWindowClaimTimeout      = 5 * time.Second
)

type firefoxPlacement struct {
	title     string
	workspace string
}

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
	summary, err := readSnapshotSummary(dir)
	if err != nil {
		return "", err
	}
	if b.hypr != nil {
		open, err := b.LayoutWindowIsOpen(name)
		if err != nil {
			return "", err
		}
		if open {
			return b.claimSnapshotWorkspace(fmt.Sprintf("snapshot %q window already open", name), name, summary.Workspace, dryRun), nil
		}
	}

	profile, err := discoverFirefoxProfile("")
	if err != nil {
		return "", err
	}
	result, err := b.restoreSnapshotExact(name, dir, profile, force, dryRun)
	if err != nil {
		return "", err
	}
	return b.claimSnapshotWorkspace(result, name, summary.Workspace, dryRun), nil
}

func (b *Browser) claimSnapshotWorkspace(result, name string, workspace int, dryRun bool) string {
	if workspace <= 0 {
		return result
	}
	if dryRun {
		return fmt.Sprintf("%s\nwould claim window to workspace %d", result, workspace)
	}
	if b.hypr == nil {
		return result
	}
	if err := b.claimWindowToWorkspace(name, workspace); err != nil {
		return fmt.Sprintf("%s\nwarning: could not claim window to workspace %d: %v", result, workspace, err)
	}
	return result
}

func (b *Browser) claimWindowToWorkspace(name string, workspace int) error {
	deadline := time.Now().Add(firefoxWindowClaimTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := b.ClaimWindow(name, workspace); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return lastErr
}

func (b *Browser) restoreSnapshotExact(name, snapshotDir string, profile firefoxProfile, force, dryRun bool) (string, error) {
	snapshotPayload, err := buildSessionPayload(snapshotDir)
	if err != nil {
		return "", err
	}
	snapshotTitle, err := snapshotPayloadSelectedTitle(snapshotPayload)
	if err != nil {
		return "", err
	}
	layoutTitles, err := b.LayoutWindowTitles(name)
	if err != nil {
		return "", err
	}
	if len(layoutTitles) == 0 {
		layoutTitles = []string{snapshotTitle}
	}
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")
	if dryRun {
		return fmt.Sprintf("would stop Firefox (force=%t)\nwould merge snapshot %q into %s\nwould launch Firefox", force, name, target), nil
	}

	placements, err := b.captureFirefoxPlacements(layoutTitles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "browser restore: capture Firefox placements: %v\n", err)
	}
	// Merge only windows that are open right now; a dead Firefox has no live
	// windows, and merging its stale session file would resurrect them.
	wasRunning := FirefoxRunning()
	if err := stopFirefox(force); err != nil {
		return "", err
	}
	var livePayload []byte
	if wasRunning {
		livePayload, err = loadStoppedFirefoxSessionPayload(profile)
		if err != nil {
			return "", err
		}
	}
	payload, err := buildMergedSessionPayload(livePayload, snapshotPayload)
	if err != nil {
		return "", err
	}
	result, err := b.injectAndLaunchStopped(payload, profile)
	if err != nil {
		return "", err
	}
	if err := b.restoreFirefoxPlacements(placements, layoutTitles); err != nil {
		fmt.Fprintf(os.Stderr, "browser restore: restore Firefox placements: %v\n", err)
	}
	return result, nil
}

func (b *Browser) captureFirefoxPlacements(layoutTitles []string) ([]firefoxPlacement, error) {
	if b.hypr == nil {
		return nil, nil
	}
	clients, err := b.hypr.Clients()
	if err != nil {
		return nil, err
	}
	return firefoxPlacements(clients, layoutTitles...), nil
}

func firefoxPlacements(clients []hypr.Window, layoutTitles ...string) []firefoxPlacement {
	placements := make([]firefoxPlacement, 0, len(clients))
	for _, client := range clients {
		if !isFirefoxWindow(client) {
			continue
		}
		title := trimFirefoxTitle(client.Title)
		if layoutTitleMatches(title, layoutTitles) {
			continue
		}
		placements = append(placements, firefoxPlacement{
			title:     title,
			workspace: firefoxWorkspaceTarget(client.Workspace),
		})
	}
	return placements
}

func firefoxWorkspaceTarget(workspace hypr.WsRef) string {
	if workspace.ID < 0 {
		return workspace.Name
	}
	return strconv.Itoa(workspace.ID)
}

func (b *Browser) restoreFirefoxPlacements(placements []firefoxPlacement, layoutTitles []string) error {
	if b.hypr == nil || len(placements) == 0 {
		return nil
	}

	deadline := time.Now().Add(firefoxPlacementRestoreTimeout)
	restoredAddresses := make(map[string]struct{})
	var lastErr error
	for len(placements) > 0 && time.Now().Before(deadline) {
		clients, err := b.hypr.Clients()
		if err != nil {
			lastErr = err
			time.Sleep(250 * time.Millisecond)
			continue
		}

		usedAddresses := make(map[string]struct{}, len(clients)+len(restoredAddresses))
		for address := range restoredAddresses {
			usedAddresses[address] = struct{}{}
		}
		remaining := placements[:0]
		for _, placement := range placements {
			client, found := firefoxWindowForPlacement(clients, placement, usedAddresses)
			if !found {
				remaining = append(remaining, placement)
				continue
			}
			usedAddresses[client.Address] = struct{}{}
			if firefoxWorkspaceTarget(client.Workspace) == placement.workspace {
				restoredAddresses[client.Address] = struct{}{}
				continue
			}
			if err := b.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", placement.workspace, client.Address)); err != nil {
				lastErr = err
				remaining = append(remaining, placement)
			} else {
				restoredAddresses[client.Address] = struct{}{}
			}
		}
		placements = remaining
		if len(placements) == 0 {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if len(placements) == 1 {
		clients, err := b.hypr.Clients()
		if err != nil {
			lastErr = err
		} else if client, found := lastFirefoxPlacementWindow(clients, restoredAddresses, layoutTitles...); found {
			if err := b.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", placements[0].workspace, client.Address)); err != nil {
				lastErr = err
			} else {
				return nil
			}
		}
	}
	if lastErr != nil {
		return fmt.Errorf("%d Firefox window placements not restored: %w", len(placements), lastErr)
	}
	return fmt.Errorf("%d Firefox window placements not restored before timeout", len(placements))
}

func firefoxWindowForPlacement(clients []hypr.Window, placement firefoxPlacement, usedAddresses map[string]struct{}) (hypr.Window, bool) {
	for _, client := range clients {
		title := trimFirefoxTitle(client.Title)
		if !isFirefoxWindow(client) || !(title == "" && placement.title == "" || titlesMatch(title, placement.title)) {
			continue
		}
		if _, used := usedAddresses[client.Address]; !used {
			return client, true
		}
	}
	return hypr.Window{}, false
}

func lastFirefoxPlacementWindow(clients []hypr.Window, usedAddresses map[string]struct{}, layoutTitles ...string) (hypr.Window, bool) {
	var candidate hypr.Window
	for _, client := range clients {
		if !isFirefoxWindow(client) || layoutTitleMatches(client.Title, layoutTitles) {
			continue
		}
		if _, used := usedAddresses[client.Address]; used {
			continue
		}
		if candidate.Address != "" {
			return hypr.Window{}, false
		}
		candidate = client
	}
	return candidate, candidate.Address != ""
}

func restoreBackupDir(profile firefoxProfile) string {
	return filepath.Join(profile.Root, "hyprd-restore-backups", time.Now().Format("20060102-150405"))
}

// injectAndLaunch stops Firefox, backs up session files inside the profile, injects the payload, and launches.
func (b *Browser) injectAndLaunch(payload []byte, profile firefoxProfile, force, dryRun bool) (string, error) {
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")

	if dryRun {
		return fmt.Sprintf("would stop Firefox (force=%t)\nwould inject %d bytes into %s\nwould launch Firefox", force, len(payload), target), nil
	}

	if err := stopFirefox(force); err != nil {
		return "", err
	}
	return b.injectAndLaunchStopped(payload, profile)
}

func (b *Browser) injectAndLaunchStopped(payload []byte, profile firefoxProfile) (string, error) {
	backupDir, err := injectSessionPayload(profile, payload)
	if err != nil {
		return "", err
	}

	if err := b.launchFirefoxProfile(profile); err != nil {
		return "", err
	}
	return fmt.Sprintf("restored %d windows into %s\nbackup: %s", countPayloadWindows(payload), profile.Root, backupDir), nil
}

func loadStoppedFirefoxSessionPayload(profile firefoxProfile) ([]byte, error) {
	source, ok := firstFirefoxSessionFile([]string{
		filepath.Join(profile.Root, "sessionstore.jsonlz4"),
		filepath.Join(profile.Root, "sessionstore-backups", "recovery.jsonlz4"),
	})
	if !ok {
		return nil, nil
	}
	payload, err := decodeMozillaLZ4File(source)
	if err != nil {
		return nil, fmt.Errorf("load live Firefox session %s: %w", source, err)
	}
	return payload, nil
}

func buildMergedSessionPayload(livePayload, snapshotPayload []byte) ([]byte, error) {
	snapshotTitle, err := snapshotPayloadSelectedTitle(snapshotPayload)
	if err != nil {
		return nil, err
	}
	if len(livePayload) == 0 {
		return snapshotPayload, nil
	}

	var snapshot struct {
		Windows []json.RawMessage `json:"windows"`
	}
	if err := json.Unmarshal(snapshotPayload, &snapshot); err != nil {
		return nil, fmt.Errorf("parse generated snapshot session: %w", err)
	}
	var snapshotWindow firefoxWindow
	if err := json.Unmarshal(snapshot.Windows[0], &snapshotWindow); err != nil {
		return nil, fmt.Errorf("parse generated snapshot window: %w", err)
	}
	snapshotLayout, _ := firefoxWindowLayout(snapshotWindow)

	var live struct {
		Windows []json.RawMessage `json:"windows"`
	}
	if err := json.Unmarshal(livePayload, &live); err != nil {
		return nil, fmt.Errorf("parse live Firefox session: %w", err)
	}
	for i, raw := range live.Windows {
		var window firefoxWindow
		if err := json.Unmarshal(raw, &window); err != nil {
			return nil, fmt.Errorf("parse live Firefox window %d: %w", i+1, err)
		}
		liveLayout, stamped := firefoxWindowLayout(window)
		if stamped {
			if snapshotLayout != "" && liveLayout == snapshotLayout {
				return livePayload, nil
			}
			continue
		}
		if titlesMatch(summarizeFirefoxWindow(window).SelectedTitle, snapshotTitle) {
			return livePayload, nil
		}
	}

	var doc map[string]any
	if err := json.Unmarshal(livePayload, &doc); err != nil {
		return nil, fmt.Errorf("parse live Firefox session document: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("live Firefox session document is null")
	}
	var windows []any
	if value, exists := doc["windows"]; exists && value != nil {
		var ok bool
		windows, ok = value.([]any)
		if !ok {
			return nil, fmt.Errorf("live Firefox session windows is not an array")
		}
	}
	var snapshotWindowValue any
	if err := json.Unmarshal(snapshot.Windows[0], &snapshotWindowValue); err != nil {
		return nil, fmt.Errorf("parse generated snapshot window document: %w", err)
	}
	windows = append(windows, snapshotWindowValue)
	doc["windows"] = windows
	doc["selectedWindow"] = len(windows)

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode merged Firefox session: %w", err)
	}
	return payload, nil
}

func snapshotPayloadSelectedTitle(payload []byte) (string, error) {
	var snapshot struct {
		Windows []json.RawMessage `json:"windows"`
	}
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return "", fmt.Errorf("parse generated snapshot session: %w", err)
	}
	if len(snapshot.Windows) != 1 {
		return "", fmt.Errorf("generated snapshot session has %d windows, want 1", len(snapshot.Windows))
	}
	var window firefoxWindow
	if err := json.Unmarshal(snapshot.Windows[0], &window); err != nil {
		return "", fmt.Errorf("parse generated snapshot window: %w", err)
	}
	return summarizeFirefoxWindow(window).SelectedTitle, nil
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
