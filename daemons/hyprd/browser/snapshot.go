package browser

// snapshot.go writes snapshot artifacts and summarizes Firefox windows into launch-ready browser config.

import (
	"cmp"
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
	browserSnapshotWindow `yaml:",inline"`
	Browser               config.BrowserConfig `yaml:"-"`
	Tabs                  []browserTabSummary  `yaml:"-"`
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

func (b *Browser) writeSnapshot(name string, profile firefoxProfile, windowIndex int, store *firefoxSessionStore) (string, error) {
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}

	root, err := repoSessionsRoot()
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
		Window:  windowSummary.browserSnapshotWindow,
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

	sessionData, err := snapshotSessionJSON(store.Payload, windowIndex)
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

// snapshotSessionJSON narrows the session to a single window, preserving all other top-level keys verbatim.
func snapshotSessionJSON(payload []byte, windowIndex int) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(payload, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal session envelope: %w", err)
	}

	var windows []json.RawMessage
	if err := json.Unmarshal(doc["windows"], &windows); err != nil {
		return nil, fmt.Errorf("unmarshal windows array: %w", err)
	}
	if windowIndex < 0 || windowIndex >= len(windows) {
		return nil, fmt.Errorf("window index %d is out of range", windowIndex+1)
	}

	narrowed, err := json.Marshal([]json.RawMessage{windows[windowIndex]})
	if err != nil {
		return nil, err
	}
	doc["windows"] = narrowed
	doc["selectedWindow"] = json.RawMessage(`1`)
	doc["_closedWindows"] = json.RawMessage(`[]`)
	return json.MarshalIndent(doc, "", "  ")
}

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

// summarizeFirefoxWindow projects a session-store window into a launch-ready BrowserConfig.
//
// Pinned tabs go to Pinned, ungrouped visible to URLs, grouped to Groups.
// HyprOrderMatchesTabs is true when URL-mode restore preserves the original tab order.
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
	groupURLs := map[string][]string{}
	for _, tab := range summaryTabs {
		if tab.Hidden || tab.URL == "" {
			continue
		}
		switch {
		case tab.Pinned:
			browserCfg.Pinned = append(browserCfg.Pinned, tab.URL)
		case tab.GroupID == "":
			browserCfg.URLs = append(browserCfg.URLs, tab.URL)
		default:
			groupURLs[tab.GroupID] = append(groupURLs[tab.GroupID], tab.URL)
		}
	}

	seenGroups := map[string]struct{}{}
	for _, group := range window.Groups {
		if group.ID == "" {
			continue
		}
		seenGroups[group.ID] = struct{}{}
		browserCfg.Groups = append(browserCfg.Groups, config.BrowserGroup{
			Name:      groupName(group, group.ID),
			Color:     group.Color,
			Collapsed: group.Collapsed,
			URLs:      groupURLs[group.ID],
		})
	}

	for _, tab := range summaryTabs {
		if tab.GroupID == "" {
			continue
		}
		if _, ok := seenGroups[tab.GroupID]; ok {
			continue
		}
		seenGroups[tab.GroupID] = struct{}{}
		browserCfg.Groups = append(browserCfg.Groups, config.BrowserGroup{
			Name: cmp.Or(tab.Group, tab.GroupID),
			URLs: groupURLs[tab.GroupID],
		})
	}

	var liveOrder []string
	for _, tab := range summaryTabs {
		if !tab.Hidden && tab.URL != "" {
			liveOrder = append(liveOrder, tab.URL)
		}
	}

	return browserWindowSummary{
		browserSnapshotWindow: browserSnapshotWindow{
			SelectedTab:          max(window.Selected, 1),
			TabCount:             len(window.Tabs),
			GroupCount:           len(window.Groups),
			SelectedTitle:        selectedTabTitle(window),
			SelectedURL:          selectedTabURL(window),
			HyprOrderMatchesTabs: slices.Equal(browserCfg.AllURLs(), liveOrder),
		},
		Browser: browserCfg,
		Tabs:    summaryTabs,
	}
}

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

func repoSessionsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "dotfiles", "daemons", "hyprd", "browser", "sessions"), nil
}

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

func legacyBrowserStateRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "firefox-sessions"), nil
}

func snapshotRoots() []string {
	var roots []string
	if root, err := repoSessionsRoot(); err == nil {
		roots = append(roots, root)
	}
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
