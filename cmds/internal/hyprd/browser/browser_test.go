package browser

// browser_test.go covers snapshot summarization and helper selection behavior for browser restore logic.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dotfiles/cmds/internal/config"

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

func TestManagedProfileForSessionClonesSnapshotSource(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	source := filepath.Join(t.TempDir(), "source.default")
	if err := os.MkdirAll(filepath.Join(source, "sessionstore-backups"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	for path, data := range map[string]string{
		filepath.Join(source, "prefs.js"):                                 "user_pref(\"browser.foo\", true);\n",
		filepath.Join(source, "sessionstore.jsonlz4"):                     "stale session",
		filepath.Join(source, ".parentlock"):                              "lock",
		filepath.Join(source, "sessionstore-backups", "recovery.jsonlz4"): "stale recovery",
	} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	snapshotDir := filepath.Join(stateHome, "hyprd", "browser-sessions", "managed-test")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot: %v", err)
	}
	summary := browserSnapshotSummary{
		Name:    "managed-test",
		Profile: browserProfileSummary{Name: "default", Path: source},
		Window:  browserSnapshotWindow{SelectedTab: 1},
	}
	data, err := yaml.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "snapshot.yaml"), data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	profile, err := ManagedProfileForSession("LeadPier", config.BrowserConfig{Snapshot: "managed-test"})
	if err != nil {
		t.Fatalf("ManagedProfileForSession returned error: %v", err)
	}
	if got, want := profile.Root, filepath.Join(stateHome, "hyprd", "firefox-profiles", "LeadPier"); got != want {
		t.Fatalf("profile root = %q, want %q", got, want)
	}
	if !fileExists(filepath.Join(profile.Root, "prefs.js")) {
		t.Fatalf("prefs.js was not copied")
	}
	for _, path := range []string{
		filepath.Join(profile.Root, "sessionstore.jsonlz4"),
		filepath.Join(profile.Root, ".parentlock"),
		filepath.Join(profile.Root, "sessionstore-backups"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s should not have been cloned", path)
		}
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

func TestDiscoverFirefoxProfileAcceptsAbsoluteDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	firefoxDir := filepath.Join(root, ".mozilla", "firefox")
	profileDir := filepath.Join(root, ".local", "state", "hyprd", "firefox-profiles", "leadpier")
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

func TestRestoreProfileForSnapshotFallsBackToDefaultWhenRecordedProfileIsMissing(t *testing.T) {
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

	snapshotDir := t.TempDir()
	summary := browserSnapshotSummary{
		Profile: browserProfileSummary{Name: "hyprd-leadpier", Path: filepath.Join(home, ".local", "state", "hyprd", "firefox-profiles", "leadpier")},
	}
	data, err := yaml.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "snapshot.yaml"), data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	profile, err := restoreProfileForSnapshot(snapshotDir, "")
	if err != nil {
		t.Fatalf("restoreProfileForSnapshot returned error: %v", err)
	}
	if profile.Root != defaultProfile {
		t.Fatalf("profile root = %q, want %q", profile.Root, defaultProfile)
	}
}

func TestSnapshotProfileFallsBackToNamedManagedProfile(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	profileDir := filepath.Join(stateHome, "hyprd", "firefox-profiles", "leadpier")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir managed profile: %v", err)
	}

	profile, err := (&Browser{}).snapshotProfile("", "leadpier", "active")
	if err != nil {
		t.Fatalf("snapshotProfile returned error: %v", err)
	}
	if profile.Root != profileDir {
		t.Fatalf("profile root = %q, want %q", profile.Root, profileDir)
	}
}

func TestSnapshotProfileErrorsWhenNoTargetProfile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	_, err := (&Browser{}).snapshotProfile("", "missing", "active")
	if err == nil {
		t.Fatalf("snapshotProfile returned nil error")
	}
}

func TestBrowserModeDefaultsAndExact(t *testing.T) {
	if got, want := browserMode(config.BrowserConfig{}), "urls"; got != want {
		t.Fatalf("default mode = %q, want %q", got, want)
	}
	if got, want := browserMode(config.BrowserConfig{Snapshot: "coms"}), "exact"; got != want {
		t.Fatalf("snapshot mode = %q, want %q", got, want)
	}
	if got, want := browserForce(config.BrowserConfig{Snapshot: "coms"}), true; got != want {
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
