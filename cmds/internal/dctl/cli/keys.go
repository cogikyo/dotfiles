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
	"dotfiles/cmds/internal/dctl/secureboot"
	"dotfiles/cmds/internal/ui"
)

type KeysCmd struct {
	Enroll keysEnroll `cmd:"" help:"Enroll the inserted YubiKey: PINs, age identity, rekey, release-signing key."`
	Luks   keysLuks   `cmd:"" help:"Add the inserted YubiKey and a recovery key to the root LUKS2 header (root)."`
	Remove keysRemove `cmd:"" help:"Remove the age recipient and release signer, then rekey; LUKS tokens remain."`
	Status keysStatus `cmd:"" help:"Show enrolled and inserted YubiKeys and LUKS tokens."`
	Totp   keysTotp   `cmd:"" name:"totp" help:"Seal a new boot TOTP secret to the TPM and show it once (root, Secure Boot enforced)."`
}

var errUser = errors.New("run as your user, not root")

type keysEnroll struct{}

func (keysEnroll) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errUser
	}
	return secrets.Locked(root, func() error {
		return keys.Enroll(ctx, u, root, execx.OSRunner{Frame: u.Frame}, func(e secrets.Edit) error { return secrets.Rekey(u, root, e) })
	})
}

type keysLuks struct{}

func (keysLuks) Run(ctx context.Context, u *ui.UI) error {
	if os.Geteuid() != 0 {
		return errors.New("needs root: sudo dctl keys luks")
	}
	return keys.Luks(ctx, u, execx.OSRunner{Frame: u.Frame}, "/sys", u.Confirm)
}

type keysTotp struct{}

func (keysTotp) Run(ctx context.Context, u *ui.UI) error {
	if os.Geteuid() != 0 {
		return errors.New("needs root: sudo dctl keys totp")
	}
	return secureboot.Seal(ctx, u, execx.OSRunner{Frame: u.Frame}, "/sys/firmware/efi")
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
	for _, e := range r.Enrolled {
		level := ui.OK
		if e.Recipient == "" || !e.Signer {
			level = ui.Warn
		}
		u.Node(level, "yubikey "+e.Serial, fmt.Sprintf("age recipient %s, release signer %s", yes(e.Recipient != ""), yes(e.Signer)))
	}
	if r.Inserted != "" {
		u.Node(ui.Info, "inserted", "yubikey "+r.Inserted)
	} else {
		u.Node(ui.Info, "inserted", r.Absent)
	}
	switch {
	case r.Unread != "":
		u.Node(ui.Warn, "luks", r.Unread)
	case r.Luks != nil:
		level := ui.OK
		if len(r.Luks.Fido2) == 0 || !r.Luks.Recovery {
			level = ui.Warn
		}
		u.Node(level, r.Luks.Device, fmt.Sprintf("%d FIDO2 tokens, recovery key %s", len(r.Luks.Fido2), yes(r.Luks.Recovery)))
	}
	if len(r.Enrolled) == 0 {
		u.Close(ui.Warn, "no YubiKey enrolled")
		return nil
	}
	u.Close(ui.OK, "%d enrolled", len(r.Enrolled))
	return nil
}

func yes(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}
