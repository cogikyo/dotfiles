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
	Name    string                `yaml:"name"`
	Profile browserProfileSummary `yaml:"profile"`
	Source  browserSourceSummary  `yaml:"source"`
	Window  browserSnapshotWindow `yaml:"window"`
	Browser config.BrowserConfig  `yaml:"browser"`
	Tabs    []browserTabSummary   `yaml:"tabs,omitempty"`
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
	dir := filepath.Join(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	windowSummary := summarizeFirefoxWindow(store.Windows[windowIndex])
	summary := browserSnapshotSummary{
		Name: slug,
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
	if err := os.WriteFile(filepath.Join(dir, "snapshot.yaml"), summaryData, 0o644); err != nil {
		return "", err
	}

	return dir, nil
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

// SnapshotSelectedTitle returns the selected tab title from a named snapshot's metadata.
func SnapshotSelectedTitle(name string) (string, error) {
	dir, err := resolveSnapshotDir(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.yaml"))
	if err != nil {
		return "", err
	}
	var summary browserSnapshotSummary
	if err := yaml.Unmarshal(data, &summary); err != nil {
		return "", err
	}
	return summary.Window.SelectedTitle, nil
}

// ClaimWindow finds a Firefox window matching the snapshot's selected title and moves it to the target workspace.
func (b *Browser) ClaimWindow(snapshot string, workspace int) error {
	if b.hypr == nil {
		return fmt.Errorf("no hyprland client")
	}

	title, err := SnapshotSelectedTitle(snapshot)
	if err != nil {
		return err
	}

	clients, err := b.hypr.Clients()
	if err != nil {
		return err
	}

	for _, c := range clients {
		if !strings.Contains(strings.ToLower(c.Class), "firefox") {
			continue
		}
		if c.Workspace.ID == workspace {
			return nil
		}
		if titlesMatch(trimFirefoxTitle(c.Title), title) {
			return b.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", workspace, c.Address))
		}
	}

	return fmt.Errorf("no Firefox window matching %q for snapshot %q", title, snapshot)
}

// buildSessionPayload constructs a minimal Firefox session JSON from snapshot metadata.
// This avoids storing raw Firefox session data (which contains cookies, formdata, storage).
func buildSessionPayload(dir string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.yaml"))
	if err != nil {
		return nil, err
	}
	var meta browserSnapshotSummary
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	tabs := make([]map[string]any, 0, len(meta.Tabs))
	for _, tab := range meta.Tabs {
		t := map[string]any{
			"entries": []map[string]string{{"url": tab.URL, "title": tab.Title}},
			"index":   1,
		}
		if tab.Pinned {
			t["pinned"] = true
		}
		if tab.Hidden {
			t["hidden"] = true
		}
		if tab.GroupID != "" {
			t["groupId"] = tab.GroupID
		}
		tabs = append(tabs, t)
	}

	seen := map[string]bool{}
	var groups []map[string]any
	for _, tab := range meta.Tabs {
		if tab.GroupID == "" || seen[tab.GroupID] {
			continue
		}
		seen[tab.GroupID] = true
		groups = append(groups, map[string]any{
			"id":    tab.GroupID,
			"name":  tab.Group,
			"color": tab.GroupColor,
		})
	}

	window := map[string]any{
		"tabs":     tabs,
		"selected": meta.Window.SelectedTab,
	}
	if len(groups) > 0 {
		window["groups"] = groups
	}

	session := map[string]any{
		"version":        []any{"sessionrestore", 1},
		"windows":        []any{window},
		"selectedWindow": 1,
		"_closedWindows": []any{},
	}
	return json.MarshalIndent(session, "", "  ")
}

// loadSnapshotPayload returns Firefox session JSON for a named snapshot,
// generating it from snapshot.yaml metadata.
func loadSnapshotPayload(name string) ([]byte, error) {
	dir, err := resolveSnapshotDir(name)
	if err != nil {
		return nil, err
	}
	return buildSessionPayload(dir)
}

func resolveSnapshotDir(name string) (string, error) {
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}

	for _, root := range snapshotRoots() {
		dir := filepath.Join(root, slug)
		if fileExists(filepath.Join(dir, "snapshot.yaml")) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no snapshot found for %q", slug)
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
