package browser

// session_store.go loads and parses Firefox sessionstore payloads and resolves target windows for snapshots.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// defaultSessionCheckpoints fakes a clean shutdown so Firefox trusts the injected session file.
var defaultSessionCheckpoints = []byte("{\"profile-after-change\":true,\"final-ui-startup\":true,\"sessionstore-windows-restored\":true}\n")

// firefoxSessionEnvelope mirrors Firefox's sessionstore JSON top-level structure.
//
// Only Windows is inspected; remaining fields pass through verbatim to survive schema changes.
type firefoxSessionEnvelope struct {
	Version        json.RawMessage   `json:"version,omitempty"`
	Session        json.RawMessage   `json:"session,omitempty"`
	Global         json.RawMessage   `json:"global,omitempty"`
	MaxSplitViewID json.RawMessage   `json:"maxSplitViewId,omitempty"`
	SavedGroups    json.RawMessage   `json:"savedGroups,omitempty"`
	Windows        []json.RawMessage `json:"windows"`
}

type firefoxSessionStore struct {
	Source  string
	Payload []byte
	Raw     firefoxSessionEnvelope
	Windows []firefoxWindow
}

type firefoxWindow struct {
	Selected int            `json:"selected"`
	Tabs     []firefoxTab   `json:"tabs"`
	Groups   []firefoxGroup `json:"groups"`
}

// firefoxTab mirrors one entry in window.tabs; Index is 1-based into Entries (history stack).
type firefoxTab struct {
	Index        int            `json:"index"`
	Entries      []firefoxEntry `json:"entries"`
	Pinned       bool           `json:"pinned"`
	Hidden       bool           `json:"hidden"`
	GroupID      string         `json:"groupId"`
	Group        string         `json:"group"`
	LastAccessed int64          `json:"lastAccessed"`
}

type firefoxEntry struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type firefoxGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Collapsed bool   `json:"collapsed"`
}

func (b *Browser) loadFirefoxSession(profile firefoxProfile) (*firefoxSessionStore, error) {
	source, err := firefoxSessionSourceFile(profile)
	if err != nil {
		return nil, err
	}
	payload, err := decodeMozillaLZ4File(source)
	if err != nil {
		return nil, err
	}
	return parseFirefoxSession(payload, source)
}

func (b *Browser) loadSnapshotSession(name, snapshotID string) (string, *firefoxSessionStore, error) {
	dir, err := resolveSnapshotDir(name, snapshotID)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return "", nil, err
	}
	store, err := parseFirefoxSession(data, filepath.Join(dir, "session.json"))
	if err != nil {
		return "", nil, err
	}
	return dir, store, nil
}

func parseFirefoxSession(payload []byte, source string) (*firefoxSessionStore, error) {
	var raw firefoxSessionEnvelope
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", source, err)
	}

	store := &firefoxSessionStore{Source: source, Payload: payload, Raw: raw}
	store.Windows = make([]firefoxWindow, 0, len(raw.Windows))
	for i, windowRaw := range raw.Windows {
		var window firefoxWindow
		if err := json.Unmarshal(windowRaw, &window); err != nil {
			return nil, fmt.Errorf("parse %s window %d: %w", source, i+1, err)
		}
		store.Windows = append(store.Windows, window)
	}
	store.Raw.Windows = nil
	return store, nil
}

