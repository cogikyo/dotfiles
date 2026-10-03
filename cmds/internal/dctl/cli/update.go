package cli

import (
	"context"
	"errors"
	"os"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
	"dotfiles/cmds/internal/dctl/update"
)

type UpdateCmd struct {
	All   bool     `help:"Run every step without asking and pass --noconfirm to pacman and yay; never flashes firmware."`
	Steps []string `arg:"" optional:"" help:"Steps to run, without the per-step prompt: pacman, aur, repos, cli, firmware (default: all, asking for each)."`
}

func (c *UpdateCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errors.New("run dctl update as your user; it elevates only the pacman transaction")
	}
	all := c.All || u.Yes()
	steps, err := update.Select(update.Steps(u, root, execx.OSRunner{}, all), c.Steps)
	if err != nil {
		return err
	}
	return update.Run(ctx, u, steps, !all && len(c.Steps) == 0)
}
