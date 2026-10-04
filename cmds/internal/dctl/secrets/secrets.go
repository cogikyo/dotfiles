package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"

	"filippo.io/age"
)

type State string

const (
	OK        State = "ok"
	Missing   State = "missing"
	WrongMode State = "mode"
	Irregular State = "irregular"
)

type Status struct {
	Entry
	Sealed bool        `json:"sealed"`
	State  State       `json:"state"`
	Have   fs.FileMode `json:"have,omitempty"`
}

func List(root paths.Root) ([]Status, error) {
	entries, err := Manifest(root)
	if err != nil {
		return nil, err
	}
	home, err := OpenHome(root)
	if err != nil {
		return nil, err
	}
	defer home.Close()
	return list(home, root, entries)
}

func list(home *Tree, root paths.Root, entries []Entry) ([]Status, error) {
	out := make([]Status, 0, len(entries))
	for _, e := range entries {
		s := Status{Entry: e, State: OK}
		if _, err := os.Stat(e.ciphertext(root)); err == nil {
			s.Sealed = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		st, err := home.stat(e.Target)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			s.State = Missing
		case errors.Is(err, errIrregular):
			s.State = Irregular
		case err != nil:
			return nil, err
		case st.Mode().Perm() != e.Mode:
			s.State, s.Have = WrongMode, st.Mode().Perm()
		}
		out = append(out, s)
	}
	return out, nil
}

func Decrypt(u *ui.UI, root paths.Root, names []string) error {
	entries, err := pick(root, names)
	if err != nil {
		return err
	}
	keys, err := LoadKeys(u, root)
	if err != nil {
		return err
	}
	return writeTargets(u, root, keys, entries)
}

func writeTargets(u *ui.UI, root paths.Root, keys *Keys, entries []Entry) error {
	home, err := OpenHome(root)
	if err != nil {
		return err
	}
	defer home.Close()
	type plan struct {
		e    Entry
		data []byte
	}
	var writes, changed []plan
	defer func() {
		for _, p := range slices.Concat(writes, changed) {
			clear(p.data)
		}
	}()
	var errs []error
	unchanged := 0
	for _, e := range entries {
		data, err := reveal(keys, root, e)
		if aborted(err) {
			return err
		}
		if err != nil {
			clear(data)
			errs = append(errs, fmt.Errorf("%s: %w", e.Name, err))
			continue
		}
		have, st, err := home.read(e.Target)
		same := err == nil && bytes.Equal(have, data)
		clear(have)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			writes = append(writes, plan{e, data})
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", e.Path(), err))
			clear(data)
		case same && st.Mode().Perm() == e.Mode:
			unchanged++
			clear(data)
		case same:
			writes = append(writes, plan{e, data})
		default:
			changed = append(changed, plan{e, data})
		}
	}
	if len(changed) > 0 {
		u.Warn("%d targets differ from the repo", len(changed))
		for _, p := range changed {
			u.Dim("%s", p.e.Path())
		}
		u.Warn("No plaintext backup is kept.")
		ok, err := u.Confirm(fmt.Sprintf("Overwrite these %d files?", len(changed)))
		if err != nil {
			return fmt.Errorf("confirm overwrite: %w", err)
		}
		if ok {
			writes = append(writes, changed...)
		} else {
			u.Warn("kept %d changed targets", len(changed))
		}
	}
	for _, p := range writes {
		if err := home.write(p.e.Target, p.data, p.e.Mode); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.e.Path(), err))
			continue
		}
		u.Step("%s", p.e.Path())
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	u.OK("decrypted %d, unchanged %d", len(writes), unchanged)
	return nil
}

func Sync(u *ui.UI, root paths.Root) error {
	recipients, err := Recipients(u, root)
	if err != nil {
		return err
	}
	keys, err := LoadKeys(u, root)
	if err != nil {
		return err
	}
	return sealTargets(u, root, keys, recipients)
}

