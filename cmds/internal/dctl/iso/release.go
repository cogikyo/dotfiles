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
	run := execx.OSRunner{}
	rev, err := revision(ctx, run, root.Dotfiles, nil)
	if err != nil {
		return err
	}
	if want := "dotfiles-" + rev[:12] + ".iso"; filepath.Base(iso) != want {
		return fmt.Errorf("%s was not built from HEAD %s (expected %s)", iso, rev[:12], want)
	}
	pushed, err := run.Output(ctx, root.Dotfiles, "git", "rev-parse", "origin/master")
	if err != nil {
		return err
	}
	if pushed != rev {
		return fmt.Errorf("HEAD %s is not origin/master; push master first", rev[:12])
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
	if _, err := signedSum(ctx, root.Share("allowed_signers"), iso); err != nil {
		return err
	}
	u.OK("signature verifies against share/allowed_signers")

	tag := "iso-" + time.Now().Format("2006.01.02") + "-" + rev[:12]
	argv := []string{"release", "create", tag, "--target", rev, "--title", tag,
		"--notes", fmt.Sprintf("Offline installer ISO built from %s. Write it with `dctl iso usb %s /dev/sdX`.", rev[:12], filepath.Base(iso)),
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
