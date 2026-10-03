package packages

import (
	"context"
	"fmt"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

func Extra(dir string, run execx.Runner) setup.Stage {
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
	return setup.Stage{Name: "extra", Items: []setup.Item{
		{
			Name: "extra-installed",
			Check: func(ctx context.Context) error {
				names, err := pending(ctx)
				if err != nil || len(names) == 0 {
					return err
				}
				return fmt.Errorf("missing %d: %s", len(names), strings.Join(names, " "))
			},
			Fix: func(ctx context.Context) error {
				names, err := pending(ctx)
				if err != nil || len(names) == 0 {
					return err
				}
				return install(ctx, run, names)
			},
		},
		{
			Name: "extra-docker",
			Check: func(ctx context.Context) error {
				state, err := run.Output(ctx, "", "systemctl", "is-enabled", "docker.socket")
				switch state {
				case "enabled":
					return nil
				case "not-found":
					return setup.Manual("docker.socket not found; install extra.lst first")
				case "":
					return fmt.Errorf("systemctl is-enabled docker.socket: %w", err)
				}
				return fmt.Errorf("docker.socket is %s", state)
			},
			Fix: func(ctx context.Context) error {
				return run.Run(ctx, "", "sudo", "systemctl", "enable", "docker.socket")
			},
		},
	}}
}

func install(ctx context.Context, run execx.Runner, names []string) error {
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
		args := append([]string{"-S", "--needed", "--noconfirm", step.flag}, step.names...)
		if err := run.Run(ctx, "", "yay", args...); err != nil {
			return err
		}
	}
	return nil
}
