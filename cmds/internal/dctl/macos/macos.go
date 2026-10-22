package macos

import (
	"context"
	"errors"
	"os"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/home"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/dctl/update"
	"dotfiles/cmds/internal/ui"
)

type CLI struct {
	JSON  bool `help:"Emit JSON results where supported."`
	Plain bool `help:"Disable colors and animation."`
	Yes   bool `short:"y" help:"Accept yes/no confirmations."`

	Setup  SetupCmd  `cmd:"" help:"Apply macOS setup stages that are pending, or redo named stages."`
	Update UpdateCmd `cmd:"" help:"Pull the dotfiles, upgrade Homebrew, reapply setup, and rebuild dctl, one step at a time."`
}

type SetupCmd struct {
	Status bool     `help:"Show stage status and change nothing; exit non-zero for pending, failed, or blocked stages."`
	From   string   `placeholder:"STAGE" help:"Redo this stage and every later one."`
	Stages []string `arg:"" optional:"" help:"Stages to redo: packages, home (default: show the full plan)."`
}

func (c *SetupCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errors.New("run dctl setup as your user; Homebrew refuses to run as root")
	}
	flow := setup.Flow{UI: u, Stages: catalog(u, root), Home: root.Home}
	return flow.Run(ctx, setup.Request{Names: c.Stages, From: c.From, Status: c.Status})
}

type UpdateCmd struct {
	All   bool     `help:"Run every step without asking."`
	Dirty bool     `help:"Build dctl from uncommitted cmds/ without asking."`
	Steps []string `arg:"" optional:"" help:"Steps to run, without the per-step prompt: dotfiles, brew, setup, cmd (default: all, asking for each)."`
}

func (c *UpdateCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errors.New("run dctl update as your user; Homebrew refuses to run as root")
	}
	all := c.All || u.Yes()
	reapply := func(ctx context.Context) error {
		flow := setup.Flow{UI: u, Stages: catalog(u, root), Home: root.Home}
		return flow.Run(ctx, setup.Request{Accept: all})
	}
	steps, err := update.Select(update.Mac(u, root, execx.OSRunner{UI: u}, all, c.Dirty, reapply), c.Steps)
	if err != nil {
		return err
	}
	return update.Run(ctx, u, steps, !all && len(c.Steps) == 0)
}

func catalog(u *ui.UI, root paths.Root) []setup.Stage {
	brew := packages.Brew(root.Packages("brew.lst"), execx.OSRunner{UI: u})
	brew.About = "Homebrew formulae and casks"
	links := home.Mac(root)
	links.About, links.Needs = "links", []string{"packages"}
	return []setup.Stage{brew, links}
}
