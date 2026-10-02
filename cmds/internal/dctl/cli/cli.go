package cli

import (
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/porkbun"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/update"
)

type CLI struct {
	JSON  bool `help:"Emit one JSON document."`
	Plain bool `help:"Disable colors and animation."`
	Yes   bool `short:"y" help:"Assume yes for confirmations."`

	Doctor  DoctorCmd   `cmd:"" group:"actions" help:"Check the machine and optionally fix it."`
	Porkbun porkbun.Cmd `cmd:"" group:"actions" help:"Manage personal Porkbun DNS records (Linux)."`

	Update  update.Cmd `cmd:"" group:"lifecycle" help:"Update system and package lists."`
	Secrets SecretsCmd `cmd:"" group:"lifecycle" help:"Manage age-encrypted secrets."`
	Keys    KeysCmd    `cmd:"" group:"lifecycle" help:"Enroll, remove, and inspect YubiKeys."`
	Repos   repos.Cmd  `cmd:"" group:"lifecycle" help:"Manage configured repositories."`
	ISO     iso.Cmd    `cmd:"" name:"iso" group:"lifecycle" help:"Build and release custom Arch ISOs."`
}
