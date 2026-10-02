package keys

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/ui"
)

type Header struct {
	Device   string   `json:"device"`
	Fido2    []string `json:"fido2_tokens"`
	Recovery bool     `json:"recovery"`
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
		} `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(out), &meta); err != nil {
		return h, fmt.Errorf("%s: LUKS2 metadata: %w", dev, err)
	}
	for id, t := range meta.Tokens {
		switch t.Type {
		case "systemd-fido2":
			h.Fido2 = append(h.Fido2, id)
		case "systemd-recovery":
			h.Recovery = h.Recovery || len(t.Keyslots) > 0
		}
	}
	slices.Sort(h.Fido2)
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

func Luks(ctx context.Context, u *ui.UI, run execx.Runner, sys string, confirm func(string) (bool, error)) error {
	dev, err := rootDevice(ctx, run, sys)
	if err != nil {
		return err
	}
	h, err := header(ctx, run, dev)
	if err != nil {
		return err
	}
	enroll := true
	if len(h.Fido2) > 0 {
		if enroll, err = confirm(fmt.Sprintf("%s already has %d FIDO2 tokens; enroll the inserted key too?", dev, len(h.Fido2))); err != nil {
			return err
		}
	}
	if enroll {
		u.Step("FIDO2 token on %s", dev)
		if err := run.Run(ctx, "", "systemd-cryptenroll", "--fido2-device=auto", "--fido2-with-client-pin=yes", "--fido2-with-user-presence=yes", dev); err != nil {
			return err
		}
	}
	if !h.Recovery {
		u.Warn("systemd-cryptenroll shows the recovery key once; write it down")
		if err := run.Run(ctx, "", "systemd-cryptenroll", "--recovery-key", dev); err != nil {
			return err
		}
	}
	u.OK("%s: LUKS enrollment done; the passphrase slot is unchanged", dev)
	return nil
}

func Group(run execx.Runner, sys string) doctor.Group {
	return doctor.Group{Name: "keys", Root: true, Checks: []doctor.Check{{
		Name:  "keys-luks",
		Check: func(ctx context.Context) error { return checkLuks(ctx, run, sys) },
	}}}
}

func checkLuks(ctx context.Context, run execx.Runner, sys string) error {
	dev, err := rootDevice(ctx, run, sys)
	if err != nil {
		return doctor.Block("%v", err)
	}
	if os.Geteuid() != 0 {
		return doctor.Block("reading the LUKS header of %s needs root: sudo dctl doctor keys", dev)
	}
	h, err := header(ctx, run, dev)
	if err != nil {
		return err
	}
	var missing []string
	if len(h.Fido2) == 0 {
		missing = append(missing, "FIDO2 token")
	}
	if !h.Recovery {
		missing = append(missing, "recovery key")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s has no %s: sudo dctl keys luks", dev, strings.Join(missing, " or "))
	}
	return nil
}
