package packages

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

func Extra(dir, opencode string, run execx.Runner) setup.Stage {
	pending := func(ctx context.Context) ([]string, error) {
		l, err := Load(dir)
		if err != nil {
			return nil, err
		}
		have, err := installed(ctx, run)
		if err != nil {
			return nil, err
		}
		return absent(l.Extra, have), nil
	}
	return setup.Stage{Name: "extra", Background: true, Items: []setup.Item{
		{
			Name: "extra-installed",
			Check: func(ctx context.Context) error {
				names, err := pending(ctx)
				if err != nil || len(names) == 0 {
					return err
				}
				return fmt.Errorf("%d packages missing: %s", len(names), strings.Join(names, " "))
			},
			Fix: func(ctx context.Context) error {
				names, err := pending(ctx)
				if err != nil || len(names) == 0 {
					return err
				}
				if err := run.Run(ctx, "", "sudo", slices.Concat([]string{"-n", "pacman"}, Upgrade(true))...); err != nil {
					return err
				}
				return install(ctx, run, names, "--sudoflags=-n")
			},
		},
		{
			Name: "extra-docker",
			Check: func(ctx context.Context) error {
				name, err := dockerContext(ctx, run)
				if err != nil {
					return err
				}
				if name != "rootless" {
					if err := rootfulData(); err != nil {
						return err
					}
					return fmt.Errorf("docker context is %s, not rootless", name)
				}
				return rootlessAnswers(ctx, run)
			},
			Fix: func(ctx context.Context) error {
				name, err := dockerContext(ctx, run)
				if err != nil {
					return err
				}
				if name != "rootless" {
					if err := rootfulData(); err != nil {
						return err
					}
				}
				if rootlessAnswers(ctx, run) != nil {
					if err := run.Run(ctx, "", "systemctl", "--user", "start", "docker.service"); err != nil {
						return err
					}
					if err := rootlessAnswers(ctx, run); err != nil {
						return err
					}
				}
				if name == "rootless" {
					return nil
				}
				if _, err := run.Output(ctx, "", "docker", "context", "inspect", "rootless"); err != nil {
					if err := run.Run(ctx, "", "docker", "context", "create", "rootless", "--docker", "host="+rootlessHost()); err != nil {
						return err
					}
				}
				return run.Run(ctx, "", "docker", "context", "use", "rootless")
			},
		},
		{
			Name:  "extra-bun",
			Check: func(context.Context) error { return bunLocked(opencode) },
			Fix:   func(ctx context.Context) error { return Bun(ctx, run, opencode) },
		},
		androidItem(run),
	}}
}

func dockerContext(ctx context.Context, run execx.Runner) (string, error) {
	name, err := run.Output(ctx, "", "docker", "context", "show")
	if err != nil {
		return "", fmt.Errorf("docker context show: %w", err)
	}
	return name, nil
}

func rootlessHost() string {
	dir := cmp.Or(os.Getenv("XDG_RUNTIME_DIR"), fmt.Sprintf("/run/user/%d", os.Getuid()))
	return "unix://" + filepath.Join(dir, "docker.sock")
}

func rootlessAnswers(ctx context.Context, run execx.Runner) error {
	host := rootlessHost()
	sec, err := run.Output(ctx, "", "docker", "-H", host, "info", "-f", "{{json .SecurityOptions}}")
	if err != nil {
		return fmt.Errorf("no docker daemon answers at %s: %w", host, err)
	}
	if !strings.Contains(sec, "name=rootless") {
		return fmt.Errorf("the docker daemon at %s is not rootless", host)
	}
	return nil
}

var rootful = []string{"/var/lib/docker", "/var/lib/containerd"}

func rootfulData() error {
	for _, dir := range rootful {
		if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
			return setup.Manual("rootful Docker holds data in %s; migrate first", dir)
		}
	}
	return nil
}

func install(ctx context.Context, run execx.Runner, names []string, sudo ...string) error {
	if err := synced(); err != nil {
		return err
	}
	repo, err := run.Output(ctx, "", "pacman", "-Slq")
	if err != nil {
		return fmt.Errorf("pacman -Slq: %w", err)
	}
	official := set(strings.FieldsSeq(repo))
	var fromRepo, fromAUR []string
	for _, name := range names {
		if official[name] {
			fromRepo = append(fromRepo, name)
		} else {
			fromAUR = append(fromAUR, name)
		}
	}
	for _, step := range []struct {
		flag  string
		names []string
	}{{"--repo", fromRepo}, {"--aur", fromAUR}} {
		if len(step.names) == 0 {
			continue
		}
		args := slices.Concat([]string{"-S", "--needed", "--noconfirm"}, sudo, []string{"--answerclean", "None", "--answerdiff", "None", "--answeredit", "None", step.flag}, step.names)
		if err := run.Run(ctx, "", "yay", args...); err != nil {
			return err
		}
	}
	return nil
}
