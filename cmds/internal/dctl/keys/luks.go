package keys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"
)

const (
	YubiKeys = 2
	enrolled = "etc/dctl/luks-yubikeys"             // Serials for this disk; the LUKS header cannot identify each key.
	unpaper  = "etc/dctl/luks-recovery.unconfirmed" // Persists until the user confirms the paper copy.
)

type Token struct {
	ID    string   `json:"id"`
	Slots []string `json:"keyslots"`
	PIN   bool     `json:"pin"`
	Touch bool     `json:"touch"`
}

func (t Token) unattended() bool { return t.PIN && !t.Touch }

type Header struct {
	Device   string  `json:"device"`
	Fido2    []Token `json:"fido2_tokens"`
	Recovery bool    `json:"recovery"`
}

func (h Header) Unattended() int {
	n := 0
	for _, t := range h.Fido2 {
		if t.unattended() {
			n++
		}
	}
	return n
}

func rootDevice(ctx context.Context, run execx.Runner, sys string) (string, error) {
	src, err := run.Output(ctx, "", "findmnt", "-nvo", "SOURCE", "/")
	if err != nil {
		return "", err
	}
	name, ok := strings.CutPrefix(src, "/dev/mapper/")
	if !ok {
		return "", fmt.Errorf("/ is on %s, not a LUKS2 mapping", src)
	}
	dms, err := filepath.Glob(filepath.Join(sys, "block", "dm-*"))
	if err != nil {
		return "", err
	}
	for _, dm := range dms {
		if b, err := os.ReadFile(filepath.Join(dm, "dm", "name")); err != nil || strings.TrimSpace(string(b)) != name {
			continue
		}
		uuid, err := os.ReadFile(filepath.Join(dm, "dm", "uuid"))
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(string(uuid), "CRYPT-LUKS2-") {
			return "", fmt.Errorf("%s is not a LUKS2 mapping", src)
		}
		slaves, err := os.ReadDir(filepath.Join(dm, "slaves"))
		if err != nil {
			return "", err
		}
		if len(slaves) != 1 {
			return "", fmt.Errorf("%s has %d backing devices", src, len(slaves))
		}
		return "/dev/" + slaves[0].Name(), nil
	}
	return "", fmt.Errorf("%s has no device-mapper entry under %s", src, sys)
}

