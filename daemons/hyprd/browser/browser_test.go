package browser

import (
	"testing"

	"dotfiles/daemons/config"
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

func TestBrowserModeDefaultsAndExact(t *testing.T) {
	if got, want := browserMode(config.BrowserConfig{}), "urls"; got != want {
		t.Fatalf("default mode = %q, want %q", got, want)
	}

	if got, want := browserMode(config.BrowserConfig{Mode: " exact "}), "exact"; got != want {
		t.Fatalf("trimmed mode = %q, want %q", got, want)
	}
}
