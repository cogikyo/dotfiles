package keys

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/ui"
)

const application = "ssh:dctl-release"

func Enroll(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner, rekey func(secrets.Edit) error) error {
	if err := secrets.Preflight(root); err != nil {
		return err
	}
	serial, err := inserted(ctx, run)
	if err != nil {
		return err
	}
	u.Step("yubikey %s", serial)
	pins, err := pinSteps(ctx, run, serial)
	if err != nil {
		return err
	}
	for _, args := range pins {
		if _, err := run.Run(ctx, "", "ykman", args...); err != nil {
			return err
		}
	}
	if err := enrollAge(ctx, u, run, serial, rekey); err != nil {
		return err
	}
	if err := enrollSigner(ctx, root, run, serial); err != nil {
		return err
	}
	u.OK("yubikey %s enrolled; for disk unlock run: sudo dctl keys luks", serial)
	return nil
}

func pinSteps(ctx context.Context, run execx.Runner, serial string) ([][]string, error) {
	out, err := run.Output(ctx, "", "ykman", ykman(serial, "fido", "info")...)
	if err != nil {
		return nil, err
	}
	fido := fields(out)
	var steps [][]string
	switch fido["PIN"] {
	case "Not set":
		steps = append(steps, ykman(serial, "fido", "access", "change-pin"))
	case "Blocked":
		return nil, fmt.Errorf("yubikey %s: the FIDO2 PIN is blocked", serial)
	}
	if fido["Always Require UV"] == "Off" {
		steps = append(steps, ykman(serial, "fido", "config", "toggle-always-uv"))
	}
	return append(steps, ykman(serial, "piv", "access", "change-pin"), ykman(serial, "piv", "access", "change-puk")), nil
}

func enrollAge(ctx context.Context, u *ui.UI, run execx.Runner, serial string, rekey func(secrets.Edit) error) error {
	keys, err := onKey(ctx, run, serial)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		if _, err := run.Run(ctx, "", "age-plugin-yubikey", "--generate", "--serial", serial, "--pin-policy", "once", "--touch-policy", "cached"); err != nil {
			return err
		}
		if keys, err = onKey(ctx, run, serial); err != nil {
			return err
		}
	}
	u.Step("rekeying with yubikey %s", serial)
	return rekey(func(l *secrets.Ledger) error {
		have := lines(l.Recipients)
		i := slices.IndexFunc(keys, func(k Key) bool { return slices.Contains(have, k.Recipient) })
		if i < 0 {
			i = slices.IndexFunc(keys, func(k Key) bool { return k.Recipient != "" })
		}
		if i < 0 {
			return fmt.Errorf("yubikey %s holds no age identity with a recipient", serial)
		}
		l.Recipients = appended(l.Recipients, keys[i].Recipient)
		l.Identities = appended(l.Identities, keys[i].Stub)
		return nil
	})
}

func enrollSigner(ctx context.Context, root paths.Root, run execx.Runner, serial string) error {
	signed, err := read(root.Share("allowed_signers"))
	if err != nil {
		return err
	}
	i := slices.IndexFunc(signed, func(line string) bool { return signs(line, principal(serial)) })
	home, err := secrets.OpenHome(root)
	if err != nil {
		return err
	}
	defer home.Close()
	rel := filepath.Join(".ssh", "id_ed25519_sk_"+serial)
	name, args := "id", []string{"-t", "ed25519-sk", "-O", "resident", "-O", "application=" + application, "-C", principal(serial), "-f", "id"}
	if i >= 0 {
		pub, err := home.ReadFile(rel + ".pub")
		if priv, perr := home.ReadFile(rel); err == nil && perr == nil && skKey(string(pub)) == skKey(signed[i]) {
			clear(priv)
			return nil
		}
		name, args = "id_ed25519_sk_rk_"+strings.TrimPrefix(application, "ssh:"), []string{"-K"}
	}
	tmp, err := os.MkdirTemp("", "dctl-sk-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err := run.Run(ctx, tmp, "ssh-keygen", args...); err != nil {
		return err
	}
	pub, err := os.ReadFile(filepath.Join(tmp, name+".pub"))
	if err != nil {
		return fmt.Errorf("no %s key from ssh-keygen: %w", application, err)
	}
	key := skKey(string(pub))
	if key == "" || i >= 0 && key != skKey(signed[i]) {
		return fmt.Errorf("the %s key from ssh-keygen does not match allowed_signers", application)
	}
	priv, err := os.ReadFile(filepath.Join(tmp, name))
	if err != nil {
		return err
	}
	defer clear(priv)
	if err := home.WriteFile(rel+".pub", pub, 0o644); err != nil {
		return err
	}
	if err := home.WriteFile(rel, priv, 0o600); err != nil || i >= 0 {
		return err
	}
	line := fmt.Sprintf(`%s namespaces="file" %s`, principal(serial), key)
	return rewrite(root, func(data []byte) []byte { return appended(data, line) })
}

