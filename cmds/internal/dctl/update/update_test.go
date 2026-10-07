package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

type fake struct {
	t       *testing.T
	builds  map[string]string
	dirty   string
	rebuild string
	calls   []string
}

func (f *fake) Run(_ context.Context, _ string, name string, args ...string) error {
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	return nil
}

func (f *fake) Output(_ context.Context, _ string, name string, args ...string) (string, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, line)
	if name == "env" && slices.Contains(args, "build") {
		out := args[slices.Index(args, "-o")+1]
		for bin, data := range f.builds {
			if err := os.WriteFile(filepath.Join(out, bin), []byte(data), 0o755); err != nil {
				f.t.Fatal(err)
			}
		}
		return "", nil
	}
	switch line {
	case "git status --porcelain --untracked-files=all -- cmds":
		return f.dirty, nil
	case "hyprd rebuild":
		if strings.HasPrefix(f.rebuild, "error:") {
			return f.rebuild, errors.New("exit status 1")
		}
		return f.rebuild, nil
	}
	if line == "ewwd status" || line == "ewwd open" || strings.HasPrefix(line, "systemctl --user restart ") {
		return "", nil
	}
	return "", errors.New("unexpected " + line)
}

func quiet() *ui.UI { return ui.New(ui.Options{JSON: true}) }

func TestSteps(t *testing.T) {
	steps := Steps(quiet(), paths.Root{}, &fake{t: t}, false, nil)
	var names []string
	for _, s := range steps {
		names = append(names, s.Name)
	}
	if want := []string{"pacman", "aur", "packages", "repos", "cmd", "go", "rust"}; !slices.Equal(names, want) {
		t.Fatalf("steps %v, want %v", names, want)
	}
	got, err := Select(steps, []string{"rust", "aur"})
	if err != nil || len(got) != 2 || got[0].Name != "aur" || got[1].Name != "rust" {
		t.Fatalf("select %v %v", got, err)
	}
	if _, err := Select(steps, []string{"npm"}); err == nil {
		t.Error("unknown step accepted")
	}
}

func TestNoconfirm(t *testing.T) {
	root := paths.Root{Dotfiles: t.TempDir()}
	for _, rel := range []string{"base.lst", "aur.lst", "extra.lst", "eww/PKGBUILD", "yubikey-tools/PKGBUILD"} {
		path := root.Packages(rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, all := range []bool{false, true} {
		f := &fake{t: t}
		steps, _ := Select(Steps(quiet(), root, f, all, nil), []string{"aur"})
		if err := steps[0].Run(t.Context()); err != nil {
			t.Fatal(err)
		}
		if want := map[bool]string{false: "yay -Sua --ignore eww,yubikey-tools", true: "yay -Sua --ignore eww,yubikey-tools --noconfirm"}[all]; f.calls[0] != want {
			t.Errorf("all=%v: %q, want %q", all, f.calls[0], want)
		}
	}
}

func TestRun(t *testing.T) {
	var ran []string
	step := func(name string, err error) Step {
		return Step{Name: name, Run: func(context.Context) error { ran = append(ran, name); return err }}
	}
	steps := []Step{step("a", errors.New("boom")), step("b", nil)}
	err := Run(t.Context(), ui.New(ui.Options{JSON: true, Yes: true}), steps, true)
	if err == nil || !strings.Contains(err.Error(), "a") || strings.Contains(err.Error(), "b") || !slices.Equal(ran, []string{"a", "b"}) {
		t.Fatalf("err %v, ran %v", err, ran)
	}
	ran = nil
	if err := Run(t.Context(), quiet(), steps, true); !errors.Is(err, ui.ErrNoTTY) || len(ran) != 0 {
		t.Fatalf("prompt without a terminal: %v, ran %v", err, ran)
	}
	if err := Run(t.Context(), quiet(), steps[1:], false); err != nil || !slices.Equal(ran, []string{"b"}) {
		t.Fatalf("named run: %v, ran %v", err, ran)
	}
}

func TestCommands(t *testing.T) {
	root := paths.Root{Home: t.TempDir(), Dotfiles: t.TempDir()}
	bin := filepath.Join(root.Home, ".local", "bin")
	f := &fake{t: t, builds: map[string]string{}, rebuild: "rebuilt: restarting..."}
	for _, name := range binaries.Names {
		f.builds[name] = name + " v1"
	}
	if err := commands(t.Context(), quiet(), root, f, false, binaries.Names); err != nil {
		t.Fatal(err)
	}
	for _, name := range binaries.Names {
		data, err := os.ReadFile(filepath.Join(bin, name))
		switch {
		case name == "hyprd" && err == nil:
			t.Error("dctl placed hyprd itself")
		case name != "hyprd" && string(data) != name+" v1":
			t.Errorf("%s = %q, %v", name, data, err)
		}
	}
	for _, want := range []string{"hyprd rebuild", "systemctl --user restart ewwd.service", "ewwd open", "systemctl --user restart newtab.service"} {
		if !slices.Contains(f.calls, want) {
			t.Errorf("calls %q lack %q", f.calls, want)
		}
	}
	if !slices.ContainsFunc(f.calls, func(c string) bool {
		return strings.HasPrefix(c, "env CGO_ENABLED=0 GOFLAGS= GOWORK=off go build -trimpath -buildvcs=false -o ")
	}) {
		t.Errorf("calls %q lack the shared build flags", f.calls)
	}

	if err := os.WriteFile(filepath.Join(bin, "hyprd"), []byte("hyprd v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if err := commands(t.Context(), quiet(), root, f, false, binaries.Names); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, "systemctl") || c == "hyprd rebuild" }) {
		t.Errorf("second run calls %q, want no rebuild or restart", f.calls)
	}
	if entries, _ := os.ReadDir(bin); len(entries) != len(binaries.Names) {
		t.Errorf("bin holds %d entries, want only the %d commands", len(entries), len(binaries.Names))
	}

	f.builds["hyprd"] = "hyprd v2"
	f.rebuild = "error: full lock active; unlock before rebuilding"
	if err := commands(t.Context(), quiet(), root, f, false, binaries.Names); err != nil {
		t.Errorf("hyprd refusal = %v, want a reported skip", err)
	}
	f.rebuild = "error: build failed: exit status 1"
	if err := commands(t.Context(), quiet(), root, f, false, binaries.Names); err == nil {
		t.Error("hyprd build failure reported as success")
	}

	f.dirty, f.calls = " M cmds/x.go", nil
	if err := commands(t.Context(), quiet(), root, f, true, binaries.Names); err != nil || slices.ContainsFunc(f.calls, func(c string) bool { return strings.Contains(c, "go build") }) {
		t.Errorf("dirty cmds under --all: %v, calls %q", err, f.calls)
	}
	if err := commands(t.Context(), quiet(), root, f, false, binaries.Names); !errors.Is(err, ui.ErrNoTTY) {
		t.Errorf("dirty cmds without --all = %v, want a prompt", err)
	}
}
