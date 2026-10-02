package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/certs"
	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/hardware"
	"dotfiles/cmds/internal/dctl/home"
	"dotfiles/cmds/internal/dctl/keys"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/secureboot"
	"dotfiles/cmds/internal/dctl/system"
	"dotfiles/cmds/internal/dctl/tailscale"
	"dotfiles/cmds/internal/dctl/ui"
	"dotfiles/cmds/internal/dctl/vpn"
)

type DoctorCmd struct {
	Fix     bool     `help:"Fix failed checks, then check again."`
	Offline bool     `help:"Never touch the network."`
	Groups  []string `arg:"" optional:"" help:"Groups to run (default: all)."`
}

func (c *DoctorCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	run := execx.OSRunner{}
	groups, err := doctor.Select([]doctor.Group{
		system.Group("/", root.System()),
		packages.Group(root.Packages(), c.Offline, run),
		home.Group(root, run),
		secrets.Group(u, root),
		keys.Group(run, "/sys"),
		secureboot.Group(run, "/"),
		vpn.Group(root, run),
		repos.Group(root, run),
		home.Firefox(root),
		binaries.Group(root, c.Offline, run),
		certs.Group(root, run),
		hardware.Group("/sys", run),
		packages.Extra(root.Packages(), run),
		tailscale.Group(run),
		hardware.Firmware(run),
	}, c.Groups, c.Offline)
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
