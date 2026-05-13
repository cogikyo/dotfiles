package browser

// browser_test.go covers snapshot summarization and helper selection behavior for browser restore logic.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"

	"gopkg.in/yaml.v3"
)

func TestSummarizeFirefoxWindowPreservesPinnedAndGroups(t *testing.T) {
	window := firefoxWindow{
		Selected: 2,
		Groups: []firefoxGroup{
			{ID: "grp-local", Name: "local", Color: "blue"},
			{ID: "grp-prod", Name: "prod", Color: "green", Collapsed: true},
		},
		Tabs: []firefoxTab{
			{
				Index:   1,
				Pinned:  true,
				Entries: []firefoxEntry{{URL: "https://git.example.com", Title: "Git"}},
			},
			{
				Index:   1,
				GroupID: "grp-local",
				Entries: []firefoxEntry{{URL: "https://localhost:4000", Title: "Local"}},
			},
			{
				Index:   1,
				GroupID: "grp-prod",
				Entries: []firefoxEntry{{URL: "https://app.example.com", Title: "Prod"}},
			},
			{
				Index:   1,
				Hidden:  true,
				Entries: []firefoxEntry{{URL: "https://hidden.example.com", Title: "Hidden"}},
			},
		},
	}

	summary := summarizeFirefoxWindow(window)

	if got, want := summary.Browser.Pinned, []string{"https://git.example.com"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("pinned = %v, want %v", got, want)
	}

	if got, want := len(summary.Browser.Groups), 2; got != want {
		t.Fatalf("group count = %d, want %d", got, want)
	}

	if got, want := summary.Browser.Groups[0].Name, "local"; got != want {
		t.Fatalf("first group name = %q, want %q", got, want)
	}
	if got, want := summary.Browser.Groups[0].URLs, []string{"https://localhost:4000"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("first group urls = %v, want %v", got, want)
	}

	if got, want := summary.Browser.Groups[1].Collapsed, true; got != want {
		t.Fatalf("second group collapsed = %t, want %t", got, want)
	}

	if got, want := summary.HyprOrderMatchesTabs, true; got != want {
		t.Fatalf("HyprOrderMatchesTabs = %t, want %t", got, want)
	}

	if got, want := summary.SelectedTitle, "Local"; got != want {
		t.Fatalf("SelectedTitle = %q, want %q", got, want)
	}
}

func TestSlugifySnapshotName(t *testing.T) {
	got, err := slugifySnapshotName(" leadpier / browser snapshot ")
	if err != nil {
		t.Fatalf("slugifySnapshotName returned error: %v", err)
	}
	if want := "leadpier-browser-snapshot"; got != want {
		t.Fatalf("slug = %q, want %q", got, want)
	}
}

func TestArgsUseProfile(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "hyprd-profile")
	if !argsUseProfile([]string{"firefox", "--profile", profile}, profile) {
		t.Fatalf("--profile arg did not match")
	}
	if !argsUseProfile([]string{"firefox", "--profile=" + profile}, profile) {
		t.Fatalf("--profile= arg did not match")
	}
	if argsUseProfile([]string{"firefox", "--profile", filepath.Join(t.TempDir(), "other")}, profile) {
		t.Fatalf("different profile should not match")
	}
}

func TestFirefoxOpenTargetUsesVisibleWorkspaceWindowFirst(t *testing.T) {
	clients := []hypr.Window{
		{Address: "shadow", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: -98, Name: "special:shadow"}},
		{Address: "older", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: 4}, FocusHistoryID: 5},
		{Address: "recent", InitialClass: "firefox-developer-edition", Workspace: hypr.WsRef{ID: 4}, FocusHistoryID: 1},
	}
	target, ok := firefoxOpenTargetForWorkspace(clients, 4, nil)
	if !ok {
		t.Fatalf("firefoxOpenTargetForWorkspace returned no target")
	}
	if target.Window.Address != "recent" || target.NeedsThreeBodySwap {
		t.Fatalf("target = %+v, want visible recent window without swap", target)
	}
}

