package cli

import (
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/porkbun"
)

type CLI struct {
	JSON  bool `help:"Emit one JSON document."`
	Plain bool `help:"Disable colors and animation."`
	Yes   bool `short:"y" help:"Assume yes for confirmations."`

	Install InstallCmd  `cmd:"" group:"actions" help:"Install this machine from the dctl ISO (live environment only)."`
	Doctor  DoctorCmd   `cmd:"" group:"actions" help:"Check the machine and optionally fix it."`
	Porkbun porkbun.Cmd `cmd:"" group:"actions" help:"Manage personal Porkbun DNS records (Linux)."`

	Update  UpdateCmd  `cmd:"" group:"lifecycle" help:"Upgrade the system and report package-list drift."`
	Secrets SecretsCmd `cmd:"" group:"lifecycle" help:"Manage age-encrypted secrets."`
	Keys    KeysCmd    `cmd:"" group:"lifecycle" help:"Enroll, remove, and inspect YubiKeys."`
	Repos   ReposCmd   `cmd:"" group:"lifecycle" help:"Manage configured repositories."`
	ISO     iso.Cmd    `cmd:"" name:"iso" group:"lifecycle" help:"Build, write, and release the offline installer ISO."`
}
