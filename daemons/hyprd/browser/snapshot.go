package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"dotfiles/daemons/config"

	"gopkg.in/yaml.v3"
)

var snapshotNamePattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type browserWindowSummary struct {
	SelectedTab          int                  `yaml:"selected_tab"`
	TabCount             int                  `yaml:"tab_count"`
	GroupCount           int                  `yaml:"group_count"`
	SelectedTitle        string               `yaml:"selected_title,omitempty"`
	SelectedURL          string               `yaml:"selected_url,omitempty"`
	HyprOrderMatchesTabs bool                 `yaml:"hypr_order_matches_tabs"`
	Browser              config.BrowserConfig `yaml:"-"`
	Tabs                 []browserTabSummary  `yaml:"-"`
}

type browserTabSummary struct {
	Position   int    `yaml:"position"`
	Title      string `yaml:"title,omitempty"`
	URL        string `yaml:"url,omitempty"`
	Pinned     bool   `yaml:"pinned,omitempty"`
	Hidden     bool   `yaml:"hidden,omitempty"`
	GroupID    string `yaml:"group_id,omitempty"`
	Group      string `yaml:"group,omitempty"`
	GroupColor string `yaml:"group_color,omitempty"`
}

type browserSnapshotSummary struct {
	Name      string                `yaml:"name"`
	CreatedAt string                `yaml:"created_at"`
	Profile   browserProfileSummary `yaml:"profile"`
	Source    browserSourceSummary  `yaml:"source"`
	Window    browserSnapshotWindow `yaml:"window"`
	Browser   config.BrowserConfig  `yaml:"browser"`
	Tabs      []browserTabSummary   `yaml:"tabs,omitempty"`
}

type browserProfileSummary struct {
	Name       string `yaml:"name"`
	Path       string `yaml:"path"`
	InstallKey string `yaml:"install_key,omitempty"`
}

type browserSourceSummary struct {
	SessionFile string `yaml:"session_file"`
	WindowIndex int    `yaml:"window_index"`
}

type browserSnapshotWindow struct {
	SelectedTab          int    `yaml:"selected_tab"`
	TabCount             int    `yaml:"tab_count"`
	GroupCount           int    `yaml:"group_count"`
	SelectedTitle        string `yaml:"selected_title,omitempty"`
	SelectedURL          string `yaml:"selected_url,omitempty"`
	HyprOrderMatchesTabs bool   `yaml:"hypr_order_matches_tabs"`
}

// writeSnapshot persists window windowIndex of store under a slugified name.
//
// Layout under browserStateRoot():
//
//	<slug>/<timestamp>/snapshot.yaml  — human-readable summary + BrowserConfig
//	<slug>/<timestamp>/session.json   — single-window session envelope (uncompressed)
//	<slug>/latest                     — symlink to the newest <timestamp>
//
// Returns the absolute path to the new snapshot directory.
func (b *Browser) writeSnapshot(name string, profile firefoxProfile, windowIndex int, store *firefoxSessionStore) (string, error) {
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}

	root, err := browserStateRoot()
	if err != nil {
		return "", err
	}
	baseDir := filepath.Join(root, slug)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}

	snapshotID := time.Now().Format("20060102-150405")
	targetDir := filepath.Join(baseDir, snapshotID)
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		return "", err
	}

	windowSummary := summarizeFirefoxWindow(store.Windows[windowIndex])
	summary := browserSnapshotSummary{
		Name:      slug,
		CreatedAt: time.Now().Format(time.RFC3339),
		Profile: browserProfileSummary{
			Name:       profile.Name,
			Path:       profile.Root,
			InstallKey: profile.InstallKey,
		},
		Source: browserSourceSummary{
			SessionFile: store.Source,
			WindowIndex: windowIndex + 1,
		},
		Window: browserSnapshotWindow{
			SelectedTab:          windowSummary.SelectedTab,
			TabCount:             windowSummary.TabCount,
			GroupCount:           windowSummary.GroupCount,
			SelectedTitle:        windowSummary.SelectedTitle,
			SelectedURL:          windowSummary.SelectedURL,
			HyprOrderMatchesTabs: windowSummary.HyprOrderMatchesTabs,
		},
		Browser: windowSummary.Browser,
		Tabs:    windowSummary.Tabs,
	}

	summaryData, err := yaml.Marshal(summary)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "snapshot.yaml"), summaryData, 0o644); err != nil {
		return "", err
	}

	sessionData, err := snapshotSessionJSON(store.Raw, windowIndex)
	if err != nil {
		return "", err
	}
	sessionData = append(sessionData, '\n')
	if err := os.WriteFile(filepath.Join(targetDir, "session.json"), sessionData, 0o644); err != nil {
		return "", err
	}

	if err := updateLatestSnapshotLink(baseDir, snapshotID); err != nil {
		return "", err
	}
	return targetDir, nil
}

