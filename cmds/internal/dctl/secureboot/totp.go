package secureboot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"
)

const (
	tcti  = "device:/dev/tpmrm0"
	empty = "No TOTP secret is currently stored"
)

func totp(args ...string) []string { return append([]string{"-T", tcti}, args...) }

func enforced(efi string) error {
	fw, err := Read(efi)
	switch {
	case err != nil:
		return err
	case !fw.Enabled || fw.Setup:
		return errors.New("Secure Boot is not enforced; finish dctl setup secureboot and enable Secure Boot in the BIOS first")
	}
	return nil
}

func sealed(ctx context.Context, run execx.Runner, efi string) error {
	if err := enforced(efi); err != nil {
		return setup.Manual("%v", err)
	}
	if _, err := run.Output(ctx, "", "tpm2-totp", totp("show")...); err != nil {
		return setup.Manual("no TOTP unseals at the current boot state; run sudo dctl keys totp (%v)", err)
	}
	return nil
}

func Seal(ctx context.Context, u *ui.UI, run execx.Runner, efi string) error {
	if err := enforced(efi); err != nil {
		return err
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	if _, err := run.Output(ctx, "", "tpm2-totp", totp("clean")...); err != nil && !strings.Contains(err.Error(), empty) {
		return fmt.Errorf("remove the old TOTP secret: %w", err)
	}
	u.Section("TOTP secret", "sealed to PCRs 0,7 (SHA256)")
	if err := run.Run(ctx, "", "tpm2-totp", totp("--pcrs", "0,7", "--banks", "SHA256", "--label", host, "init")...); err != nil {
		return err
	}
	u.Warn("scan the QR code into your authenticator now; it shows only once")
	return nil
}
