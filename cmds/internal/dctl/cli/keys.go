package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/keys"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/ui"
)

type KeysCmd struct {
	Enroll keysEnroll `cmd:"" help:"Enroll the inserted YubiKey: PINs, age identity, rekey, release-signing key."`
	Luks   keysLuks   `cmd:"" help:"Add the inserted YubiKey and a recovery key to the root LUKS2 header (root)."`
	Remove keysRemove `cmd:"" help:"Remove the age recipient and release signer, then rekey; LUKS tokens remain."`
	Status keysStatus `cmd:"" help:"Show enrolled and inserted YubiKeys and LUKS tokens."`
}

var errUser = errors.New("run as your user, not root")

type keysEnroll struct{}

func (keysEnroll) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errUser
	}
	return secrets.Locked(root, func() error {
		return keys.Enroll(ctx, u, root, execx.OSRunner{IO: true}, func(e secrets.Edit) error { return secrets.Rekey(u, root, e) })
	})
}

type keysLuks struct{}

func (keysLuks) Run(ctx context.Context, u *ui.UI) error {
	if os.Geteuid() != 0 {
		return errors.New("needs root: sudo dctl keys luks")
	}
	return keys.Luks(ctx, u, execx.OSRunner{IO: true}, "/sys", u.Confirm)
}

type keysRemove struct {
	Serial string `arg:"" help:"Serial of the lost YubiKey."`
}

func (c keysRemove) Run(u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("%w; for LUKS tokens, see sudo dctl keys status and systemd-cryptenroll --wipe-slot", errUser)
	}
	return secrets.Locked(root, func() error {
		return keys.Remove(u, root, c.Serial, func(e secrets.Edit) error { return secrets.Rekey(u, root, e) })
	})
}

type keysStatus struct{}

func (keysStatus) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	r, err := keys.Status(ctx, root, execx.OSRunner{}, "/sys")
	if err != nil {
		return err
	}
	if u.JSON() {
		return u.Emit(r)
	}
	if len(r.Enrolled) == 0 {
		u.Info("no YubiKey enrolled")
	}
	for _, e := range r.Enrolled {
		u.Info("yubikey %s: age recipient %s, release signer %s", e.Serial, yes(e.Recipient != ""), yes(e.Signer))
	}
	if r.Inserted != "" {
		u.Info("inserted: yubikey %s", r.Inserted)
	}
	if r.Luks != nil {
		u.Info("%s: %d FIDO2 tokens, recovery key %s", r.Luks.Device, len(r.Luks.Fido2), yes(r.Luks.Recovery))
	}
	for _, n := range r.Notes {
		u.Dim("%s", n)
	}
	return nil
}

func yes(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}