func sealTargets(u *ui.UI, root paths.Root, keys *Keys, recipients []age.Recipient) error {
	entries, err := Manifest(root)
	if err != nil {
		return err
	}
	home, err := OpenHome(root)
	if err != nil {
		return err
	}
	defer home.Close()
	repo, err := openRepo(root)
	if err != nil {
		return err
	}
	defer repo.Close()
	var errs []error
	sealed, unchanged := 0, 0
	for _, e := range entries {
		data, _, err := home.read(e.Target)
		switch {
		case errors.Is(err, fs.ErrNotExist) && e.Staged:
			continue
		case errors.Is(err, fs.ErrNotExist):
			u.Warn("missing %s", e.Path())
			continue
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", e.Path(), err))
			continue
		}
		same, err := matches(keys, root, e, data)
		if aborted(err) {
			clear(data)
			return err
		}
		if err == nil && !same {
			err = seal(repo, e, data, recipients)
		}
		clear(data)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", e.Name, err))
		case same:
			unchanged++
		default:
			sealed++
			u.Step("%s <- %s", e.Name, e.Path())
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	u.OK("sealed %d, unchanged %d", sealed, unchanged)
	return nil
}

func matches(keys *Keys, root paths.Root, e Entry, data []byte) (bool, error) {
	old, err := reveal(keys, root, e)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	defer clear(old)
	return err == nil && bytes.Equal(old, data), err
}

type Ledger struct {
	Recipients []byte
	Identities []byte
}

type Edit func(*Ledger) error

// ReadLedger reads raw recipients and identities; a missing identities file is allowed, but recipients must exist.
func ReadLedger(root paths.Root) (Ledger, error) {
	var l Ledger
	var err error
	if l.Recipients, err = os.ReadFile(root.Secrets("recipients")); err != nil {
		return l, err
	}
	if l.Identities, err = os.ReadFile(root.Secrets("identities")); errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	return l, err
}

func Rekey(u *ui.UI, root paths.Root, edit Edit) error {
	if err := Preflight(root); err != nil {
		return err
	}
	l, err := ReadLedger(root)
	if err != nil {
		return err
	}
	if edit != nil {
		if err := edit(&l); err != nil {
			return err
		}
	}
	recipients, err := parseRecipients(u, l.Recipients)
	if err != nil {
		return err
	}
	keys, err := LoadKeys(u, root)
	if err != nil {
		return err
	}
	files := map[string][]byte{"recipients": l.Recipients}
	if l.Identities != nil {
		files["identities"] = l.Identities
	}
	return rekey(u, root, keys, recipients, files)
}

func rekey(u *ui.UI, root paths.Root, keys *Keys, recipients []age.Recipient, files map[string][]byte) error {
	entries, err := Manifest(root)
	if err != nil {
		return err
	}
	staged := maps.Clone(files)
	if staged == nil {
		staged = map[string][]byte{}
	}
	var errs []error
	for _, e := range entries {
		data, err := reveal(keys, root, e)
		if aborted(err) {
			return err
		}
		if err == nil {
			staged[e.Name+".age"], err = Seal(data, recipients)
		}
		clear(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("secrets/ unchanged: %w", errors.Join(errs...))
	}
	for _, name := range []string{"manifest", "recipients", "identities", "identity.age"} {
		if _, ok := staged[name]; ok {
			continue
		}
		data, err := os.ReadFile(root.Secrets(name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		staged[name] = data
	}
	if err := swap(root.Secrets(), staged); err != nil {
		return err
	}
	u.OK("re-encrypted %d secrets for %d recipients", len(entries), len(recipients))
	return nil
}

func seal(repo *Tree, e Entry, data []byte, recipients []age.Recipient) error {
	ciphertext, err := Seal(data, recipients)
	if err != nil {
		return err
	}
	return repo.write(e.Name+".age", ciphertext, 0o644)
}

var errUnlisted = errors.New("the phrase unlocked identity.age, but its recipient is not in secrets/recipients")

func VerifyPhrase(u *ui.UI, root paths.Root) error {
	phrase, err := u.Secret("Recovery phrase (paper):")
	if err != nil {
		return err
	}
	if err := checkPhrase(root, phrase); err != nil {
		return err
	}
	u.OK("recovery phrase verified")
	return nil
}

func checkPhrase(root paths.Root, phrase string) error {
	if err := debugged(); err != nil {
		return err
	}
	wrapped, err := os.ReadFile(root.Secrets("identity.age"))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(root.Secrets("recipients"))
	if err != nil {
		return err
	}
	recipients, err := recipientLines(data)
	if err != nil {
		return err
	}
	id, err := Unwrap(wrapped, phrase)
	if err != nil {
		return err
	}
	if !slices.Contains(recipients, id.Recipient().String()) {
		return errUnlisted
	}
	return nil
}

func Stage(u *ui.UI, root paths.Root) setup.Stage {
	return setup.Stage{Name: "secrets", Items: []setup.Item{
		{
			Name:  "secrets-repo",
			Check: func(context.Context) error { return checkRepo(u, root) },
		},
		{
			Name:  "secrets-targets",
			Check: func(context.Context) error { return checkTargets(root) },
			Fix:   func(context.Context) error { return restore(u, root) },
		},
	}}
}

func checkRepo(u *ui.UI, root paths.Root) error {
	if _, err := Recipients(u, root); err != nil {
		return err
	}
	if _, err := os.Stat(root.Secrets("identity.age")); err != nil {
		return err
	}
	statuses, err := List(root)
	if err != nil {
		return err
	}
	var unsealed []string
	for _, s := range statuses {
		if !s.Sealed {
			unsealed = append(unsealed, s.Name+".age")
		}
	}
	if len(unsealed) > 0 {
		return fmt.Errorf("missing ciphertext: %s", strings.Join(unsealed, ", "))
	}
	return nil
}

func checkTargets(root paths.Root) error {
	statuses, err := List(root)
	if err != nil {
		return err
	}
	var bad []string
	for _, s := range statuses {
		switch {
		case s.Staged || s.State == OK:
		case s.State == WrongMode:
			bad = append(bad, fmt.Sprintf("%s (mode %04o, want %04o)", s.Path(), s.Have, s.Mode))
		default:
			bad = append(bad, fmt.Sprintf("%s (%s)", s.Path(), s.State))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%d targets: %s", len(bad), strings.Join(bad, ", "))
	}
	return nil
}

func restore(u *ui.UI, root paths.Root) error {
	entries, err := Manifest(root)
	if err != nil {
		return err
	}
	home, err := OpenHome(root)
	if err != nil {
		return err
	}
	defer home.Close()
	statuses, err := list(home, root, entries)
	if err != nil {
		return err
	}
	keys, err := LoadKeys(u, root)
	if err != nil {
		return err
	}
	var errs []error
	for _, s := range statuses {
		if s.Staged {
			continue
		}
		switch s.State {
		case WrongMode:
			errs = append(errs, home.chmod(s.Target, s.Mode))
		case Irregular:
			errs = append(errs, fmt.Errorf("%s: %w; move it aside by hand", s.Path(), errIrregular))
		case Missing:
			data, err := reveal(keys, root, s.Entry)
			if aborted(err) {
				return err
			}
			if err == nil {
				err = home.write(s.Target, data, s.Mode)
			}
			clear(data)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Path(), err))
			}
		}
	}
	return errors.Join(errs...)
}

func pick(root paths.Root, names []string) ([]Entry, error) {
	entries, err := Manifest(root)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return slices.DeleteFunc(entries, func(e Entry) bool { return e.Staged }), nil
	}
	out := make([]Entry, 0, len(names))
	for _, name := range names {
		i := slices.IndexFunc(entries, func(e Entry) bool { return e.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("unknown secret %q", name)
		}
		out = append(out, entries[i])
	}
	return out, nil
}

func reveal(keys *Keys, root paths.Root, e Entry) ([]byte, error) {
	ciphertext, err := os.ReadFile(e.ciphertext(root))
	if err != nil {
		return nil, err
	}
	return keys.Open(ciphertext)
}

func aborted(err error) bool {
	return errors.Is(err, ui.ErrCanceled) || errors.Is(err, ui.ErrNoTTY)
}
