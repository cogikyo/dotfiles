package cli

import (
	"context"

	"dotfiles/cmds/internal/dctl/installer"
	"dotfiles/cmds/internal/dctl/ui"
)

type InstallCmd struct{}

func (c *InstallCmd) Run(ctx context.Context, u *ui.UI) error {
	return installer.Run(ctx, u)
}
