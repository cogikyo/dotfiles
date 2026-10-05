package iso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

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

func Release(ctx context.Context, u *ui.UI, root paths.Root, iso, key string) error {
	if os.Geteuid() == 0 {
		return errors.New("run release as your user; gh and the security key need your session")
	}
	f, err := os.Open(iso)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if err := oversize(st.Size(), nil); err != nil {
		return err
	}
	short := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(iso), "dotfiles-"), ".iso")
	if len(short) != 12 || strings.Trim(short, "0123456789abcdef") != "" {
		return fmt.Errorf("%s is not named dotfiles-<rev12>.iso by dctl iso build", iso)
	}
	run := execx.OSRunner{Frame: u.Frame}
	rev, err := run.Output(ctx, root.Dotfiles, "git", "rev-parse", "--verify", "--quiet", short+"^{commit}")
	if err != nil {
		return fmt.Errorf("resolve the ISO revision %s: %w", short, err)
	}
	_, err = run.Output(ctx, root.Dotfiles, "git", "merge-base", "--is-ancestor", rev, "origin/master")
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		return fmt.Errorf("%s was built from %s, which origin/master does not contain; push master first", iso, short)
	}
	if err != nil {
		return err
	}
	key, err = signingKey(root.Home, key)
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

	hash, err := digest(f)
	if err != nil {
		return err
	}
	sums := iso + ".sha256"
	if err := os.WriteFile(sums, formatSums([]sum{{hash, filepath.Base(iso)}}), 0o644); err != nil {
		return err
	}
	if err := os.Remove(sums + ".sig"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	u.Section("sign", filepath.Base(sums)+"; touch the security key")
	if err := run.Run(ctx, "", "ssh-keygen", "-Y", "sign", "-f", key, "-n", "file", sums); err != nil {
		return err
	}
	signed, err := signedSum(ctx, root.Share("allowed_signers"), iso)
	if err != nil {
		return err
	}
	if signed != hash {
		return fmt.Errorf("%s changed while signing; its sha256 is not the one measured", sums)
	}
	u.OK("signature verifies against share/allowed_signers")

	tag := "iso-" + time.Now().Format("2006.01.02") + "-" + rev[:12]
	argv := []string{"release", "create", tag, "--target", rev, "--title", tag,
		"--notes", fmt.Sprintf("Offline installer ISO built from %s. Write it with `dctl iso usb --iso %s /dev/sdX`.", rev[:12], filepath.Base(iso)),
		iso, sums, sums + ".sig"}
	u.Section("publish", tag)
	u.Detail("`gh %s`", strings.Join(argv, " "))
	typed, err := u.Text("Type "+tag+" to publish this public release", "")
	if err != nil {
		return err
	}
	if typed != tag {
		return errors.New("confirmation did not match; nothing published")
	}
	if err := run.Run(ctx, root.Dotfiles, "gh", argv...); err != nil {
		return err
	}
	u.Close(ui.OK, "published %s", tag)
	return nil
}
