package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/home"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/system"
	"dotfiles/cmds/internal/dctl/ui"
)

type DoctorCmd struct {
	Fix     bool     `help:"Fix failed checks, then check again."`
	Offline bool     `help:"Never touch the network."`
	Groups  []string `arg:"" optional:"" help:"Groups to run (default: all)."`
}

func (c *DoctorCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	groups, err := doctor.Select([]doctor.Group{
		system.Group("/", root.System()),
		home.Group(root),
	}, c.Groups)
	if err != nil {
		return err
	}
	results, err := doctor.Run(ctx, u, groups, doctor.Options{Fix: c.Fix, Elevated: os.Geteuid() == 0, Rerun: c.rerun(root)})
	if err != nil {
		return err
	}
	unhealthy := 0
	for _, r := range results {
		if !r.Healthy() {
			unhealthy++
		}
	}
	if u.JSON() {
		if err := u.Emit(results); err != nil {
			return err
		}
	}
	if unhealthy > 0 {
		return fmt.Errorf("doctor: %d of %d checks unhealthy", unhealthy, len(results))
	}
	u.OK("doctor: %d checks healthy", len(results))
	return nil
}

func (c *DoctorCmd) rerun(root paths.Root) string {
	exe, err := os.Executable()
	if err != nil {
		exe = "dctl"
	}
	cmd := []string{exe, "doctor", "--fix"}
	if c.Offline {
		cmd = append(cmd, "--offline")
	}
	if root.Dotfiles != filepath.Join(root.Home, "dotfiles") {
		cmd = append([]string{"env", "DOTFILES=" + root.Dotfiles}, cmd...)
	}
	return strings.Join(cmd, " ")
}
