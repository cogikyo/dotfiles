// Package cli defines the top-level dctl command tree.
//
// Responsibilities:
// - Group root actions and lifecycle commands for Kong.
// - Keep global flags on the application context used by subcommands.
package cli

// cli.go defines the root command groups and small action handlers.

import (
	"dotfiles/cmds/internal/dctl/app"
	"dotfiles/cmds/internal/dctl/install"
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/update"
)

type CLI struct {
	JSON     bool `help:"Emit one JSON document."`
	Plain    bool `help:"Disable colors and TUI affordances."`
	Yes      bool `short:"y" help:"Assume yes for safe confirmations."`
	Defaults bool `help:"Use defaults and avoid interactive prompts."`

	Check CheckCmd `cmd:"" group:"actions" help:"Run install healthchecks."`

	Update  update.Cmd  `cmd:"" group:"lifecycle" help:"Update system and package lists."`
	Secrets secrets.Cmd `cmd:"" group:"lifecycle" help:"Manage age-encrypted secrets."`
	Install install.Cmd `cmd:"" group:"lifecycle" help:"Run dotfiles install steps."`
	Repos   repos.Cmd   `cmd:"" group:"lifecycle" help:"Manage configured repositories."`
	ISO     iso.Cmd     `cmd:"" name:"iso" group:"lifecycle" help:"Build and release custom Arch ISOs."`
}

func NormalizeArgs(args []string) []string {
	if len(args) > 0 && args[0] == "plain" {
		out := make([]string, 0, len(args)+1)
		out = append(out, "--plain")
		out = append(out, args[1:]...)
		return out
	}
	return args
}

type CheckCmd struct{}

func (c *CheckCmd) Run(ctx *app.Context) error {
	return install.Check(ctx.Context, ctx.Root, ctx.Output, nil)
}
