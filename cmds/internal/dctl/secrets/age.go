package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"

	"filippo.io/age"
	"filippo.io/age/plugin"
)

func Lines(data []byte) []string {
	var out []string
	for line := range strings.Lines(string(data)) {
		if line = strings.TrimSpace(line); line != "" && line[0] != '#' {
			out = append(out, line)
		}
	}
	return out
}

func recipientLines(data []byte) ([]string, error) {
	out := Lines(data)
	if len(out) == 0 {
		return nil, errors.New("secrets/recipients: no recipients")
	}
	return out, nil
}

func debugged() error {
	if _, ok := os.LookupEnv("AGEDEBUG"); ok {
		return errors.New("refusing to run with AGEDEBUG set: it logs plugin traffic, including PINs and file keys")
	}
	return nil
}

func Recipients(u *ui.UI, root paths.Root) ([]age.Recipient, error) {
	data, err := os.ReadFile(root.Secrets("recipients"))
	if err != nil {
		return nil, err
	}
	return parseRecipients(u, data)
}

func parseRecipients(u *ui.UI, data []byte) ([]age.Recipient, error) {
	if err := debugged(); err != nil {
		return nil, err
	}
	lines, err := recipientLines(data)
	if err != nil {
		return nil, err
	}
	out := make([]age.Recipient, 0, len(lines))
	for _, line := range lines {
		r, err := parseRecipient(line, u)
		if err != nil {
			return nil, fmt.Errorf("recipient %q: %w", line, err)
		}
		out = append(out, r)
	}
	return out, nil
}

func parseRecipient(line string, u *ui.UI) (age.Recipient, error) {
	if r, err := age.ParseX25519Recipient(line); err == nil {
		return r, nil
	}
	return plugin.NewRecipient(line, clientUI(u, new(error)))
}

func Seal(plaintext []byte, recipients []age.Recipient) ([]byte, error) {
	var out bytes.Buffer
	w, err := age.Encrypt(&out, recipients...)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func open(ciphertext []byte, ids ...age.Identity) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), ids...)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(ciphertext)+bytes.MinRead))
	if _, err := buf.ReadFrom(r); err != nil {
		clear(buf.Bytes())
		return nil, err
	}
	return buf.Bytes(), nil
}

var errPhrase = errors.New("the phrase does not unlock identity.age")

func Unwrap(wrapped []byte, phrase string) (*age.X25519Identity, error) {
	scrypt, err := age.NewScryptIdentity(phrase)
	if err != nil {
		return nil, err
	}
	data, err := open(wrapped, scrypt)
	if _, ok := errors.AsType[*age.NoIdentityMatchError](err); ok {
		return nil, errPhrase
	}
	if err != nil {
		return nil, err
	}
	defer clear(data)
	ids, err := age.ParseIdentities(bytes.NewReader(data))
	if err != nil || len(ids) != 1 {
		return nil, errors.New("identity.age must hold exactly one X25519 identity")
	}
	id, ok := ids[0].(*age.X25519Identity)
	if !ok {
		return nil, errors.New("identity.age must hold exactly one X25519 identity")
	}
	return id, nil
}

type Keys struct {
	u       *ui.UI
	wrapped string
	plugins []age.Identity
	ids     []age.Identity
	stop    error
}

func LoadKeys(u *ui.UI, root paths.Root) (*Keys, error) {
	if err := debugged(); err != nil {
		return nil, err
	}
	k := &Keys{u: u, wrapped: root.Secrets("identity.age")}
	data, err := os.ReadFile(root.Secrets("identities"))
	if errors.Is(err, fs.ErrNotExist) {
		return k, nil
	}
	if err != nil {
		return nil, err
	}
	for _, stub := range Lines(data) {
		if !strings.HasPrefix(stub, "AGE-PLUGIN-") {
			return nil, errors.New("secrets/identities: only plugin identity stubs belong here")
		}
		id, err := plugin.NewIdentity(stub, clientUI(u, &k.stop))
		if err != nil {
			return nil, fmt.Errorf("secrets/identities: %w", err)
		}
		k.plugins = append(k.plugins, id)
	}
	return k, nil
}

func (k *Keys) Open(ciphertext []byte) ([]byte, error) {
	if len(k.plugins) > 0 {
		data, err := open(ciphertext, k.plugins...)
		if err == nil {
			return data, nil
		}
		if k.stop != nil {
			return nil, fmt.Errorf("%w: %w", k.stop, err)
		}
		k.u.Warn("%v", err)
		k.u.Info("falling back to the age phrase")
		k.plugins = nil
	}
	if len(k.ids) == 0 {
		id, err := k.phrase()
		if err != nil {
			return nil, err
		}
		k.ids = []age.Identity{id}
	}
	return open(ciphertext, k.ids...)
}

func (k *Keys) phrase() (*age.X25519Identity, error) {
	wrapped, err := os.ReadFile(k.wrapped)
	if err != nil {
		return nil, err
	}
	for {
		phrase, err := k.u.Secret("Age phrase:")
		if err != nil {
			return nil, err
		}
		id, err := Unwrap(wrapped, phrase)
		if !errors.Is(err, errPhrase) {
			return id, err
		}
		k.u.Warn("wrong phrase")
	}
}

func clientUI(u *ui.UI, stop *error) *plugin.ClientUI {
	note := func(err error) error {
		if aborted(err) {
			*stop = err
		}
		return err
	}
	return &plugin.ClientUI{
		DisplayMessage: func(name, message string) error {
			u.Info("%s: %s", name, message)
			return nil
		},
		RequestValue: func(name, prompt string, secret bool) (string, error) {
			if secret {
				value, err := u.Secret(prompt)
				return value, note(err)
			}
			value, err := u.Text(prompt, "")
			return value, note(err)
		},
		Confirm: func(name, prompt, yes, no string) (bool, error) {
			options := []string{yes}
			if no != "" {
				options = append(options, no)
			}
			i, err := u.Select(prompt, options, 0)
			return i == 0, note(err)
		},
		WaitTimer: func(name string) {
			u.Info("waiting on age-plugin-%s; touch the key if it blinks", name)
		},
	}
}
