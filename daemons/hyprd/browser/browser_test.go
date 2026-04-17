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
