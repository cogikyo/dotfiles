package iso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
)

type Cmd struct {
	Build   BuildCmd   `cmd:"" help:"Build the offline installer ISO (sudo)."`
	Test    TestCmd    `cmd:"" help:"Install an ISO into a QEMU VM and check the installed system."`
	USB     USBCmd     `cmd:"" name:"usb" help:"Verify a signed ISO and write it to a removable disk."`
	Release ReleaseCmd `cmd:"" help:"Sign an ISO and publish it as a GitHub release."`
}

type ReleaseCmd struct {
	ISO string `arg:"" type:"existingfile" help:"ISO from dctl iso build (dotfiles-<rev>.iso)."`
	Key string `help:"Hardware SSH key that signs the checksum (default: the only ~/.ssh/id_ed25519_sk_* key); its public key must be in share/allowed_signers."`
}

func signingKey(home, flag string) (string, error) {
	if flag != "" {
		return paths.ExpandHome(home, flag), nil
	}
	found, err := filepath.Glob(filepath.Join(home, ".ssh", "id_ed25519_sk_*"))
	if err != nil {
		return "", err
	}
	found = slices.DeleteFunc(found, func(f string) bool { return strings.HasSuffix(f, ".pub") })
	switch len(found) {
	case 0:
		return "", errors.New("no ~/.ssh/id_ed25519_sk_* key; enroll one with `dctl keys enroll`")
	case 1:
		return found[0], nil
	}
	return "", fmt.Errorf("several signing keys (%s); pick one with --key", strings.Join(found, ", "))
}

func (c ReleaseCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if os.Geteuid() == 0 {
		return errors.New("run release as your user; gh and the security key need your session")
	}
	st, err := os.Stat(c.ISO)
	if err != nil {
		return err
	}
	if err := oversize(st.Size(), nil); err != nil {
		return err
	}
	run := execx.OSRunner{}
	rev, err := revision(ctx, run, root.Dotfiles, nil)
	if err != nil {
		return err
	}
	if want := "dotfiles-" + rev[:12] + ".iso"; filepath.Base(c.ISO) != want {
		return fmt.Errorf("%s was not built from HEAD %s (expected %s)", c.ISO, rev[:12], want)
	}
	if pushed, err := run.Output(ctx, root.Dotfiles, "git", "rev-parse", "origin/master"); err != nil || pushed != rev {
		return fmt.Errorf("HEAD %s is not origin/master; push master first", rev[:12])
	}
	key, err := signingKey(root.Home, c.Key)
	if err != nil {
		return err
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(pub), "sk-") {
		return fmt.Errorf("%s is not a hardware (sk-) key", key)
	}

	sum, err := digest(c.ISO)
	if err != nil {
		return err
	}
	sums := c.ISO + ".sha256"
	if err := os.WriteFile(sums, []byte(sum+"  "+filepath.Base(c.ISO)+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.Remove(sums + ".sig"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	u.Step("Signing %s; touch the security key", filepath.Base(sums))
	if _, err := (execx.OSRunner{IO: true}).Run(ctx, "", "ssh-keygen", "-Y", "sign", "-f", key, "-n", "file", sums); err != nil {
		return err
	}
	if signed, err := signedSum(ctx, root.Share("allowed_signers"), c.ISO); err != nil || signed != sum {
		return fmt.Errorf("signed checksum does not verify against share/allowed_signers: %v", err)
	}
	u.OK("signature verifies against share/allowed_signers")

	tag := "iso-" + time.Now().Format("2006.01.02") + "-" + rev[:12]
	argv := []string{"release", "create", tag, "--target", rev, "--title", tag,
		"--notes", fmt.Sprintf("Offline installer ISO built from %s. Write it with `dctl iso usb %s /dev/sdX`.", rev[:12], filepath.Base(c.ISO)),
		c.ISO, sums, sums + ".sig"}
	u.Info("gh %s", strings.Join(argv, " "))
	typed, err := u.Text("Type "+tag+" to publish this public release", "")
	if err != nil {
		return err
	}
	if typed != tag {
		return errors.New("confirmation did not match; nothing published")
	}
	if _, err := (execx.OSRunner{IO: true}).Run(ctx, root.Dotfiles, "gh", argv...); err != nil {
		return err
	}
	u.OK("published %s", tag)
	return nil
}
