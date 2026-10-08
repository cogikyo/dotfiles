package secureboot

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"
)

const (
	tcti    = "device:/dev/tpmrm0"
	marker  = "etc/dctl/boot-totp"       // Copied into the initramfs by sd-totp after authenticator verification.
	built   = "etc/dctl/boot-totp.built" // Matches the marker only after a verified boot-image rebuild.
	empty   = "No TOTP secret is currently stored"
	changed = "The system state has changed"
	lockout = "lockout mode"
)

var (
	errUnsealed = errors.New("boot TOTP not sealed")
	errUnmarked = errors.New("TOTP secret sealed; /" + marker + " marker missing")
	errUnbuilt  = errors.New("boot images not rebuilt since the boot TOTP marker changed")
	errStopped  = errors.New("stopped before authenticator verification")
)

func totp(args ...string) []string { return append([]string{"-T", tcti}, args...) }

func enforced(efi string) error {
	fw, err := Read(efi)
	switch {
	case err != nil:
		return err
	case !fw.Enabled || fw.Setup:
		return errors.New("Secure Boot not enforced; run dctl setup secureboot, reboot, then continue")
	}
	return nil
}

func marked(root string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, marker))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func rebuilt(root string) (bool, error) {
	note, err := os.ReadFile(filepath.Join(root, marker))
	if err != nil {
		return false, err
	}
	stamp, err := os.ReadFile(filepath.Join(root, built))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil && string(stamp) == string(note), err
}

func TOTP(u *ui.UI, run execx.Runner, root string) setup.Stage {
	check := func(ctx context.Context) error { return sealed(ctx, run, root) }
	return setup.Stage{Name: "totp", Root: true, Items: []setup.Item{{
		Name:  "totp-sealed",
		Check: check,
		Fix: func(ctx context.Context) error {
			switch err := check(ctx); {
			case errors.Is(err, errUnbuilt):
				return rebuild(ctx, run, root)
			case errors.Is(err, errUnsealed), errors.Is(err, errUnmarked):
				return Seal(ctx, u, run, root)
			}
			return nil
		},
	}}}
}

func sealed(ctx context.Context, run execx.Runner, root string) error {
	if enforced(filepath.Join(root, "sys", "firmware", "efi")) != nil {
		return setup.Later("after reboot with Secure Boot enabled")
	}
	tpm := filepath.Join(root, "dev", "tpmrm0")
	if _, err := os.Stat(tpm); errors.Is(err, fs.ErrNotExist) {
		return setup.Manual("no TPM at %s; enable it in the BIOS, then check with systemd-analyze has-tpm2", tpm)
	}
	was, err := marked(root)
	if err != nil {
		return err
	}
	_, err = run.Output(ctx, "", "tpm2-totp", totp("show")...)
	switch {
	case err == nil && !was:
		return errUnmarked
	case err == nil:
		ok, err := rebuilt(root)
		switch {
		case err != nil:
			return err
		case !ok:
			return errUnbuilt
		}
		return nil
	case strings.Contains(err.Error(), empty) && was:
		return setup.Manual("NO BOOT TOTP: sealed secret missing; investigate before resealing with sudo dctl keys totp")
	case strings.Contains(err.Error(), empty):
		return errUnsealed
	case strings.Contains(err.Error(), changed):
		return setup.Manual("NO BOOT TOTP: PCR 0 or 7 changed; investigate before resealing; reseal only after a known firmware or Secure Boot change, with sudo dctl keys totp")
	case strings.Contains(err.Error(), lockout):
		return setup.Manual("TPM in lockout; do not clear it; wait, then check with sudo tpm2-totp -T %s show", tcti)
	}
	return setup.Manual("tpm2-totp cannot unseal (%s); check with sudo tpm2-totp -T %s show", last(err), tcti)
}
func last(err error) string {
	lines := strings.Split(strings.TrimSpace(err.Error()), "\n")
	return lines[len(lines)-1]
}

func Seal(ctx context.Context, u *ui.UI, run execx.Runner, root string) error {
	if err := enforced(filepath.Join(root, "sys", "firmware", "efi")); err != nil {
		return err
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	had, err := marked(root)
	if err != nil {
		return err
	}
	if err := unmark(root); err != nil {
		return err
	}
	init := execx.Reason(execx.Interactive(run), "seal a new TOTP secret to PCRs 0 and 7")
	for {
		if _, err := run.Output(ctx, "", "tpm2-totp", totp("clean")...); err != nil && !strings.Contains(err.Error(), empty) {
			return fmt.Errorf("could not remove old TOTP secret: %w", err)
		}
		u.Section("Boot TOTP", "add the QR code or secret= value to your authenticator; shown once")
		if err := init.Run(ctx, "", "tpm2-totp", totp("--pcrs", "0,7", "--banks", "SHA256", "--label", host, "init")...); err != nil {
			return discard(ctx, run, root, had, err)
		}
		ok, err := matches(ctx, u, run)
		if err != nil {
			return discard(ctx, run, root, had, err)
		}
		if ok {
			u.OK("authenticator code verified")
			if err := mark(ctx, run, root, host); err != nil {
				return err
			}
			u.Info("reboot; compare the boot TOTP with your authenticator before unlocking")
			return nil
		}
	}
}

func mark(ctx context.Context, run execx.Runner, root, label string) error {
	path := filepath.Join(root, marker)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	note := fmt.Sprintf("label=%s\nsealed=%s\n", label, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(note), 0o644); err != nil {
		return err
	}
	return rebuild(ctx, run, root)
}

func unmark(root string) error {
	for _, name := range []string{marker, built} {
		if err := os.Remove(filepath.Join(root, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func rebuild(ctx context.Context, run execx.Runner, root string) error {
	if err := run.Run(ctx, "", "limine-update"); err != nil {
		return fmt.Errorf("limine-update failed; boot images may not match /%s: %w", marker, err)
	}
	if err := verify(ctx, run, root); err != nil {
		return fmt.Errorf("boot-image verification failed after limine-update; do not reboot until dctl setup secureboot passes: %w", err)
	}
	note, err := os.ReadFile(filepath.Join(root, marker))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, built), note, 0o644)
}

func matches(ctx context.Context, u *ui.UI, run execx.Runner) (bool, error) {
	for {
		before, err := run.Output(ctx, "", "tpm2-totp", totp("show")...)
		if err != nil {
			return false, err
		}
		code, err := u.Text("Authenticator code", "")
		if err != nil {
			return false, err
		}
		after, err := run.Output(ctx, "", "tpm2-totp", totp("show")...)
		if err != nil {
			return false, err
		}
		if code = strings.ReplaceAll(code, " ", ""); code == before || code == after {
			return true, nil
		}
		i, err := u.Select("Code does not match the TPM", []string{"Retry code", "Replace secret and scan again", "Stop and remove the new secret"}, 0)
		switch {
		case err != nil:
			return false, err
		case i == 1:
			return false, nil
		case i == 2:
			return false, errStopped
		}
	}
}

func discard(ctx context.Context, run execx.Runner, root string, had bool, cause error) error {
	ctx = context.WithoutCancel(ctx)
	var errs []error
	if _, err := run.Output(ctx, "", "tpm2-totp", totp("clean")...); err != nil && !strings.Contains(err.Error(), empty) {
		errs = append(errs, fmt.Errorf("could not remove unconfirmed TOTP secret; run sudo dctl keys totp: %w", err))
	}
	if had {
		if err := rebuild(ctx, run, root); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w; %w", cause, errors.Join(errs...))
	}
	return fmt.Errorf("%w; unconfirmed TOTP secret and marker removed", cause)
}
