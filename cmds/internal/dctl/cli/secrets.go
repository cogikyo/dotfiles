package cli

import (
	"fmt"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/ui"
)

type SecretsCmd struct {
	Sync         secretsSync    `cmd:"" help:"Seal targets whose plaintext changed; unchanged secrets keep their old recipients."`
	Rekey        secretsRekey   `cmd:"" help:"Re-seal every secret to the current recipients."`
	Decrypt      secretsDecrypt `cmd:"" help:"Write targets from the repo; staged secrets only when named."`
	List         secretsList    `cmd:"" help:"Show manifest entries and target status."`
	VerifyPhrase secretsPhrase  `cmd:"" name:"verify-phrase" help:"Check the age phrase without writing anything."`
}

type secretsSync struct{}

func (secretsSync) Run(u *ui.UI, root paths.Root) error { return secrets.Sync(u, root) }

type secretsRekey struct{}

func (secretsRekey) Run(u *ui.UI, root paths.Root) error { return secrets.Rekey(u, root, nil) }

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
	w := u.Writer()
	fmt.Fprintf(w, "%-24s %-48s %-4s %s\n", "NAME", "TARGET", "MODE", "STATE")
	for _, s := range statuses {
		state := string(s.State)
		switch {
		case s.State == secrets.WrongMode:
			state = fmt.Sprintf("mode %04o", s.Have)
		case s.Staged && s.State == secrets.Missing:
			state = "staged"
		}
		if !s.Sealed {
			state += ", no ciphertext"
		}
		fmt.Fprintf(w, "%-24s %-48s %04o %s\n", s.Name, s.Path(), s.Mode, state)
	}
	return nil
}

type secretsPhrase struct{}

func (secretsPhrase) Run(u *ui.UI, root paths.Root) error { return secrets.VerifyPhrase(u, root) }
