package cli

import (
	"context"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/ui"
)

type ReposCmd struct {
	Sync   reposSync   `cmd:"" help:"Clone repos from packages/repos.lst that are missing."`
	Update reposUpdate `cmd:"" help:"Fast-forward clean checkouts; report the rest untouched."`
}

type reposSync struct{}

func (reposSync) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return repos.Sync(ctx, u, root, execx.OSRunner{})
}

type reposUpdate struct{}

func (reposUpdate) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return repos.Update(ctx, u, root, execx.OSRunner{})
}
