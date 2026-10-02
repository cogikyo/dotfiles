package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

type state struct {
	backend string
	ssh     bool
}

func Group(run execx.Runner) doctor.Group {
	return doctor.Group{Name: "tailscale", Sudo: true, Online: true, Checks: []doctor.Check{{
		Name: "tailscale-ssh",
		Check: func(ctx context.Context) error {
			s, err := read(ctx, run)
			switch {
			case err != nil:
				return err
			case s.backend != "Running":
				return fmt.Errorf("tailscale is %s, want Running", s.backend)
			case !s.ssh:
				return errors.New("Tailscale SSH is off")
			}
			return nil
		},
		Fix: func(ctx context.Context) error {
			s, err := read(ctx, run)
			args := []string{"up", "--ssh"}
			if err == nil && s.backend == "Running" {
				args = []string{"set", "--ssh=true"}
			}
			return run.Run(ctx, "", "tailscale", args...)
		},
	}}}
}

func read(ctx context.Context, run execx.Runner) (state, error) {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return state{}, doctor.Block("tailscale not found; install it from base.lst")
	}
	var status struct{ BackendState string }
	var prefs struct{ RunSSH bool }
	for _, q := range []struct {
		args []string
		into any
	}{{[]string{"status", "--json"}, &status}, {[]string{"debug", "prefs"}, &prefs}} {
		out, err := run.Output(ctx, "", "tailscale", q.args...)
		if err != nil {
			return state{}, fmt.Errorf("tailscale %v: %w; is tailscaled running?", q.args, err)
		}
		if err := json.Unmarshal([]byte(out), q.into); err != nil {
			return state{}, fmt.Errorf("parse tailscale %v: %w", q.args, err)
		}
	}
	return state{status.BackendState, prefs.RunSSH}, nil
}