func TestFirefoxOpenTargetFallsBackToThreeBodyShadow(t *testing.T) {
	clients := []hypr.Window{
		{Address: "editor", Class: "kitty", Workspace: hypr.WsRef{ID: 4}, FocusHistoryID: 0},
		{Address: "shadow", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: -98, Name: "special:shadow"}, FocusHistoryID: 7},
	}
	target, ok := firefoxOpenTargetForWorkspace(clients, 4, &state.ThreeBodyState{Shadow: "shadow"})
	if !ok {
		t.Fatalf("firefoxOpenTargetForWorkspace returned no target")
	}
	if target.Window.Address != "shadow" || !target.NeedsThreeBodySwap || target.WorkspaceID != 4 {
		t.Fatalf("target = %+v, want shadow window with swap on workspace 4", target)
	}
}

func TestFirefoxOpenTargetIgnoresStaleThreeBodyShadow(t *testing.T) {
	clients := []hypr.Window{
		{Address: "visible", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: 4}, FocusHistoryID: 1},
		{Address: "stale", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: 2, Name: "2"}, FocusHistoryID: 0},
	}
	target, ok := firefoxOpenTargetForWorkspace(clients, 4, &state.ThreeBodyState{Shadow: "stale"})
	if !ok {
		t.Fatalf("firefoxOpenTargetForWorkspace returned no target")
	}
	if target.Window.Address != "visible" || target.NeedsThreeBodySwap {
		t.Fatalf("target = %+v, want visible workspace Firefox when shadow state is stale", target)
	}
}

func TestFirefoxOpenTargetPrefersThreeBodyBrowserOverStrayVisibleWindow(t *testing.T) {
	clients := []hypr.Window{
		{Address: "stray", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: 4}, FocusHistoryID: 1},
		{Address: "shadow", Class: "firefox-developer-edition", Workspace: hypr.WsRef{ID: -98, Name: "special:shadow"}, FocusHistoryID: 7},
	}
	target, ok := firefoxOpenTargetForWorkspace(clients, 4, &state.ThreeBodyState{Master: "editor", Active: "agents", Shadow: "shadow"})
	if !ok {
		t.Fatalf("firefoxOpenTargetForWorkspace returned no target")
	}
	if target.Window.Address != "shadow" || !target.NeedsThreeBodySwap {
		t.Fatalf("target = %+v, want three-body shadow instead of stray visible Firefox", target)
	}
}

func TestDiscoverFirefoxProfileAcceptsAbsoluteDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	firefoxDir := filepath.Join(root, ".mozilla", "firefox")
	profileDir := filepath.Join(root, "profiles", "leadpier")
	if err := os.MkdirAll(firefoxDir, 0o755); err != nil {
		t.Fatalf("mkdir firefox root: %v", err)
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firefoxDir, "profiles.ini"), nil, 0o644); err != nil {
		t.Fatalf("write profiles.ini: %v", err)
	}

	profile, err := discoverFirefoxProfile(profileDir)
	if err != nil {
		t.Fatalf("discoverFirefoxProfile returned error: %v", err)
	}
	if profile.Root != profileDir || profile.Name != "leadpier" {
		t.Fatalf("profile = %+v, want root %q name leadpier", profile, profileDir)
	}
}

func TestProfileForSnapshotUsesMainForComsAndManagedForOthers(t *testing.T) {
	home := setupFirefoxHome(t)
	mainProfile := filepath.Join(home, ".mozilla", "firefox", "default-release")

	coms, err := profileForSnapshot("coms", false)
	if err != nil {
		t.Fatalf("profileForSnapshot(coms) returned error: %v", err)
	}
	if !coms.Main || coms.Root != mainProfile {
		t.Fatalf("coms profile = %+v, want main profile %q", coms, mainProfile)
	}

	leadpier, err := profileForSnapshot("leadpier", false)
	if err != nil {
		t.Fatalf("profileForSnapshot(leadpier) returned error: %v", err)
	}
	wantRoot := filepath.Join(home, ".local", "share", "hyprd", "firefox-profiles", "leadpier")
	if leadpier.Main || leadpier.Root != wantRoot || leadpier.Name != "hyprd-leadpier" {
		t.Fatalf("leadpier profile = %+v, want managed profile %q", leadpier, wantRoot)
	}
}