// snapshotSessionJSON builds a single-window session envelope suitable for writing back via encodeMozillaLZ4File.
//
// Envelope-level fields (version, session, global, maxSplitViewId, savedGroups) pass through verbatim so the
// restored session looks identical to Firefox aside from the narrowed window list.
func snapshotSessionJSON(raw firefoxSessionEnvelope, windowIndex int) ([]byte, error) {
	if windowIndex < 0 || windowIndex >= len(raw.Windows) {
		return nil, fmt.Errorf("window index %d is out of range", windowIndex+1)
	}

	doc := map[string]any{
		"windows":        []json.RawMessage{raw.Windows[windowIndex]},
		"selectedWindow": 1,
		"_closedWindows": []any{},
	}
	if len(raw.Version) > 0 {
		doc["version"] = json.RawMessage(raw.Version)
	}
	if len(raw.Session) > 0 {
		doc["session"] = json.RawMessage(raw.Session)
	}
	if len(raw.Global) > 0 {
		doc["global"] = json.RawMessage(raw.Global)
	}
	if len(raw.MaxSplitViewID) > 0 {
		doc["maxSplitViewId"] = json.RawMessage(raw.MaxSplitViewID)
	}
	if len(raw.SavedGroups) > 0 {
		doc["savedGroups"] = json.RawMessage(raw.SavedGroups)
	}
	return json.MarshalIndent(doc, "", "  ")
}

// updateLatestSnapshotLink atomically points baseDir/latest at snapshotID via a temp symlink + rename.
func updateLatestSnapshotLink(baseDir, snapshotID string) error {
	latest := filepath.Join(baseDir, "latest")
	tmp := filepath.Join(baseDir, ".latest.tmp")
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.Symlink(snapshotID, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, latest); err != nil {
		return err
	}
	return nil
}

// summarizeFirefoxWindow projects a session-store window into a snapshot summary and a launch-ready BrowserConfig.
//
// Grouping rules:
//   - Pinned tabs -> BrowserConfig.Pinned (never assigned to a group).
//   - Ungrouped visible tabs -> BrowserConfig.URLs.
//   - Grouped tabs -> BrowserConfig.Groups, preserving declared order, then appending orphan groups referenced by
//     tabs but missing from window.Groups (older session files).
//
// HyprOrderMatchesTabs is true when re-launching the BrowserConfig would reproduce the tab order.
// Callers surface this to flag when URL-mode restore would scramble the layout.
func summarizeFirefoxWindow(window firefoxWindow) browserWindowSummary {
	groupByID := make(map[string]firefoxGroup, len(window.Groups))
	for _, group := range window.Groups {
		groupByID[group.ID] = group
	}

	var summaryTabs []browserTabSummary
	for i, tab := range window.Tabs {
		entry := selectedEntry(tab)
		groupID := tabGroupID(tab)
		group := groupByID[groupID]
		summaryTabs = append(summaryTabs, browserTabSummary{
			Position:   i + 1,
			Title:      entry.Title,
			URL:        entry.URL,
			Pinned:     tab.Pinned,
			Hidden:     tab.Hidden,
			GroupID:    groupID,
			Group:      groupName(group, groupID),
			GroupColor: group.Color,
		})
	}

	browserCfg := config.BrowserConfig{}
	for _, tab := range summaryTabs {
		if tab.Hidden || tab.URL == "" {
			continue
		}
		switch {
		case tab.Pinned:
			browserCfg.Pinned = append(browserCfg.Pinned, tab.URL)
		case tab.GroupID == "":
			browserCfg.URLs = append(browserCfg.URLs, tab.URL)
		}
	}

	seenGroups := map[string]struct{}{}
	for _, group := range window.Groups {
		if group.ID == "" {
			continue
		}
		seenGroups[group.ID] = struct{}{}
		groupCfg := config.BrowserGroup{
			Name:      groupName(group, group.ID),
			Color:     group.Color,
			Collapsed: group.Collapsed,
		}
		for _, tab := range summaryTabs {
			if !tab.Hidden && tab.URL != "" && tab.GroupID == group.ID {
				groupCfg.URLs = append(groupCfg.URLs, tab.URL)
			}
		}
		browserCfg.Groups = append(browserCfg.Groups, groupCfg)
	}

	for _, tab := range summaryTabs {
		if tab.GroupID == "" {
			continue
		}
		if _, ok := seenGroups[tab.GroupID]; ok {
			continue
		}
		seenGroups[tab.GroupID] = struct{}{}
		groupCfg := config.BrowserGroup{Name: firstNonEmpty(tab.Group, tab.GroupID)}
		for _, candidate := range summaryTabs {
			if !candidate.Hidden && candidate.URL != "" && candidate.GroupID == tab.GroupID {
				groupCfg.URLs = append(groupCfg.URLs, candidate.URL)
			}
		}
		browserCfg.Groups = append(browserCfg.Groups, groupCfg)
	}

	var liveOrder []string
	for _, tab := range summaryTabs {
		if !tab.Hidden && tab.URL != "" {
			liveOrder = append(liveOrder, tab.URL)
		}
	}

	return browserWindowSummary{
		SelectedTab:          max(window.Selected, 1),
		TabCount:             len(window.Tabs),
		GroupCount:           len(window.Groups),
		SelectedTitle:        selectedTabTitle(window),
		SelectedURL:          selectedTabURL(window),
		HyprOrderMatchesTabs: slices.Equal(browserCfg.AllURLs(), liveOrder),
		Browser:              browserCfg,
		Tabs:                 summaryTabs,
	}
}

