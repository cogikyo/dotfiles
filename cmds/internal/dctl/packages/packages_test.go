package packages

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/execx"
)

type fake struct {
	out   map[string]string
	calls []string
}

func (f *fake) Run(ctx context.Context, dir, name string, args ...string) (*execx.Result, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, cmd)
	return &execx.Result{Stdout: f.out[cmd]}, nil
}

func (f *fake) Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	res, err := f.Run(ctx, dir, name, args...)
	return res.Stdout, err
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

func TestParse(t *testing.T) {
	got, err := Parse(strings.NewReader("# section\nzsh\n  git  # vcs\n\nzsh\nbase-devel extra words\n"))
	if err != nil || !slices.Equal(got, []string{"base-devel", "git", "zsh"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestMissing(t *testing.T) {
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
