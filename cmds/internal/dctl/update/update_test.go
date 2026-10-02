package update

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestClassify(t *testing.T) {
	listed := map[string]bool{"base": true, "git": true, "yay": true, "eww": true, "docker": true, "missing-base": true}
	installed := map[string]bool{"base": true, "git": true, "yay": true, "eww": true, "htop": true, "slack-desktop": true, "glibc": true, "libfoo": true}
	d := classify(listed, installed,
		[]string{"base", "git", "htop"},
		[]string{"eww", "slack-desktop", "yay"},
		[]string{"git", "libfoo"},
	)
	for _, c := range []struct {
		name      string
		got, want []string
	}{
		{"repo", d.Repo, []string{"htop"}},
		{"aur", d.AUR, []string{"slack-desktop"}},
		{"missing", d.Missing, []string{"docker", "missing-base"}},
		{"orphans", d.Orphans, []string{"libfoo"}},
	} {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestListsIncludeExtra(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"base.lst": "base\n", "aur.lst": "", "extra.lst": "docker # online\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := lists(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !listed["docker"] {
		t.Errorf("extra.lst names missing from listed set: %v", listed)
	}
}