func TestProfileForSnapshotSeedsManagedProfile(t *testing.T) {
	home := setupFirefoxHome(t)
	mainProfile := filepath.Join(home, ".mozilla", "firefox", "default-release")
	for _, path := range []string{
		filepath.Join(mainProfile, "prefs.js"),
		filepath.Join(mainProfile, "cookies.sqlite"),
		filepath.Join(mainProfile, "parent.lock"),
		filepath.Join(mainProfile, "cache2", "ignored"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	profile, err := profileForSnapshot("leadpier", true)
	if err != nil {
		t.Fatalf("profileForSnapshot returned error: %v", err)
	}
	for _, name := range []string{"prefs.js", "cookies.sqlite"} {
		if _, err := os.Stat(filepath.Join(profile.Root, name)); err != nil {
			t.Fatalf("seeded profile missing %s: %v", name, err)
		}
	}
	for _, path := range []string{filepath.Join(profile.Root, "parent.lock"), filepath.Join(profile.Root, "cache2", "ignored")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("volatile path %s was copied or stat failed unexpectedly: %v", path, err)
		}
	}
}

func TestRestoreConfiguredSnapshotUsesManagedProfileForNonComs(t *testing.T) {
	home := setupFirefoxHome(t)
	snapshotRoot := filepath.Join(home, "dotfiles", "cmds", "internal", "hyprd", "browser", "sessions", "leadpier")
	writeSnapshotSummary(t, snapshotRoot, browserSnapshotSummary{
		Name:   "leadpier",
		Window: browserSnapshotWindow{SelectedTab: 1, SelectedTitle: "LeadPier"},
		Tabs:   []browserTabSummary{{Position: 1, Title: "LeadPier", URL: "https://leadpier.example.com"}},
	})

	out, err := (&Browser{}).RestoreConfiguredSnapshot(config.BrowserConfig{Snapshot: "leadpier"}, true)
	if err != nil {
		t.Fatalf("RestoreConfiguredSnapshot returned error: %v", err)
	}
	wantRoot := filepath.Join(home, ".local", "share", "hyprd", "firefox-profiles", "leadpier")
	if !strings.Contains(out, wantRoot) {
		t.Fatalf("dry-run output %q does not use managed profile %q", out, wantRoot)
	}
}

func setupFirefoxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	firefoxDir := filepath.Join(home, ".mozilla", "firefox")
	defaultProfile := filepath.Join(firefoxDir, "default-release")
	if err := os.MkdirAll(defaultProfile, 0o755); err != nil {
		t.Fatalf("mkdir default profile: %v", err)
	}
	profilesINI := "[Profile0]\nName=dev-edition-default\nIsRelative=1\nPath=default-release\nDefault=1\n"
	if err := os.WriteFile(filepath.Join(firefoxDir, "profiles.ini"), []byte(profilesINI), 0o644); err != nil {
		t.Fatalf("write profiles.ini: %v", err)
	}
	return home
}

func TestBrowserModeDefaultsAndExact(t *testing.T) {
	if got, want := browserMode(config.BrowserConfig{}), "urls"; got != want {
		t.Fatalf("default mode = %q, want %q", got, want)
	}
	if got, want := browserMode(config.BrowserConfig{Snapshot: "coms"}), "exact"; got != want {
		t.Fatalf("snapshot mode = %q, want %q", got, want)
	}
	if got, want := browserForce(config.BrowserConfig{Snapshot: "coms"}), false; got != want {
		t.Fatalf("snapshot force = %t, want %t", got, want)
	}

	if got, want := browserMode(config.BrowserConfig{Mode: " exact "}), "exact"; got != want {
		t.Fatalf("trimmed mode = %q, want %q", got, want)
	}
}

func TestSelectedEntryClampsIndex(t *testing.T) {
	tab := firefoxTab{
		Entries: []firefoxEntry{
			{Title: "first"},
			{Title: "second"},
			{Title: "third"},
		},
	}

	cases := []struct {
		name  string
		index int
		want  string
	}{
		{name: "negative", index: -2, want: "first"},
		{name: "zero", index: 0, want: "first"},
		{name: "middle", index: 2, want: "second"},
		{name: "past end", index: 99, want: "third"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tab.Index = tc.index
			if got := selectedEntry(tab).Title; got != tc.want {
				t.Fatalf("selectedEntry(%d) = %q, want %q", tc.index, got, tc.want)
			}
		})
	}
}

