package cli

import (
	"context"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
)

type UpdateCmd struct{}

func (UpdateCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return packages.Update(ctx, u, root.Packages(), execx.OSRunner{})
}
