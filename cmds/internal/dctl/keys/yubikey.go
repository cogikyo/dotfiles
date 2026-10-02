package keys

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"

	"filippo.io/age/plugin"
)

type Key struct {
	Serial    string `json:"serial"`
	Slot      int    `json:"slot"`
	Stub      string `json:"-"`
	Recipient string `json:"recipient,omitempty"`
}

func (k Key) principal() string { return principal(k.Serial) }

func principal(serial string) string { return "yubikey-" + serial }

type stub struct {
	serial string
	slot   byte
	tag    [4]byte
}

func parseStub(line string) (stub, bool) {
	name, data, err := plugin.ParseIdentity(line)
	if err != nil || name != "yubikey" || len(data) < 9 {
		return stub{}, false
	}
	return stub{
		serial: strconv.FormatUint(uint64(binary.LittleEndian.Uint32(data)), 10),
		slot:   data[4],
		tag:    [4]byte(data[5:9]),
	}, true
}

func tag(recipient string) ([4]byte, bool) {
	name, data, err := plugin.ParseRecipient(recipient)
	if err != nil || name != "yubikey" {
		return [4]byte{}, false
	}
	sum := sha256.Sum256(data)
	return [4]byte(sum[:4]), true
}

func pair(stubs, recipients []string) []Key {
	var out []Key
	for _, line := range stubs {
		s, ok := parseStub(line)
		if !ok {
			continue
		}
		k := Key{Serial: s.serial, Slot: int(s.slot), Stub: line}
		for _, r := range recipients {
			if t, ok := tag(r); ok && t == s.tag {
				k.Recipient = r
				break
			}
		}
		out = append(out, k)
	}
	return out
}

func inserted(ctx context.Context, run execx.Runner) (string, error) {
	out, err := run.Output(ctx, "", "ykman", "list", "--serials")
	if err != nil {
		return "", err
	}
	serials := strings.Fields(out)
	switch len(serials) {
	case 0:
		return "", errors.New("no YubiKey with a readable serial is inserted")
	case 1:
		return serials[0], nil
	}
	return "", fmt.Errorf("%d YubiKeys inserted (%s); leave only one in", len(serials), strings.Join(serials, ", "))
}

func ykman(serial string, args ...string) []string {
	return append([]string{"--device", serial}, args...)
}

func fields(text string) map[string]string {
	out := map[string]string{}
	for line := range strings.Lines(text) {
		if k, v, ok := strings.Cut(line, ":"); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

func onKey(ctx context.Context, run execx.Runner, serial string) ([]Key, error) {
	ids, err := run.Output(ctx, "", "age-plugin-yubikey", "--identity", "--serial", serial)
	if err != nil {
		return nil, err
	}
	list, err := run.Output(ctx, "", "age-plugin-yubikey", "--list", "--serial", serial)
	if err != nil {
		return nil, err
	}
	var stubs, recipients []string
	for line := range strings.FieldsSeq(ids) {
		if strings.HasPrefix(line, "AGE-PLUGIN-YUBIKEY-") {
			stubs = append(stubs, line)
		}
	}
	for line := range strings.FieldsSeq(list) {
		if strings.HasPrefix(line, "age1yubikey1") {
			recipients = append(recipients, line)
		}
	}
	return pair(stubs, recipients), nil
}

func read(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return secrets.Lines(data), err
}

func appended(data []byte, line string) []byte {
	if slices.Contains(secrets.Lines(data), line) {
		return data
	}
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	return append(data, line+"\n"...)
}

func dropped(data []byte, drop func(string) bool) []byte {
	var keep []byte
	for line := range strings.Lines(string(data)) {
		if !drop(strings.TrimSpace(line)) {
			keep = append(keep, line...)
		}
	}
	return keep
}

func rewrite(root paths.Root, change func([]byte) []byte) error {
	checkout, err := secrets.OpenCheckout(root)
	if err != nil {
		return err
	}
	defer checkout.Close()
	rel := filepath.Join("share", "allowed_signers")
	data, err := checkout.ReadFile(rel)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return checkout.WriteFile(rel, change(data), 0o644)
}

func signers(path string) ([]string, error) {
	lines, err := read(path)
	var out []string
	for _, line := range lines {
		first, _, _ := strings.Cut(line, " ")
		out = append(out, first)
	}
	return out, err
}

func signs(line, principal string) bool {
	first, _, _ := strings.Cut(line, " ")
	return line != "" && line[0] != '#' && first == principal
}