// resolveSnapshotDir finds the on-disk directory for a snapshot.
//
// snapshotID may be "" to mean latest.
// Both the current and legacy state roots are searched so snapshots taken before the hyprd rename still resolve.
func resolveSnapshotDir(name, snapshotID string) (string, error) {
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}

	for _, root := range snapshotRoots() {
		baseDir := filepath.Join(root, slug)
		if snapshotID != "" {
			candidate := filepath.Join(baseDir, snapshotID)
			if isDir(candidate) {
				return candidate, nil
			}
			continue
		}

		dir, err := latestSnapshotDir(baseDir)
		if err == nil {
			return dir, nil
		}
	}

	if snapshotID == "" {
		return "", fmt.Errorf("no snapshot found for %q", slug)
	}
	return "", fmt.Errorf("snapshot %q not found for %q", snapshotID, slug)
}

func latestSnapshotDir(baseDir string) (string, error) {
	latest := filepath.Join(baseDir, "latest")
	if info, err := os.Lstat(latest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(latest)
			if err != nil {
				return "", err
			}
			if isDir(resolved) {
				return resolved, nil
			}
		}
		if info.IsDir() {
			return latest, nil
		}
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "latest" {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no snapshot directories in %s", baseDir)
	}
	slices.Sort(dirs)
	return filepath.Join(baseDir, dirs[len(dirs)-1]), nil
}

// browserStateRoot returns $XDG_STATE_HOME/hyprd/browser-sessions, defaulting to ~/.local/state/hyprd/browser-sessions.
func browserStateRoot() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "hyprd", "browser-sessions"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hyprd", "browser-sessions"), nil
}

// legacyBrowserStateRoot is the pre-rename location.
//
// Still read so old snapshots keep resolving; new snapshots are only written under browserStateRoot().
func legacyBrowserStateRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "firefox-sessions"), nil
}

func snapshotRoots() []string {
	var roots []string
	if root, err := browserStateRoot(); err == nil {
		roots = append(roots, root)
	}
	if root, err := legacyBrowserStateRoot(); err == nil {
		roots = append(roots, root)
	}
	return roots
}

func slugifySnapshotName(name string) (string, error) {
	slug := strings.Trim(snapshotNamePattern.ReplaceAllString(strings.TrimSpace(name), "-"), "-")
	if slug == "" {
		return "", fmt.Errorf("snapshot name must contain at least one visible character")
	}
	return slug, nil
}
