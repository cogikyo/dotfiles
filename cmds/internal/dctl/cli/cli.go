package cli

import "dotfiles/cmds/internal/dctl/porkbun"

type CLI struct {
	JSON  bool `help:"Emit JSON results where supported."`
	Plain bool `help:"Disable colors and animation."`
	Yes   bool `short:"y" help:"Accept yes/no confirmations; never confirms erasing a disk."`

	Install InstallCmd  `cmd:"" help:"Erase a whole disk and install this machine from the dctl ISO (root, live environment only)."`
	Setup   SetupCmd    `cmd:"" help:"Apply setup stages that are pending, or redo named stages."`
	Porkbun porkbun.Cmd `cmd:"" help:"Manage personal Porkbun DNS records (Linux)."`
	Update  UpdateCmd   `cmd:"" help:"Upgrade packages, repos, and commands, one step at a time."`
	Secrets SecretsCmd  `cmd:"" help:"Manage age-encrypted secrets."`
	Keys    KeysCmd     `cmd:"" help:"Enroll, remove, and inspect YubiKeys."`
	ISO     ISOCmd      `cmd:"" name:"iso" help:"Build, test, write, and release the offline installer ISO."`
}
