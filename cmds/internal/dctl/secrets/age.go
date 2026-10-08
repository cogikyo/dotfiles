package secrets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
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
	return plugin.NewRecipient(line, clientUI(u, new(error), new(bool)))
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

var errPhrase = errors.New("recovery phrase does not unlock identity.age")

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
	plugins []yubikey
	ids     []age.Identity
	piv     piv
	stop    error
	gone    bool
	halt    error
}

type yubikey struct {
	id     age.Identity
	serial string
}

func stubSerial(stub string) string {
	name, data, err := plugin.ParseIdentity(stub)
	if err != nil || name != "yubikey" || len(data) < 4 {
		return ""
	}
	return strconv.FormatUint(uint64(binary.LittleEndian.Uint32(data)), 10)
}

type piv interface {
	Serials() ([]string, error)
	Tries(serial string) (int, error)
}

type ykman struct{}

func (ykman) Serials() ([]string, error) {
	out, err := exec.Command("ykman", "list", "--serials").Output()
	if err != nil {
		return nil, fmt.Errorf("ykman list --serials: %w", err)
	}
	return strings.Fields(string(out)), nil
}

var triesLine = regexp.MustCompile(`(?m)^PIN tries remaining:\s+(\d+)`)

func (ykman) Tries(serial string) (int, error) {
	out, err := exec.Command("ykman", "--device", serial, "piv", "info").Output()
	if err != nil {
		return 0, fmt.Errorf("ykman piv info: %w", err)
	}
	m := triesLine.FindSubmatch(out)
	if m == nil {
		return 0, errors.New("ykman piv info: PIV PIN tries unavailable")
	}
	return strconv.Atoi(string(m[1]))
}

func LoadKeys(u *ui.UI, root paths.Root) (*Keys, error) {
	if err := debugged(); err != nil {
		return nil, err
	}
	k := &Keys{u: u, wrapped: root.Secrets("identity.age"), piv: ykman{}}
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
		id, err := plugin.NewIdentity(stub, clientUI(u, &k.stop, &k.gone))
		if err != nil {
			return nil, fmt.Errorf("secrets/identities: %w", err)
		}
		k.plugins = append(k.plugins, yubikey{id, stubSerial(stub)})
	}
	return k, nil
}

var (
	errSkipped = errors.New("secrets unlock skipped")
	errNoKey   = errors.New("no enrolled YubiKey detected")
)

type halt struct{ error }

func (h halt) Unwrap() error { return h.error }

func (k *Keys) Open(ciphertext []byte) ([]byte, error) {
	if k.halt != nil {
		return nil, k.halt
	}
	if len(k.plugins) > 0 {
		data, phrase, err := k.unlock(ciphertext)
		if !phrase {
			return data, err
		}
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

const (
	again = iota
	usePhrase
	skip
)

func (k *Keys) unlock(ciphertext []byte) ([]byte, bool, error) {
	for {
		ids, serials, err := k.inserted()
		if err == nil && len(ids) > 0 {
			var data []byte
			data, err = open(ciphertext, ids...)
			switch {
			case err == nil:
				return data, false, nil
			case k.stop != nil:
				return nil, false, fmt.Errorf("%w: %w", k.stop, err)
			case k.gone:
				k.gone, ids, err = false, nil, nil
			}
		}
		title, retry := "", ""
		switch {
		case err == nil:
			err, title, retry = errNoKey, "Insert an enrolled YubiKey", "Retry"
		case wrongPIN(err):
			left, blocked := k.tries(serials)
			err = fmt.Errorf("PIV PIN rejected · %s", left)
			if !blocked {
				retry = "Retry PIV PIN"
			}
		}
		if title == "" {
			title = err.Error()
		}
		choice, err := k.decide(err, title, retry)
		switch {
		case err != nil:
			return nil, false, err
		case choice == usePhrase:
			return nil, true, nil
		case choice == skip:
			k.halt = halt{errSkipped}
			return nil, false, k.halt
		}
	}
}

func (k *Keys) inserted() ([]age.Identity, []string, error) {
	have, err := k.piv.Serials()
	if err != nil {
		return nil, nil, err
	}
	var ids []age.Identity
	var serials []string
	for _, p := range k.plugins {
		if slices.Contains(have, p.serial) {
			ids = append(ids, p.id)
			serials = append(serials, p.serial)
		}
	}
	return ids, serials, nil
}

func wrongPIN(err error) bool {
	msg := strings.ToLower(err.Error())
	return slices.ContainsFunc([]string{"wrong pin", "invalid pin", "pin was too", "pin locked", "tries remaining"}, func(s string) bool {
		return strings.Contains(msg, s)
	})
}

func (k *Keys) tries(serials []string) (string, bool) {
	var parts []string
	blocked := 0
	for _, serial := range serials {
		left := "PIV PIN tries unknown"
		if n, err := k.piv.Tries(serial); err == nil {
			left = fmt.Sprintf("%d PIV PIN tries left", n)
			if n == 0 {
				left = "PIV PIN blocked"
				blocked++
			}
		}
		if len(serials) > 1 {
			left = serial + ": " + left
		}
		parts = append(parts, left)
	}
	if len(parts) == 0 {
		return "PIV PIN tries unknown", false
	}
	return strings.Join(parts, ", "), blocked == len(serials)
}

func (k *Keys) decide(reason error, title, retry string) (int, error) {
	if k.u.Yes() || !k.u.Can() {
		k.halt = halt{reason}
		return 0, k.halt
	}
	options := []string{"Use recovery phrase", "Skip secrets unlock"}
	if retry != "" {
		options = slices.Insert(options, 0, retry)
	}
	i, err := k.u.Select(title, options, 0)
	if err != nil {
		return 0, err
	}
	if retry == "" {
		i++
	}
	return i, nil
}

func (k *Keys) phrase() (*age.X25519Identity, error) {
	wrapped, err := os.ReadFile(k.wrapped)
	if err != nil {
		return nil, err
	}
	for {
		phrase, err := k.u.Secret("Recovery phrase (paper):")
		if err != nil {
			return nil, err
		}
		id, err := Unwrap(wrapped, phrase)
		if !errors.Is(err, errPhrase) {
			return id, err
		}
		k.u.Warn("recovery phrase does not match")
	}
}

func clientUI(u *ui.UI, stop *error, gone *bool) *plugin.ClientUI {
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
			if name == "yubikey" && strings.Contains(prompt, "insert") {
				*gone = true
				return false, nil
			}
			options := []string{yes}
			if no != "" {
				options = append(options, no)
			}
			i, err := u.Select(prompt, options, 0)
			return i == 0, note(err)
		},
		WaitTimer: func(name string) {
			u.Info("waiting for age-plugin-%s; touch the YubiKey if it blinks", name)
		},
	}
}
