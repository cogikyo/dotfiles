package cli

import (
	"fmt"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/ui"
)

type SecretsCmd struct {
	Sync         secretsSync    `cmd:"" help:"Seal targets whose plaintext changed; unchanged secrets keep their old recipients."`
	Rekey        secretsRekey   `cmd:"" help:"Re-seal every secret to the current recipients."`
	Decrypt      secretsDecrypt `cmd:"" help:"Write targets from the repo; staged secrets only when named."`
	List         secretsList    `cmd:"" help:"Show manifest entries and target status."`
	VerifyPhrase secretsPhrase  `cmd:"" name:"verify-phrase" help:"Check the age phrase without writing anything."`
}

type secretsSync struct{}

func (secretsSync) Run(u *ui.UI, root paths.Root) error {
	return secrets.Locked(root, func() error { return secrets.Sync(u, root) })
}

type secretsRekey struct{}

func (secretsRekey) Run(u *ui.UI, root paths.Root) error {
	return secrets.Locked(root, func() error { return secrets.Rekey(u, root, nil) })
}

type secretsDecrypt struct {
	Names []string `arg:"" optional:"" help:"Secrets to write (default: every entry not marked staged)."`
}

func (c secretsDecrypt) Run(u *ui.UI, root paths.Root) error {
	return secrets.Decrypt(u, root, c.Names)
}

type secretsList struct{}

func (secretsList) Run(u *ui.UI, root paths.Root) error {
	statuses, err := secrets.List(root)
	if err != nil {
		return err
	}
	if u.JSON() {
		return u.Emit(statuses)
	}
	off := 0
	for _, s := range statuses {
		level, state := ui.OK, string(s.State)
		switch {
		case s.State == secrets.WrongMode:
			level, state = ui.Warn, fmt.Sprintf("mode %04o", s.Have)
		case s.Staged && s.State == secrets.Missing:
			level, state = ui.Info, "staged"
		case s.State != secrets.OK:
			level = ui.Warn
		}
		if !s.Sealed {
			level, state = ui.Warn, state+", no ciphertext"
		}
		if level == ui.Warn {
			off++
		}
		note := fmt.Sprintf("%s %04o", s.Path(), s.Mode)
		if s.State != secrets.OK || !s.Sealed {
			note += ", " + state
		}
		u.Node(level, s.Name, note)
	}
	if off > 0 {
		u.Close(ui.Warn, "%d of %d secrets need attention", off, len(statuses))
		return nil
	}
	u.Close(ui.OK, "%d secrets", len(statuses))
	return nil
}

type secretsPhrase struct{}

func (secretsPhrase) Run(u *ui.UI, root paths.Root) error { return secrets.VerifyPhrase(u, root) }
