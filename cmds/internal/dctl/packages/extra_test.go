package packages

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/setup"
)

const (
	dockerInfo  = "docker -H unix:///run/user/1000/docker.sock info -f {{json .SecurityOptions}}"
	dockerStart = "systemctl --user start docker.service"
	rootlessSec = `["name=seccomp,profile=builtin","name=rootless","name=cgroupns"]`
)

type daemon struct{ *fake }

func (d daemon) Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	out, err := d.fake.Output(ctx, dir, name, args...)
	if d.calls[len(d.calls)-1] == dockerStart {
		d.out[dockerInfo] = rootlessSec
	}
	return out, err
}

func (d daemon) Run(ctx context.Context, dir, name string, args ...string) error {
	_, err := d.Output(ctx, dir, name, args...)
	return err
}

func dockerItem(t *testing.T, context string, rootfulDirs ...string) (setup.Item, *fake) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	old := rootful
	rootful = append([]string{filepath.Join(t.TempDir(), "missing")}, rootfulDirs...)
	t.Cleanup(func() { rootful = old })
	run := &fake{out: map[string]string{"docker context show": context}}
	for _, it := range Extra(t.TempDir(), t.TempDir(), daemon{run}).Items {
		if it.Name == "extra-docker" {
			return it, run
		}
	}
	t.Fatal("no extra-docker item")
	return setup.Item{}, nil
}

func switched(calls []string) bool {
	return slices.ContainsFunc(calls, func(c string) bool {
		return strings.HasPrefix(c, "docker context use") || strings.HasPrefix(c, "docker context create")
	})
}

func TestExtraDockerKeepsContextWhileRootfulDataExists(t *testing.T) {
	it, run := dockerItem(t, "default", t.TempDir())

	for name, step := range map[string]func(context.Context) error{"check": it.Check, "fix": it.Fix} {
		err := step(context.Background())
		if err == nil || !strings.Contains(err.Error(), "migrate first") {
			t.Fatalf("%s: got %v, want a migrate-first error", name, err)
		}
	}
	if switched(run.calls) || slices.Contains(run.calls, dockerStart) {
		t.Fatalf("touched the daemon or context: %q", run.calls)
	}
}

func TestExtraDockerStartsDaemonAndSwitchesWithoutRootfulData(t *testing.T) {
	it, run := dockerItem(t, "default")

	if err := it.Check(context.Background()); err == nil || strings.Contains(err.Error(), "migrate first") {
		t.Fatalf("check: got %v, want a pending context error", err)
	}
	if err := it.Fix(context.Background()); err != nil {
		t.Fatal(err)
	}
	start, use := slices.Index(run.calls, dockerStart), slices.Index(run.calls, "docker context use rootless")
	if start < 0 || use < start {
		t.Fatalf("calls %q: want docker.service started before docker context use rootless", run.calls)
	}
}

func TestExtraDockerRootlessContextNeedsAnsweringDaemon(t *testing.T) {
	it, run := dockerItem(t, "rootless", t.TempDir())

	if err := it.Check(context.Background()); err == nil {
		t.Fatal("check passed while the rootless daemon does not answer")
	}
	if err := it.Fix(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(run.calls, dockerStart) || switched(run.calls) {
		t.Fatalf("calls %q: want docker.service started and the context left alone", run.calls)
	}
	if err := it.Check(context.Background()); err != nil {
		t.Fatalf("check after fix: %v", err)
	}
}

func TestExtraDockerDoneWhenRootlessDaemonAnswers(t *testing.T) {
	it, run := dockerItem(t, "rootless", t.TempDir())
	run.out[dockerInfo] = rootlessSec

	for name, step := range map[string]func(context.Context) error{"check": it.Check, "fix": it.Fix} {
		if err := step(context.Background()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if slices.Contains(run.calls, dockerStart) || switched(run.calls) {
		t.Fatalf("calls %q: want only reads", run.calls)
	}
}