func TestSelectedWindowTabClampsSelection(t *testing.T) {
	window := firefoxWindow{
		Tabs: []firefoxTab{
			{Entries: []firefoxEntry{{Title: "first"}}},
			{Entries: []firefoxEntry{{Title: "second"}}},
			{Entries: []firefoxEntry{{Title: "third"}}},
		},
	}

	cases := []struct {
		name     string
		selected int
		want     string
	}{
		{name: "negative", selected: -2, want: "first"},
		{name: "zero", selected: 0, want: "first"},
		{name: "middle", selected: 2, want: "second"},
		{name: "past end", selected: 99, want: "third"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			window.Selected = tc.selected
			got := selectedWindowTab(window)
			if got == nil {
				t.Fatalf("selectedWindowTab(%d) = nil", tc.selected)
			}
			if title := selectedEntry(*got).Title; title != tc.want {
				t.Fatalf("selectedWindowTab(%d) = %q, want %q", tc.selected, title, tc.want)
			}
		})
	}
}

func TestBuildSessionPayloadPreservesCollapsedGroups(t *testing.T) {
	dir := t.TempDir()
	meta := browserSnapshotSummary{
		Window: browserSnapshotWindow{SelectedTab: 1},
		Tabs: []browserTabSummary{
			{Position: 1, Title: "Pinned", URL: "https://git.example.com", Pinned: true},
			{Position: 2, Title: "Local", URL: "https://local.example.com", GroupID: "grp-local", Group: "local", GroupColor: "blue", Collapsed: true},
		},
	}
	data, err := yaml.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.yaml"), data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	payload, err := buildSessionPayload(dir)
	if err != nil {
		t.Fatalf("buildSessionPayload returned error: %v", err)
	}

	var doc struct {
		Windows []struct {
			Groups []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Color     string `json:"color"`
				Collapsed bool   `json:"collapsed"`
			} `json:"groups"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got, want := len(doc.Windows), 1; got != want {
		t.Fatalf("window count = %d, want %d", got, want)
	}
	if got, want := len(doc.Windows[0].Groups), 1; got != want {
		t.Fatalf("group count = %d, want %d", got, want)
	}
	group := doc.Windows[0].Groups[0]
	if group.ID != "grp-local" || group.Name != "local" || group.Color != "blue" || !group.Collapsed {
		t.Fatalf("group = %+v, want collapsed local group", group)
	}
}

func TestBuildCombinedSessionPayloadIncludesAllSnapshotWindows(t *testing.T) {
	first := writeTestSnapshot(t, "Coms", []browserTabSummary{
		{Position: 1, Title: "Pinned", URL: "https://chat.example.com", Pinned: true},
		{Position: 2, Title: "Slack", URL: "https://slack.example.com"},
	})
	second := writeTestSnapshot(t, "LeadPier", []browserTabSummary{
		{Position: 1, Title: "LeadPier", URL: "https://leadpier.example.com", GroupID: "grp-work", Group: "work", GroupColor: "blue", Collapsed: true},
	})

	payload, err := buildCombinedSessionPayload([]string{first, second})
	if err != nil {
		t.Fatalf("buildCombinedSessionPayload returned error: %v", err)
	}

	var doc struct {
		Windows []struct {
			Tabs []struct {
				Pinned bool `json:"pinned"`
			} `json:"tabs"`
			Groups []struct {
				Name      string `json:"name"`
				Collapsed bool   `json:"collapsed"`
			} `json:"groups"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got, want := len(doc.Windows), 2; got != want {
		t.Fatalf("window count = %d, want %d", got, want)
	}
	if !doc.Windows[0].Tabs[0].Pinned {
		t.Fatalf("first window pinned tab was not preserved")
	}
	if got := doc.Windows[1].Groups; len(got) != 1 || got[0].Name != "work" || !got[0].Collapsed {
		t.Fatalf("second window groups = %+v, want collapsed work group", got)
	}
}

func TestRestoreConfiguredSnapshotsUsesSharedDefaultProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	firefoxDir := filepath.Join(home, ".mozilla", "firefox")
	defaultProfile := filepath.Join(firefoxDir, "default-release")
	if err := os.MkdirAll(defaultProfile, 0o755); err != nil {
		t.Fatalf("mkdir default profile: %v", err)
	}
	profilesINI := "[Profile0]\nName=dev-edition-default\nIsRelative=1\nPath=default-release\nDefault=1\n"
	if err := os.WriteFile(filepath.Join(firefoxDir, "profiles.ini"), []byte(profilesINI), 0o644); err != nil {
		t.Fatalf("write profiles.ini: %v", err)
	}

	snapshotRoot := filepath.Join(home, "dotfiles", "cmds", "internal", "hyprd", "browser", "sessions", "coms")
	writeSnapshotSummary(t, snapshotRoot, browserSnapshotSummary{
		Name:   "coms",
		Window: browserSnapshotWindow{SelectedTab: 1, SelectedTitle: "Coms"},
		Tabs:   []browserTabSummary{{Position: 1, Title: "Coms", URL: "https://chat.example.com"}},
	})

	out, err := (&Browser{}).RestoreConfiguredSnapshots([]config.BrowserConfig{{Snapshot: "coms"}}, true)
	if err != nil {
		t.Fatalf("RestoreConfiguredSnapshots returned error: %v", err)
	}
	if !strings.Contains(out, defaultProfile) {
		t.Fatalf("dry-run output %q does not use shared default profile %q", out, defaultProfile)
	}
}