func header(ctx context.Context, run execx.Runner, dev string) (Header, error) {
	h := Header{Device: dev}
	out, err := run.Output(ctx, "", "cryptsetup", "luksDump", "--dump-json-metadata", dev)
	if err != nil {
		return h, err
	}
	var meta struct {
		Tokens map[string]struct {
			Type     string   `json:"type"`
			Keyslots []string `json:"keyslots"`
			PIN      *bool    `json:"fido2-clientPin-required"`
			UP       *bool    `json:"fido2-up-required"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(out), &meta); err != nil {
		return h, fmt.Errorf("%s: LUKS2 metadata: %w", dev, err)
	}
	for id, t := range meta.Tokens {
		switch t.Type {
		case "systemd-fido2":
			h.Fido2 = append(h.Fido2, Token{
				ID:    id,
				Slots: t.Keyslots,
				PIN:   t.PIN != nil && *t.PIN,
				Touch: t.UP == nil || *t.UP,
			})
		case "systemd-recovery":
			h.Recovery = h.Recovery || len(t.Keyslots) > 0
		}
	}
	slices.SortFunc(h.Fido2, func(a, b Token) int { return strings.Compare(a.ID, b.ID) })
	return h, nil
}

func luks(ctx context.Context, run execx.Runner, sys string) (Header, error) {
	dev, err := rootDevice(ctx, run, sys)
	if err != nil {
		return Header{}, err
	}
	if os.Geteuid() != 0 {
		return Header{Device: dev}, fmt.Errorf("reading the LUKS header of %s needs root", dev)
	}
	return header(ctx, run, dev)
}

func Stage(u *ui.UI, run execx.Runner, root string) setup.Stage {
	sys := filepath.Join(root, "sys")
	ledger := filepath.Join(root, enrolled)
	pending := filepath.Join(root, unpaper)
	read := func(ctx context.Context) (Header, error) {
		dev, err := rootDevice(ctx, run, sys)
		if err != nil {
			return Header{}, setup.Manual("%v", err)
		}
		return header(ctx, run, dev)
	}
	return setup.Stage{Name: "luks", Root: true, Items: []setup.Item{
		{
			Name: "luks-yubikeys",
			Check: func(ctx context.Context) error {
				h, err := read(ctx)
				if err != nil {
					return err
				}
				return checkTokens(h)
			},
			Fix: func(ctx context.Context) error {
				h, err := read(ctx)
				if err != nil {
					return nil
				}
				return enrollTokens(ctx, u, run, h, ledger)
			},
		},
		{
			Name: "luks-recovery",
			Check: func(ctx context.Context) error {
				h, err := read(ctx)
				if err != nil {
					return err
				}
				if !h.Recovery {
					return errors.New("no recovery key")
				}
				if _, err := os.Stat(pending); err == nil {
					return fmt.Errorf("recovery key enrolled; paper copy unconfirmed; if lost, replace it with %s", replace(h.Device))
				} else if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				return nil
			},
			Fix: func(ctx context.Context) error {
				h, err := read(ctx)
				if err != nil {
					return nil
				}
				if !h.Recovery {
					return enrollRecovery(ctx, u, run, h.Device, pending)
				}
				if _, err := os.Stat(pending); err != nil {
					return nil
				}
				u.Section("Recovery key for "+h.Device, "")
				return confirm(u, h.Device, pending)
			},
		},
		cmdline(run, root),
	}}
}

func checkTokens(h Header) error {
	if n := h.Unattended(); n < YubiKeys {
		return fmt.Errorf("%d of %d PIN-required, no-touch FIDO2 tokens enrolled", n, YubiKeys)
	}
	for _, t := range h.Fido2 {
		if !t.unattended() {
			return setup.Manual("token %[1]s requires touch or does not require a PIN; insert only its YubiKey and enroll a replacement with sudo systemd-cryptenroll --fido2-device=auto --fido2-with-client-pin=yes --fido2-with-user-presence=no %[3]s; then remove the old slots with sudo systemd-cryptenroll --wipe-slot=%[2]s %[3]s", t.ID, strings.Join(t.Slots, ","), h.Device)
		}
	}
	return nil
}

var errSkip = errors.New("skipped")

func enrollTokens(ctx context.Context, u *ui.UI, run execx.Runner, h Header, ledger string) error {
	for n := h.Unattended(); n < YubiKeys; n++ {
		u.Section(fmt.Sprintf("YubiKey %d of %d for %s", n+1, YubiKeys, h.Device), "insert a YubiKey not yet enrolled for this disk")
		serial, err := insert(ctx, u, run, ledger)
		if errors.Is(err, errSkip) {
			return setup.Manual("skipped after %d of %d enrollments; run sudo dctl keys luks with the remaining YubiKey", n, YubiKeys)
		}
		if err != nil {
			return err
		}
		if err := record(ledger, serial); err != nil {
			return fmt.Errorf("YubiKey %s not enrolled; could not record it in /%s: %w", serial, enrolled, err)
		}
		enroll := execx.Reason(execx.Interactive(run), "enroll YubiKey "+serial+"; enter the disk passphrase, then the FIDO2 PIN")
		if err := enroll.Run(ctx, "", "systemd-cryptenroll", "--fido2-device=auto", "--fido2-with-client-pin=yes", "--fido2-with-user-presence=no", h.Device); err != nil {
			if uerr := unrecord(ledger, serial); uerr != nil {
				return fmt.Errorf("%w; YubiKey %s stays listed in /%s; delete its line there: %w", err, serial, enrolled, uerr)
			}
			return err
		}
		u.OK("YubiKey %s enrolled; disk passphrase retained", serial)
	}
	return nil
}

func record(ledger, serial string) error {
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(serial + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func unrecord(ledger, serial string) error {
	data, err := os.ReadFile(ledger)
	if err != nil {
		return err
	}
	return os.WriteFile(ledger, dropped(data, func(line string) bool { return line == serial }), 0o644)
}

func insert(ctx context.Context, u *ui.UI, run execx.Runner, ledger string) (string, error) {
	for {
		answer, err := u.Text("Insert only this YubiKey; Enter continues, s skips enrollment", "")
		if err != nil {
			return "", err
		}
		if strings.EqualFold(answer, "s") {
			return "", errSkip
		}
		serial, err := Inserted(ctx, run)
		if err != nil {
			u.Warn("%v", err)
			continue
		}
		done, err := read(ledger)
		if err != nil {
			return "", err
		}
		if slices.Contains(done, serial) {
			u.Warn("YubiKey %s is listed in /%s as enrolled for this disk; insert the other key, or delete its line there if it has no token", serial, enrolled)
			continue
		}
		out, err := run.Output(ctx, "", "ykman", ykman(serial, "fido", "info")...)
		if err != nil {
			return "", err
		}
		pin := fields(out)["PIN"]
		u.KV("YubiKey", serial)
		u.KV("FIDO2 PIN", pin)
		if err := tries(serial, pin, out); err != nil {
			return "", err
		}
		return serial, nil
	}
}

func tries(serial, pin, info string) error {
	left, err := strconv.Atoi(strings.TrimSuffix(pin, " attempt(s) remaining"))
	switch {
	case pin == "Not set":
		return setup.Manual("YubiKey %s has no FIDO2 PIN; insert only this key and run dctl keys enroll", serial)
	case pin == "Blocked":
		return setup.Manual("YubiKey %[1]s FIDO2 PIN blocked; only ykman --device %[1]s fido reset clears it, and that erases every FIDO2 credential on the key, including its release-signing key and any disk token it made; then run dctl keys enroll", serial)
	case err != nil:
		return setup.Manual("YubiKey %[1]s FIDO2 PIN state is %[2]q; check ykman --device %[1]s fido info", serial, pin)
	case strings.Contains(info, "temporarily blocked"):
		return setup.Manual("YubiKey %s FIDO2 PIN temporarily blocked after 3 wrong PINs; remove and reinsert it, then run sudo dctl keys luks", serial)
	case left <= 3:
		return setup.Manual("YubiKey %[1]s has %[2]d FIDO2 PIN tries left, and enrollment can spend 3; one correct PIN restores them: ykman --device %[1]s fido access verify-pin, then run sudo dctl keys luks", serial, left)
	}
	return nil
}

func replace(dev string) string {
	return "sudo systemd-cryptenroll --recovery-key --wipe-slot=recovery " + dev
}

func enrollRecovery(ctx context.Context, u *ui.UI, run execx.Runner, dev, pending string) error {
	u.Section("Recovery key for "+dev, "")
	u.Warn("write the recovery key on paper and keep it away from this machine")
	if err := os.MkdirAll(filepath.Dir(pending), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(pending, []byte(dev+"\n"), 0o644); err != nil {
		return err
	}
	enroll := execx.Reason(execx.Interactive(run), "enter the disk passphrase; the recovery key is printed once")
	if err := enroll.Run(ctx, "", "systemd-cryptenroll", "--recovery-key", dev); err != nil {
		return err
	}
	return confirm(u, dev, pending)
}

func confirm(u *ui.UI, dev, pending string) error {
	i, err := u.Select("Is the recovery key on paper?", []string{"Written down", "Not written down; replace it later"}, 0)
	if err != nil {
		return fmt.Errorf("recovery key enrolled; paper copy unconfirmed; if lost, replace it with %s: %w", replace(dev), err)
	}
	if i != 0 {
		return setup.Manual("recovery key not on paper; replace it with %s", replace(dev))
	}
	return os.Remove(pending)
}

var legacy = regexp.MustCompile(`[ \t]*rd\.luks\.options=[0-9A-Fa-f-]+=fido2-device=auto([ \t"']|$)`)

func cmdline(run execx.Runner, root string) setup.Item {
	defaults := filepath.Join(root, "etc", "default", "limine")
	conf := filepath.Join(root, "boot", "limine.conf")
	stale := func() ([]string, error) {
		var found []string
		for _, path := range []string{defaults, conf} {
			data, err := os.ReadFile(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if legacy.Match(data) {
				found = append(found, path)
			}
		}
		return found, nil
	}
	return setup.Item{
		Name: "luks-cmdline",
		Check: func(context.Context) error {
			found, err := stale()
			if err != nil || len(found) == 0 {
				return err
			}
			return fmt.Errorf("rd.luks.options=…fido2-device=auto blocks disk-passphrase unlock (%s)", strings.Join(found, ", "))
		},
		Fix: func(ctx context.Context) error {
			found, err := stale()
			if err != nil || len(found) == 0 {
				return err
			}
			data, err := os.ReadFile(defaults)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if fixed := legacy.ReplaceAll(data, []byte("${1}")); err == nil && string(fixed) != string(data) {
				if err := os.WriteFile(defaults, fixed, 0o644); err != nil {
					return err
				}
			}
			return run.Run(ctx, "", "limine-update")
		},
	}
}
