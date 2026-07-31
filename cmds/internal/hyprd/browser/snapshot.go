package browser

// snapshot.go writes repo-backed snapshot artifacts and summarizes Firefox windows into launch-ready config.

import (
	"cmp"
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

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
	Collapsed  bool   `yaml:"collapsed,omitempty"`
}

type browserSnapshotSummary struct {
	Name      string                `yaml:"name"`
	Workspace int                   `yaml:"workspace,omitempty"`
	Window    browserSnapshotWindow `yaml:"window"`
	Browser   config.BrowserConfig  `yaml:"browser"`
	Tabs      []browserTabSummary   `yaml:"tabs,omitempty"`
}

type browserSnapshotWindow struct {
	SelectedTab          int    `yaml:"selected_tab"`
	TabCount             int    `yaml:"tab_count"`
	GroupCount           int    `yaml:"group_count"`
	SelectedTitle        string `yaml:"selected_title,omitempty"`
	SelectedURL          string `yaml:"selected_url,omitempty"`
	HyprOrderMatchesTabs bool   `yaml:"hypr_order_matches_tabs"`
}

func (b *Browser) writeSnapshot(name string, _ firefoxProfile, windowIndex, workspace int, store *firefoxSessionStore) (string, error) {
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
		Name:      slug,
		Workspace: workspace,
		Window:    windowSummary.browserSnapshotWindow,
		Browser:   windowSummary.Browser,
		Tabs:      windowSummary.Tabs,
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
			Collapsed:  group.Collapsed,
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
	summary, err := readSnapshotSummary(dir)
	if err != nil {
		return "", err
	}
	return summary.Window.SelectedTitle, nil
}

// LayoutWindowTitles returns current and snapshot titles in live-match priority order.
func (b *Browser) LayoutWindowTitles(name string) ([]string, error) {
	dir, err := resolveSnapshotDir(name)
	if err != nil {
		return nil, err
	}
	summary, err := readSnapshotSummary(dir)
	if err != nil {
		return nil, err
	}

	current, found, liveErr := b.currentLayoutWindowTitle(summary.Name)
	if liveErr != nil {
		// A missing or mid-flush live session must retain the pre-stamp title behavior.
		found = false
	}
	return layoutTitleCandidates(current, found, summary.Window.SelectedTitle), nil
}

func layoutTitleCandidates(current string, found bool, snapshot string) []string {
	var titles []string
	if found && strings.TrimSpace(current) != "" {
		titles = append(titles, current)
	}
	if strings.TrimSpace(snapshot) != "" && (len(titles) == 0 || snapshot != titles[0]) {
		titles = append(titles, snapshot)
	}
	return titles
}

func layoutTitleMatches(windowTitle string, titles []string) bool {
	windowTitle = trimFirefoxTitle(windowTitle)
	for _, title := range titles {
		if titlesMatch(windowTitle, title) {
			return true
		}
	}
	return false
}

func layoutWindowForTitles(clients []hypr.Window, titles []string) (hypr.Window, bool) {
	for _, title := range titles {
		for _, client := range clients {
			if isFirefoxWindow(client) && titlesMatch(trimFirefoxTitle(client.Title), title) {
				return client, true
			}
		}
	}
	return hypr.Window{}, false
}

func (b *Browser) layoutWindow(snapshot string) (hypr.Window, bool, error) {
	if b.hypr == nil {
		return hypr.Window{}, false, fmt.Errorf("no hyprland client")
	}
	titles, err := b.LayoutWindowTitles(snapshot)
	if err != nil {
		return hypr.Window{}, false, err
	}
	clients, err := b.hypr.Clients()
	if err != nil {
		return hypr.Window{}, false, err
	}
	window, found := layoutWindowForTitles(clients, titles)
	return window, found, nil
}

// LayoutWindowIsOpen reports whether Hyprland has the snapshot's current or fallback window title.
func (b *Browser) LayoutWindowIsOpen(snapshot string) (bool, error) {
	_, found, err := b.layoutWindow(snapshot)
	return found, err
}

// SnapshotMatchesWindowTitle reports whether a Firefox title matches the live layout title or its snapshot fallback.
func SnapshotMatchesWindowTitle(snapshot, windowTitle string) bool {
	titles, err := (&Browser{}).LayoutWindowTitles(snapshot)
	if err != nil {
		return false
	}
	return layoutTitleMatches(windowTitle, titles)
}

// ClaimWindow finds a Firefox window matching the snapshot's selected title and moves it to the target workspace.
func (b *Browser) ClaimWindow(snapshot string, workspace int) error {
	if b.hypr == nil {
		return fmt.Errorf("no hyprland client")
	}

	window, found, err := b.layoutWindow(snapshot)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no Firefox window for snapshot %q", snapshot)
	}
	if window.Workspace.ID == workspace {
		return nil
	}
	return b.hypr.MoveWindowToWorkspace(window.Address, strconv.Itoa(workspace), false)
}

// buildSessionPayload constructs minimal Firefox session JSON from snapshot metadata.
//
// This avoids storing raw Firefox session data (which contains cookies, formdata, storage).
func buildSessionPayload(dir string) ([]byte, error) {
	meta, err := readSnapshotSummary(dir)
	if err != nil {
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
			"id":        tab.GroupID,
			"name":      tab.Group,
			"color":     tab.GroupColor,
			"collapsed": tab.Collapsed,
		})
	}

	window := map[string]any{
		"tabs":     tabs,
		"selected": meta.Window.SelectedTab,
		"extData":  map[string]string{layoutExtDataKey: meta.Name},
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

func buildCombinedSessionPayload(dirs []string) ([]byte, error) {
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no browser snapshots to restore")
	}

	combined := map[string]any{
		"version":        []any{"sessionrestore", 1},
		"windows":        []any{},
		"selectedWindow": 1,
		"_closedWindows": []any{},
	}
	windows := make([]json.RawMessage, 0, len(dirs))
	for _, dir := range dirs {
		payload, err := buildSessionPayload(dir)
		if err != nil {
			return nil, err
		}
		var doc struct {
			Windows []json.RawMessage `json:"windows"`
		}
		if err := json.Unmarshal(payload, &doc); err != nil {
			return nil, fmt.Errorf("parse generated session for %s: %w", dir, err)
		}
		if len(doc.Windows) == 0 {
			return nil, fmt.Errorf("snapshot %s generated no Firefox windows", dir)
		}
		windows = append(windows, doc.Windows...)
	}
	combined["windows"] = windows
	return json.MarshalIndent(combined, "", "  ")
}

func readSnapshotSummary(dir string) (browserSnapshotSummary, error) {
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.yaml"))
	if err != nil {
		return browserSnapshotSummary{}, err
	}
	var summary browserSnapshotSummary
	if err := yaml.Unmarshal(data, &summary); err != nil {
		return browserSnapshotSummary{}, err
	}
	return summary, nil
}

// loadSnapshotPayload returns Firefox session JSON generated from a named snapshot's metadata.
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

// repoSessionsRoot is the canonical snapshot root tracked with the dotfiles reorg under cmds/internal/hyprd.
func repoSessionsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "dotfiles", "cmds", "internal", "hyprd", "browser", "sessions"), nil
}

// legacyBrowserStateRoot is read-only compatibility for snapshots from the pre-hyprd browser helper.
func legacyBrowserStateRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "firefox-sessions"), nil
}

// snapshotRoots lists snapshot lookup roots in precedence order: repo, then legacy state.
func snapshotRoots() []string {
	var roots []string
	if root, err := repoSessionsRoot(); err == nil {
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
