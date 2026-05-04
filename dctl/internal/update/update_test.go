package update

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dotfiles/dctl/internal/execx"
	"dotfiles/dctl/internal/output"
	"dotfiles/dctl/internal/paths"
)

type call struct {
	Name string
	Args []string
}

type fakeRunner struct {
	installed map[string]bool
	outputs   map[string]string
	errors    map[string]error
	calls     []call
}

func (r *fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) (*execx.Result, error) {
	_ = ctx
	_ = dir
	r.calls = append(r.calls, call{Name: name, Args: append([]string{}, args...)})
	if name == "pacman" && len(args) == 2 && args[0] == "-Qq" {
		if r.installed[args[1]] {
			return &execx.Result{Stdout: args[1]}, nil
		}
		return &execx.Result{ExitCode: 1}, errors.New("not installed")
	}
	key := commandKey(name, args...)
	if err := r.errors[key]; err != nil {
		return &execx.Result{ExitCode: 1}, err
	}
	return &execx.Result{Stdout: r.outputs[key]}, nil
}

func (r *fakeRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	res, err := r.Run(ctx, dir, name, args...)
	return res.Stdout, err
}

func commandKey(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func TestParsePackageListStripsCommentsBlanksSortsAndDedupes(t *testing.T) {
	got := parsePackageList(`
# full line comment
 vim  # editor

git
vim
  base-devel
`)
	want := []string{"base-devel", "git", "vim"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePackageList() = %#v, want %#v", got, want)
	}
}

func TestRunDryRunUsesIgnoreAndKeepsOptionalOrphans(t *testing.T) {
	root := testRoot(t)
	writeFile(t, root.Etc("pacman.d", "ignore.conf"), "linux # pinned\n\n")
	writeFile(t, root.Etc("packages-optional.lst"), "optional-orphan\n")
	runner := &fakeRunner{outputs: map[string]string{
		"yay -Qdtq": "dead-lib\noptional-orphan\n",
		"yay -Qenq": "vim\noptional-orphan\n",
		"yay -Qemq": "aur-one\n",
	}}
	out, stdout := testOutput()

	if err := Run(context.Background(), root, out, runner, Options{DryRun: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !hasCall(runner.calls, "yay", "--version") {
		t.Fatalf("missing yay availability check; calls=%#v", runner.calls)
	}
	if hasCall(runner.calls, "yay", "-Rns", "dead-lib") {
		t.Fatalf("dry-run removed orphan; calls=%#v", runner.calls)
	}
	text := stdout.String()
	for _, want := range []string{"yay --ignore linux -Syu", "Keeping orphan optional-orphan", "- dead-lib", "Repo (explicit): 1 packages"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestInstallBuildsRepoAndAURCommandsWithIgnoreFiltering(t *testing.T) {
	root := testRoot(t)
	writeFile(t, root.Etc("packages.lst"), "vim\nlinux # already installed and ignored\n")
	writeFile(t, root.Etc("packages-aur.lst"), "aur-one\n")
	writeFile(t, root.Etc("pacman.d", "ignore.conf"), "linux\n")
	pacmanConf := filepath.Join(t.TempDir(), "pacman.conf")
	writeFile(t, pacmanConf, "[core]\n")
	runner := &fakeRunner{
		installed: map[string]bool{"linux": true},
		outputs:   map[string]string{"pacman -Qq": "linux\n"},
		errors:    map[string]error{"rustup --version": errors.New("no rustup")},
	}
	out, _ := testOutput()

	if err := Install(context.Background(), root, out, runner, Options{NonInteractive: true, PacmanConf: pacmanConf}); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if !hasCall(runner.calls, "sudo", "pacman", "-Syu", "--needed", "--noconfirm", "--ignore", "linux", "vim") {
		t.Fatalf("missing filtered repo install command; calls=%#v", runner.calls)
	}
	if !hasCall(runner.calls, "yay", "-S", "--needed", "--noconfirm", "--ignore", "linux", "aur-one") {
		t.Fatalf("missing AUR install command; calls=%#v", runner.calls)
	}
	if hasCall(runner.calls, "sudo", "pacman", "-Syu", "--needed", "--noconfirm", "--ignore", "linux", "linux", "vim") {
		t.Fatalf("ignored installed package was not filtered; calls=%#v", runner.calls)
	}
}

func TestCheckReportsInstalledReplacementCandidates(t *testing.T) {
	root := testRoot(t)
	runner := &fakeRunner{installed: map[string]bool{"dunst-git": true}}
	out, stdout := testOutput()

	if err := Check(context.Background(), root, out, runner); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	text := stdout.String()
	if !strings.Contains(text, "dunst-git -> dunst") {
		t.Fatalf("output missing replacement warning:\n%s", text)
	}
	if !hasCall(runner.calls, "yay", "--version") {
		t.Fatalf("missing yay availability check; calls=%#v", runner.calls)
	}
}

func testRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	return paths.Root{Dotfiles: dir, Home: dir, State: filepath.Join(dir, "state")}
}

func testOutput() (*output.Printer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return output.New(stdout, stderr, output.Options{Plain: true}), stdout
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasCall(calls []call, name string, args ...string) bool {
	for _, call := range calls {
		if call.Name == name && reflect.DeepEqual(call.Args, args) {
			return true
		}
	}
	return false
}
