package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

type ISOCmd struct {
	Build   isoBuild   `cmd:"" help:"Build the offline installer ISO; asks for sudo once."`
	Test    isoTest    `cmd:"" help:"Install an ISO into a QEMU VM and check the installed system."`
	USB     isoUSB     `cmd:"" name:"usb" help:"Verify a signed ISO and write it to a removable disk."`
	Release isoRelease `cmd:"" help:"Sign an ISO and publish it as a GitHub release."`
}

type isoBuild struct{}

func (isoBuild) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return iso.Build(ctx, u, root)
	}
	return asRoot(ctx, u, root, "iso", "build")
}

// asRoot reruns dctl with args through sudo, continuing this output tree.
func asRoot(ctx context.Context, u *ui.UI, root paths.Root, args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := command(ctx, slices.Concat([]string{"sudo", "env", "DOTFILES=" + root.Dotfiles}, u.Env(), []string{exe}, globals(u), args)...)
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if exit.ExitCode() == 130 {
			return ui.ErrCanceled
		}
		return fmt.Errorf("dctl %s failed; see above", strings.Join(args[:2], " "))
	}
	return err
}

type isoTest struct {
	ISO    string `arg:"" optional:"" help:"ISO to test (default: the one built from HEAD)."`
	Dctl   string `type:"existingfile" help:"dctl binary that replaces the ISO's."`
	Bundle string `type:"existingfile" help:"git bundle that replaces the ISO's."`
	Keep   bool   `help:"Keep the VM disk and firmware variables."`
}

func (c isoTest) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if c.ISO == "" {
		var err error
		if c.ISO, err = iso.Current(ctx, root.Dotfiles); err != nil {
			return err
		}
	}
	return iso.Test(ctx, u, iso.TestOptions(c))
}

type isoUSB struct {
	Device string `arg:"" help:"Whole removable disk, e.g. /dev/sdX."`
	ISO    string `help:"ISO with .sha256 and .sha256.sig beside it (default: the one built from HEAD)."`
}

func (c isoUSB) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		if c.ISO == "" {
			return errors.New("run dctl iso usb from your user account, or pass --iso")
		}
		return iso.USB(ctx, u, root, c.ISO, c.Device)
	}
	if c.ISO == "" {
		var err error
		if c.ISO, err = iso.Current(ctx, root.Dotfiles); err != nil {
			return err
		}
	}
	return asRoot(ctx, u, root, "iso", "usb", "--iso", c.ISO, c.Device)
}

type isoRelease struct {
	ISO string `arg:"" optional:"" help:"ISO to release (default: the one built from HEAD)."`
	Key string `help:"Hardware SSH key that signs the checksum (default: the only ~/.ssh/id_ed25519_sk_* key); its public key must be in share/allowed_signers."`
}

func (c isoRelease) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if c.ISO == "" {
		var err error
		if c.ISO, err = iso.Current(ctx, root.Dotfiles); err != nil {
			return err
		}
	}
	return iso.Release(ctx, u, root, c.ISO, c.Key)
}