func skKey(line string) string {
	f := strings.Fields(line)
	i := slices.Index(f, "sk-ssh-ed25519@openssh.com")
	if i < 0 || i+1 >= len(f) {
		return ""
	}
	return f[i] + " " + f[i+1]
}

func Remove(u *ui.UI, root paths.Root, serial string, rekey func(secrets.Edit) error) error {
	stubs, err := read(root.Secrets("identities"))
	if err != nil {
		return err
	}
	have, err := signers(root.Share("allowed_signers"))
	if err != nil {
		return err
	}
	stubbed := slices.ContainsFunc(pair(stubs, nil), func(k Key) bool { return k.Serial == serial })
	signed := slices.Contains(have, principal(serial))
	if !stubbed && !signed {
		return fmt.Errorf("yubikey %s is not enrolled", serial)
	}
	var errs []error
	if signed {
		if err := rewrite(root, func(data []byte) []byte {
			return dropped(data, func(line string) bool { return signs(line, principal(serial)) })
		}); err != nil {
			errs = append(errs, fmt.Errorf("allowed_signers still lists yubikey %s: %w", serial, err))
		}
	}
	if stubbed {
		err := rekey(func(l *secrets.Ledger) error {
			var gone []string
			for _, k := range pair(lines(l.Identities), lines(l.Recipients)) {
				if k.Serial == serial {
					gone = append(gone, k.Stub, k.Recipient)
				}
			}
			drop := func(line string) bool { return line != "" && slices.Contains(gone, line) }
			l.Recipients, l.Identities = dropped(l.Recipients, drop), dropped(l.Identities, drop)
			return nil
		})
		switch {
		case errors.Is(err, secrets.ErrCommitted):
			errs = append(errs, fmt.Errorf("yubikey %s removed from secrets: %w", serial, err))
		case err != nil:
			errs = append(errs, fmt.Errorf("yubikey %s still opens every secret; rerun dctl keys remove %s: %w", serial, serial, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	u.OK("yubikey %s removed; any LUKS FIDO2 token it made stays in the header", serial)
	return nil
}

type Enrolled struct {
	Key
	Signer bool `json:"signer"`
}

type Report struct {
	Enrolled []Enrolled `json:"enrolled"`
	Inserted string     `json:"inserted,omitempty"`
	Luks     *Header    `json:"luks,omitempty"`
	Notes    []string   `json:"notes,omitempty"`
}

func Status(ctx context.Context, root paths.Root, run execx.Runner, sys string) (Report, error) {
	var r Report
	recipients, err := read(root.Secrets("recipients"))
	if err != nil {
		return r, err
	}
	stubs, err := read(root.Secrets("identities"))
	if err != nil {
		return r, err
	}
	have, err := signers(root.Share("allowed_signers"))
	if err != nil {
		return r, err
	}
	for _, k := range pair(stubs, recipients) {
		r.Enrolled = append(r.Enrolled, Enrolled{Key: k, Signer: slices.Contains(have, k.principal())})
	}
	for _, p := range have {
		serial, ok := strings.CutPrefix(p, "yubikey-")
		if ok && !slices.ContainsFunc(r.Enrolled, func(e Enrolled) bool { return e.Serial == serial }) {
			r.Enrolled = append(r.Enrolled, Enrolled{Key: Key{Serial: serial}, Signer: true})
		}
	}
	if r.Inserted, err = inserted(ctx, run); err != nil {
		r.Notes = append(r.Notes, err.Error())
	}
	h, err := luks(ctx, run, sys)
	if err != nil {
		r.Notes = append(r.Notes, err.Error())
		return r, nil
	}
	r.Luks = &h
	return r, nil
}
