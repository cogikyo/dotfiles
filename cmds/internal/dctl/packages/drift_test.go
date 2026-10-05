package packages

import (
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
		[]string{"git", "glibc", "libfoo"},
		[]string{"htop"},
	)
	for _, c := range []struct {
		name      string
		got, want []string
	}{
		{"repo", d.Repo, []string{"htop"}},
		{"aur", d.AUR, []string{"slack-desktop"}},
		{"missing", d.Missing, []string{"docker", "missing-base"}},
		{"orphans", d.Orphans, []string{"libfoo"}},
		{"implicit", d.Implicit, []string{"git"}},
		{"required", d.Required, []string{"slack-desktop"}},
	} {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestDriftListsExtra(t *testing.T) {
	dir := lists(t, map[string]string{"base.lst": "base\n", "aur.lst": "", "extra.lst": "docker # online\n"})
	l, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	run := &fake{out: map[string]string{"pacman -Qq": "base\n", "pacman -Qqen": "base\ndocker\n"}}
	d, err := drift(t.Context(), run, l)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Repo) != 0 || !slices.Equal(d.Missing, []string{"docker"}) {
		t.Errorf("extra.lst names missing from the listed set: %+v", d)
	}
}
