package cli

import (
	"context"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
	"dotfiles/cmds/internal/dctl/update"
)

type UpdateCmd struct{}

func (UpdateCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return update.Run(ctx, u, root.Packages(), execx.OSRunner{IO: true})
}
