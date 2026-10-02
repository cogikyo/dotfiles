package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	Fix     bool     `help:"Repair checks with supported fixes, then recheck; some checks need manual action."`
	Offline bool     `help:"Skip online groups by default; block network-dependent repairs."`
	Groups  []string `arg:"" optional:"" help:"Groups to run (default: all)."`
}

func (c *DoctorCmd) Help() string {
	var names []string
	for _, g := range groups(nil, paths.Root{}, false) {
		names = append(names, g.Name)
	}
	return "Groups: " + strings.Join(names, ", ") + `

Examples:
  dctl doctor --offline
  dctl doctor --fix home firefox
  sudo dctl doctor --fix system`
}

func (c *DoctorCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	all := groups(u, root, c.Offline)
	selected, err := doctor.Select(all, c.Groups, c.Offline)
	if err != nil {
		return err
	}
	if c.Offline && len(c.Groups) == 0 {
		var skipped []string
		for _, g := range all {
			if g.Online {
				skipped = append(skipped, g.Name)
			}
		}
		u.Note("Skipped (--offline): %s", strings.Join(skipped, ", "))
	}
	results, err := doctor.Run(ctx, u, selected, doctor.Options{Fix: c.Fix, Elevated: os.Geteuid() == 0, Rerun: c.rerun(root)})
	if err != nil {
		return err
	}
	if u.JSON() {
		if err := u.Emit(results); err != nil {
			return err
		}
	}
	summary := doctor.Summary(results)
	if slices.ContainsFunc(results, func(r doctor.Result) bool { return !r.Healthy() }) {
		return fmt.Errorf("doctor: %s", summary)
	}
	u.OK("doctor: %s", summary)
	return nil
}

func groups(u *ui.UI, root paths.Root, offline bool) []doctor.Group {
	run := execx.OSRunner{}
	return []doctor.Group{
		system.Group("/", root.System()),
		packages.Group(root.Packages(), offline, run),
		home.Group(root, run),
		secrets.Group(u, root),
		keys.Group(run, "/sys"),
		secureboot.Group(run, "/"),
		vpn.Group(root, run),
		repos.Group(root, run),
		home.Firefox(root),
		binaries.Group(root, offline, run),
		certs.Group(root, run),
		hardware.Group("/sys", run),
		packages.Extra(root.Packages(), run),
		tailscale.Group(run),
		hardware.Firmware(run),
	}
}

func (c *DoctorCmd) rerun(root paths.Root) string {
	exe, err := os.Executable()
	if err != nil {
		exe = "dctl"
	}
	cmd := []string{exe, "doctor", "--fix"}
	if c.Offline && c.Fix {
		cmd = append(cmd, "--offline")
	}
	if root.Dotfiles != filepath.Join(root.Home, "dotfiles") {
		cmd = append([]string{"env", "DOTFILES=" + root.Dotfiles}, cmd...)
	}
	return strings.Join(cmd, " ")
}
