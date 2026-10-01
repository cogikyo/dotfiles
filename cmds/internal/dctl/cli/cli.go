package cli

import (
	"dotfiles/cmds/internal/dctl/install"
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/porkbun"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/update"
)

type CLI struct {
	JSON  bool `help:"Emit one JSON document."`
	Plain bool `help:"Disable colors and animation."`
	Yes   bool `short:"y" help:"Assume yes for confirmations."`

	Doctor  DoctorCmd   `cmd:"" group:"actions" help:"Check the machine and optionally fix it."`
	Porkbun porkbun.Cmd `cmd:"" group:"actions" help:"Manage personal Porkbun DNS records (Linux)."`

	Update  update.Cmd  `cmd:"" group:"lifecycle" help:"Update system and package lists."`
	Secrets secrets.Cmd `cmd:"" group:"lifecycle" help:"Manage age-encrypted secrets."`
	Install install.Cmd `cmd:"" group:"lifecycle" help:"Run dotfiles install steps."`
	Repos   repos.Cmd   `cmd:"" group:"lifecycle" help:"Manage configured repositories."`
	ISO     iso.Cmd     `cmd:"" name:"iso" group:"lifecycle" help:"Build and release custom Arch ISOs."`
}
