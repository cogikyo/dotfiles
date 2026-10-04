package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/update"
	"dotfiles/cmds/internal/ui"
)

type UpdateCmd struct {
	All   bool     `help:"Run every step without asking and pass --noconfirm to pacman, yay, and makepkg; never flashes firmware."`
	Only  []string `placeholder:"NAME" help:"Limit the cmd step to these dotfiles commands or packages/ recipes; implies cmd."`
	Steps []string `arg:"" optional:"" help:"Steps to run, without the per-step prompt: pacman, aur, repos, cmd, go, rust, firmware (default: all, asking for each)."`
}

func (c *UpdateCmd) Help() string {
	return `Examples:
  dctl update
  dctl update cmd go
  dctl update --only dctl,hyprd`
}

func (c *UpdateCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errors.New("run dctl update as your user; it elevates only the pacman transaction")
	}
	if err := c.only(root); err != nil {
		return err
	}
	all := c.All || u.Yes()
	steps, err := update.Select(update.Steps(u, root, execx.OSRunner{}, all, c.Only), c.Steps)
	if err != nil {
		return err
	}
	return update.Run(ctx, u, steps, !all && len(c.Steps) == 0)
}

func (c *UpdateCmd) only(root paths.Root) error {
	if len(c.Only) == 0 {
		return nil
	}
	switch {
	case len(c.Steps) == 0:
		c.Steps = []string{"cmd"}
	case !slices.Contains(c.Steps, "cmd"):
		return errors.New("--only limits the cmd step; add cmd or drop the step names")
	}
	lists, err := packages.Load(root.Packages())
	if err != nil {
		return err
	}
	known := slices.Concat(binaries.Names, lists.Local)
	for _, name := range c.Only {
		if !slices.Contains(known, name) {
			return fmt.Errorf("unknown --only %q (known: %s)", name, strings.Join(known, ", "))
		}
	}
	return nil
}
