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

type isoBuild struct {
	Fresh bool `help:"Discard the build cache in /var/cache/dctl-iso first."`
}

func (c isoBuild) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return iso.Build(ctx, u, root, c.Fresh)
	}
	args := []string{"iso", "build"}
	if c.Fresh {
		args = append(args, "--fresh")
	}
	return asRoot(ctx, u, root, args...)
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
	ISO    string `arg:"" optional:"" help:"ISO to test (default: the newest built ISO)."`
	Dctl   string `type:"existingfile" xor:"dctl" help:"dctl binary that replaces the ISO's."`
	Bundle string `type:"existingfile" xor:"bundle" help:"git bundle that replaces the ISO's."`
	Keep   bool   `help:"Keep the VM disk and firmware variables."`
	Head   bool   `xor:"dctl,bundle" help:"Replace the ISO's dctl with one built from this checkout and its bundle with HEAD as master."`
}

func (c isoTest) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if c.ISO == "" {
		var err error
		if c.ISO, err = iso.Latest(root.Dotfiles); err != nil {
			return err
		}
		u.KV("iso", c.ISO)
	}
	return iso.Test(ctx, u, root, iso.TestOptions(c))
}

type isoUSB struct {
	Device string `arg:"" help:"Whole removable disk, e.g. /dev/sdX."`
	ISO    string `help:"ISO with .sha256 and .sha256.sig beside it (default: the newest built ISO)."`
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
		if c.ISO, err = iso.Latest(root.Dotfiles); err != nil {
			return err
		}
		u.KV("iso", c.ISO)
	}
	return asRoot(ctx, u, root, "iso", "usb", "--iso", c.ISO, c.Device)
}

type isoRelease struct {
	ISO string `arg:"" optional:"" help:"ISO to release (default: the newest built ISO)."`
	Key string `help:"Hardware SSH key that signs the checksum (default: ~/.ssh/id_ed25519_sk_<serial> of the inserted YubiKey); its public key must be in share/allowed_signers."`
}

func (c isoRelease) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if c.ISO == "" {
		var err error
		if c.ISO, err = iso.Latest(root.Dotfiles); err != nil {
			return err
		}
		u.KV("iso", c.ISO)
	}
	return iso.Release(ctx, u, root, c.ISO, c.Key)
}