func writeTestSnapshot(t *testing.T, title string, tabs []browserTabSummary) string {
	t.Helper()
	dir := t.TempDir()
	writeSnapshotSummary(t, dir, browserSnapshotSummary{
		Window: browserSnapshotWindow{SelectedTab: 1, SelectedTitle: title},
		Tabs:   tabs,
	})
	return dir
}

func writeSnapshotSummary(t *testing.T, dir string, summary browserSnapshotSummary) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot: %v", err)
	}
	data, err := yaml.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.yaml"), data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

func TestSetFirefoxPrefUpserts(t *testing.T) {
	profile := firefoxProfile{Root: t.TempDir()}
	prefsPath := filepath.Join(profile.Root, "prefs.js")
	initial := "user_pref(\"browser.foo\", true);\nuser_pref(\"browser.sessionstore.resume_session_once\", false);\n"
	if err := os.WriteFile(prefsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	if err := setFirefoxPref(profile, "browser.sessionstore.resume_session_once", "true"); err != nil {
		t.Fatalf("setFirefoxPref returned error: %v", err)
	}
	if err := setFirefoxPref(profile, "browser.sessionstore.resume_session_once", "true"); err != nil {
		t.Fatalf("setFirefoxPref second call returned error: %v", err)
	}

	data, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("read prefs: %v", err)
	}
	got := string(data)
	want := "user_pref(\"browser.foo\", true);\nuser_pref(\"browser.sessionstore.resume_session_once\", true);\n"
	if got != want {
		t.Fatalf("prefs = %q, want %q", got, want)
	}
}

func TestClearSessionStoreRemovesOnlySessionFiles(t *testing.T) {
	profile := firefoxProfile{Root: t.TempDir()}
	backupsDir := filepath.Join(profile.Root, "sessionstore-backups")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		t.Fatalf("mkdir backups: %v", err)
	}
	for _, path := range []string{
		filepath.Join(profile.Root, "sessionstore.jsonlz4"),
		filepath.Join(backupsDir, "recovery.jsonlz4"),
		filepath.Join(backupsDir, "upgrade.jsonlz4-20260428"),
		filepath.Join(backupsDir, "keep.txt"),
	} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	if err := clearSessionStore(profile); err != nil {
		t.Fatalf("clearSessionStore returned error: %v", err)
	}
	for _, path := range []string{
		filepath.Join(profile.Root, "sessionstore.jsonlz4"),
		filepath.Join(backupsDir, "recovery.jsonlz4"),
		filepath.Join(backupsDir, "upgrade.jsonlz4-20260428"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists or stat failed unexpectedly: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(backupsDir, "keep.txt")); err != nil {
		t.Fatalf("keep.txt should remain: %v", err)
	}
}
