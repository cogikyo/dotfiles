package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"

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
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := command(ctx, slices.Concat([]string{"sudo", "env", "DOTFILES=" + root.Dotfiles}, u.Env(), []string{exe}, globals(u), []string{"iso", "build"})...)
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if exit.ExitCode() == 130 {
			return ui.ErrCanceled
		}
		return errors.New("ISO build failed; see above")
	}
	return err
}

type isoTest struct {
	ISO    string `arg:"" type:"existingfile" help:"ISO from dctl iso build."`
	Dctl   string `type:"existingfile" help:"dctl binary that replaces the ISO's."`
	Bundle string `type:"existingfile" help:"git bundle that replaces the ISO's."`
	Keep   bool   `help:"Keep the VM disk and firmware variables."`
}

func (c isoTest) Run(ctx context.Context, u *ui.UI) error {
	return iso.Test(ctx, u, iso.TestOptions(c))
}

type isoUSB struct {
	ISO    string `arg:"" type:"existingfile" help:"ISO with .sha256 and .sha256.sig beside it."`
	Device string `arg:"" help:"Whole removable disk, e.g. /dev/sdX."`
}

func (c isoUSB) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return iso.USB(ctx, u, root, c.ISO, c.Device)
}

type isoRelease struct {
	ISO string `arg:"" type:"existingfile" help:"ISO from dctl iso build (dotfiles-<rev>.iso)."`
	Key string `help:"Hardware SSH key that signs the checksum (default: the only ~/.ssh/id_ed25519_sk_* key); its public key must be in share/allowed_signers."`
}

func (c isoRelease) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return iso.Release(ctx, u, root, c.ISO, c.Key)
}
