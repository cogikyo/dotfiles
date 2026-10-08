package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

type state struct {
	backend string
	ssh     bool
}

func Stage(run execx.Runner) setup.Stage {
	return setup.Stage{Name: "tailscale", Root: true, Items: []setup.Item{{
		Name: "tailscale-ssh",
		Check: func(ctx context.Context) error {
			s, err := read(ctx, run)
			switch {
			case err != nil:
				return err
			case s.backend == "NeedsLogin":
				return errors.New("Tailscale login required; run dctl setup tailscale")
			case s.backend != "Running":
				return fmt.Errorf("tailscale is %s, want Running", s.backend)
			case !s.ssh:
				return errors.New("Tailscale SSH is off")
			}
			return nil
		},
		Fix: func(ctx context.Context) error {
			s, err := read(ctx, run)
			args := []string{"up", "--ssh", "--qr"}
			if err == nil && s.backend == "Running" {
				args = []string{"set", "--ssh=true"}
			}
			if args[0] == "up" {
				return execx.Interactive(execx.Reason(run, "scan the QR code with a phone that's logged in to Tailscale")).Run(ctx, "", "tailscale", args...)
			}
			return run.Run(ctx, "", "tailscale", args...)
		},
	}}}
}

func read(ctx context.Context, run execx.Runner) (state, error) {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return state{}, setup.Manual("tailscale not found; run dctl setup packages")
	}
	var status struct{ BackendState string }
	var prefs struct{ RunSSH bool }
	for _, q := range []struct {
		args []string
		into any
	}{{[]string{"status", "--json"}, &status}, {[]string{"debug", "prefs"}, &prefs}} {
		out, err := run.Output(ctx, "", "tailscale", q.args...)
		if err != nil {
			return state{}, fmt.Errorf("tailscale %v: %w; check systemctl status tailscaled", q.args, err)
		}
		if err := json.Unmarshal([]byte(out), q.into); err != nil {
			return state{}, fmt.Errorf("parse tailscale %v: %w", q.args, err)
		}
	}
	return state{status.BackendState, prefs.RunSSH}, nil
}