// firefoxSessionSourceFile returns the first existing session file in Firefox's recovery priority order.
func firefoxSessionSourceFile(profile firefoxProfile) (string, error) {
	candidates := []string{
		filepath.Join(profile.Root, "sessionstore-backups", "recovery.jsonlz4"),
		filepath.Join(profile.Root, "sessionstore-backups", "recovery.baklz4"),
		filepath.Join(profile.Root, "sessionstore.jsonlz4"),
		filepath.Join(profile.Root, "sessionstore-backups", "previous.jsonlz4"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no sessionstore file found in %s", profile.Root)
}

// resolveWindowIndex picks a window by selector: "active" (Hypr title match), "largest", or 1-based index.
func (b *Browser) resolveWindowIndex(store *firefoxSessionStore, selector string) (int, error) {
	if len(store.Windows) == 0 {
		return 0, fmt.Errorf("session file has no windows")
	}

	if selector != "" && selector != "active" {
		if selector == "largest" {
			return pickBestWindow(store.Windows, allWindowIndexes(store.Windows)), nil
		}
		if idx, err := strconv.Atoi(selector); err == nil {
			idx--
			if idx < 0 || idx >= len(store.Windows) {
				return 0, fmt.Errorf("window index %d is out of range", idx+1)
			}
			return idx, nil
		}
		return 0, fmt.Errorf("unknown window selector %q", selector)
	}

	activeTitle := b.currentFirefoxTitle()
	if activeTitle != "" {
		for i, window := range store.Windows {
			if titlesMatch(activeTitle, selectedTabTitle(window)) {
				return i, nil
			}
		}
	}

	pool := interestingWindowIndexes(store.Windows)
	if len(pool) == 0 {
		pool = allWindowIndexes(store.Windows)
	}
	return pickBestWindow(store.Windows, pool), nil
}

func allWindowIndexes(windows []firefoxWindow) []int {
	out := make([]int, 0, len(windows))
	for i := range windows {
		out = append(out, i)
	}
	return out
}

func interestingWindowIndexes(windows []firefoxWindow) []int {
	var out []int
	for i, window := range windows {
		if windowIsInteresting(window) {
			out = append(out, i)
		}
	}
	return out
}

func pickBestWindow(windows []firefoxWindow, pool []int) int {
	best := pool[0]
	for _, idx := range pool[1:] {
		bestTabs, candTabs := len(windows[best].Tabs), len(windows[idx].Tabs)
		bestGroups, candGroups := len(windows[best].Groups), len(windows[idx].Groups)
		bestAccess, candAccess := windowLastAccessed(windows[best]), windowLastAccessed(windows[idx])

		switch {
		case candGroups > bestGroups:
			best = idx
		case candGroups == bestGroups && candTabs > bestTabs:
			best = idx
		case candGroups == bestGroups && candTabs == bestTabs && candAccess > bestAccess:
			best = idx
		}
	}
	return best
}

func windowIsInteresting(window firefoxWindow) bool {
	if len(window.Tabs) > 1 || len(window.Groups) > 0 {
		return true
	}
	for _, tab := range window.Tabs {
		if tab.Pinned {
			return true
		}
		if _, ok := trivialBrowserURLs[selectedEntry(tab).URL]; !ok {
			return true
		}
	}
	return false
}

func windowLastAccessed(window firefoxWindow) int64 {
	var best int64
	for _, tab := range window.Tabs {
		if tab.LastAccessed > best {
			best = tab.LastAccessed
		}
	}
	return best
}

func selectedEntry(tab firefoxTab) firefoxEntry {
	if len(tab.Entries) == 0 {
		return firefoxEntry{}
	}
	index := min(max(tab.Index-1, 0), len(tab.Entries)-1)
	return tab.Entries[index]
}

func selectedTabTitle(window firefoxWindow) string {
	tab := selectedWindowTab(window)
	if tab == nil {
		return ""
	}
	return selectedEntry(*tab).Title
}

func selectedTabURL(window firefoxWindow) string {
	tab := selectedWindowTab(window)
	if tab == nil {
		return ""
	}
	return selectedEntry(*tab).URL
}

func selectedWindowTab(window firefoxWindow) *firefoxTab {
	if len(window.Tabs) == 0 {
		return nil
	}
	index := min(max(window.Selected-1, 0), len(window.Tabs)-1)
	return &window.Tabs[index]
}

func groupName(group firefoxGroup, fallback string) string {
	if group.Name != "" {
		return group.Name
	}
	return fallback
}

func tabGroupID(tab firefoxTab) string {
	if tab.GroupID != "" {
		return tab.GroupID
	}
	return tab.Group
}
