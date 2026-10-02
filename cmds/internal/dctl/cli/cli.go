package cli

import "dotfiles/cmds/internal/dctl/porkbun"

type CLI struct {
	JSON  bool `help:"Emit JSON results where supported."`
	Plain bool `help:"Disable colors and animation."`
	Yes   bool `short:"y" help:"Accept yes/no confirmations; typed disk and release confirmations remain required."`

	Install InstallCmd  `cmd:"" help:"Erase a whole disk and install this machine from the dctl ISO (root, live environment only)."`
	Doctor  DoctorCmd   `cmd:"" help:"Check the machine and optionally fix it."`
	Porkbun porkbun.Cmd `cmd:"" help:"Manage personal Porkbun DNS records (Linux)."`
	Update  UpdateCmd   `cmd:"" help:"Upgrade the system and report package-list drift."`
	Secrets SecretsCmd  `cmd:"" help:"Manage age-encrypted secrets."`
	Keys    KeysCmd     `cmd:"" help:"Enroll, remove, and inspect YubiKeys."`
	Repos   ReposCmd    `cmd:"" help:"Manage configured repositories."`
	ISO     ISOCmd      `cmd:"" name:"iso" help:"Build, test, write, and release the offline installer ISO."`
}
