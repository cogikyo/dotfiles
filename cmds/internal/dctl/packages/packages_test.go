package packages

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type fake struct {
	out   map[string]string
	calls []string
}

func (f *fake) Run(ctx context.Context, dir, name string, args ...string) error {
	_, err := f.Output(ctx, dir, name, args...)
	return err
}

func (f *fake) Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, cmd)
	return f.out[cmd], nil
}

func lists(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, data := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func useSync(t *testing.T, present bool) {
	t.Helper()
	old := coreDB
	coreDB = filepath.Join(t.TempDir(), "core.db")
	t.Cleanup(func() { coreDB = old })
	if present {
		if err := os.WriteFile(coreDB, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParse(t *testing.T) {
	got, err := Parse(strings.NewReader("# section\nzsh\n  git  # vcs\n\nzsh\n"))
	if err != nil || !slices.Equal(got, []string{"git", "zsh"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := Parse(strings.NewReader("zsh\nbase-devel extra words\n")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err %v, want line 2 rejected", err)
	}
}

func TestMissing(t *testing.T) {
	useSync(t, true)
	dir := lists(t, map[string]string{
		"base.lst":         "git\nzsh\nmesa\n",
		"aur.lst":          "yay\nlimine-snapper-sync\n",
		"eww/PKGBUILD":     "pkgname=eww\npkgver=1\n",
		"notes/README.txt": "not a package",
	})
	run := &fake{out: map[string]string{"pacman -Qq": "git\nyay\neww\nunrelated\n"}}
	official, other, err := missing(context.Background(), dir, run)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(official, []string{"mesa", "zsh"}) || !slices.Equal(other, []string{"limine-snapper-sync"}) {
		t.Fatalf("official %v, other %v", official, other)
	}
	if err := Group(dir, false, run).Checks[0].Fix(context.Background()); err == nil || !strings.Contains(err.Error(), "limine-snapper-sync") {
		t.Fatalf("fix err %v, want blocked on limine-snapper-sync", err)
	}
	if last := run.calls[len(run.calls)-1]; last != "pacman -S --needed --noconfirm mesa zsh" {
		t.Fatalf("fix ran %q", last)
	}
}

func TestExtraInstallsOfficialBeforeAUR(t *testing.T) {
	useSync(t, true)
	dir := lists(t, map[string]string{"extra.lst": "docker\nlazydocker\nbase-devel\nspotify\nhtop\n"})
	run := &fake{out: map[string]string{
		"pacman -Qq":  "htop\n",
		"pacman -Slq": "base-devel\ndocker\nhtop\n",
	}}
	if err := Extra(dir, run).Checks[0].Fix(context.Background()); err != nil {
		t.Fatal(err)
	}
	var installs []string
	for _, call := range run.calls {
		if strings.HasPrefix(call, "yay ") {
			installs = append(installs, call)
		}
	}
	want := []string{
		"yay -S --needed --noconfirm --repo base-devel docker",
		"yay -S --needed --noconfirm --aur lazydocker spotify",
	}
	if !slices.Equal(installs, want) {
		t.Fatalf("installs %q, want %q", installs, want)
	}
}

func TestOfflineFix(t *testing.T) {
	run := &fake{}
	if err := Group(t.TempDir(), true, run).Checks[0].Fix(context.Background()); err == nil || len(run.calls) != 0 {
		t.Fatalf("offline fix err %v, calls %v", err, run.calls)
	}
}

func TestUnsynced(t *testing.T) {
	useSync(t, false)
	dir := lists(t, map[string]string{"base.lst": "zsh\n", "aur.lst": "", "extra.lst": "htop\n"})
	run := &fake{}
	for _, fix := range []func(context.Context) error{Group(dir, false, run).Checks[0].Fix, Extra(dir, run).Checks[0].Fix} {
		if err := fix(context.Background()); err == nil || !strings.Contains(err.Error(), "dctl update") {
			t.Fatalf("fix err %v, want blocked on dctl update", err)
		}
	}
	if !slices.Equal(run.calls, []string{"pacman -Qq", "pacman -Qq"}) {
		t.Fatalf("calls %q, want only pacman -Qq", run.calls)
	}
}
