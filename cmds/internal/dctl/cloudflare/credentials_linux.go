package cloudflare

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func loadToken(home string) (string, error) {
	path := filepath.Join(home, ".local/share/dotfiles/cloudflare.env")
	fail := func(reason string) (string, error) {
		return "", fmt.Errorf("Cloudflare credentials at %s: %s; provision a user-owned regular file with mode 0600 containing only a CLOUDFLARE_API_TOKEN assignment", path, reason)
	}

	if os.Getuid() == 0 || os.Getuid() != os.Geteuid() {
		return fail("run as your normal user, without sudo")
	}

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

	token := ""
	for line := range strings.SplitSeq(strings.TrimSuffix(string(data), "\n"), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if !ok || value == "" || strings.ContainsFunc(value, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		}) {
			return fail("malformed assignment; use an unquoted value without spaces, comments, or shell syntax")
		}
		if name != "CLOUDFLARE_API_TOKEN" {
			return fail("unexpected assignment name")
		}
		if token != "" {
			return fail("duplicate CLOUDFLARE_API_TOKEN assignment")
		}
		token = value
	}
	if token == "" {
		return fail("CLOUDFLARE_API_TOKEN is required")
	}
	return token, nil
}
