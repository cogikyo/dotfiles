package porkbun

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type credentials struct {
	key    string
	secret string
}

func loadCredentials(home string) (credentials, error) {
	path := filepath.Join(home, ".local/share/dotfiles/porkbun.env")
	fail := func(reason string) (credentials, error) {
		return credentials{}, fmt.Errorf("Porkbun credentials at %s: %s; provision a user-owned regular file with mode 0600 containing only PORKBUN_API_KEY and PORKBUN_API_SECRET_KEY assignments", path, reason)
	}

	if os.Getuid() == 0 || os.Getuid() != os.Geteuid() {
		return fail("run as your normal user, without sudo")
	}

	// NOFOLLOW and NONBLOCK keep the later mode/uid/regular checks on this fd, not a symlink or blocking FIFO.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fail("cannot open file; check that it exists, is readable, and is not a symlink")
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fail("cannot inspect file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode() != 0o600 || stat.Uid != uint32(os.Getuid()) {
		return fail("unsafe ownership, permissions, or file type")
	}

	data, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(data) > 8192 {
		return fail("cannot read file or file exceeds 8 KiB")
	}

	var creds credentials
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if !ok || value == "" || strings.ContainsFunc(value, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		}) {
			return fail("malformed assignment; use unquoted key values without spaces, comments, or shell syntax")
		}
		switch name {
		case "PORKBUN_API_KEY":
			if creds.key != "" {
				return fail("duplicate PORKBUN_API_KEY assignment")
			}
			creds.key = value
		case "PORKBUN_API_SECRET_KEY":
			if creds.secret != "" {
				return fail("duplicate PORKBUN_API_SECRET_KEY assignment")
			}
			creds.secret = value
		default:
			return fail("unexpected assignment name")
		}
	}
	if creds.key == "" || creds.secret == "" {
		return fail("both assignments are required")
	}
	return creds, nil
}
