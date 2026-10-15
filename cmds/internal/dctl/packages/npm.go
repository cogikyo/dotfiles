package packages

import (
	"context"

	"dotfiles/cmds/internal/dctl/execx"
)

func NPM(ctx context.Context, run execx.Runner, dir string) error {
	return run.Run(ctx, dir, "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund")
}
