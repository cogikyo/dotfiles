package packages

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

func Extra(dir string, run execx.Runner) doctor.Group {
	pending := func(ctx context.Context) ([]string, error) {
		names, err := Read(filepath.Join(dir, "extra.lst"))
		if err != nil {
			return nil, err
		}
		have, err := installed(ctx, run)
		if err != nil {
			return nil, err
		}
		return absent(names, have), nil
	}
	return doctor.Group{Name: "extra", Online: true, Checks: []doctor.Check{
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
					return doctor.Block("docker.socket not found; install extra.lst first")
				case "":
					return doctor.Block("systemctl is-enabled docker.socket: %v", err)
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
		return doctor.Block("pacman -Slq: %v", err)
	}
	official := map[string]bool{}
	for name := range strings.FieldsSeq(repo) {
		official[name] = true
	}
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
