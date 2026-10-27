package system

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

const sysusers = "etc/sysusers.d"

func declared(overlay string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(overlay, sysusers, "*.conf"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		got, err := parseSysusers(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		names = append(names, got...)
	}
	return names, nil
}

func parseSysusers(r io.Reader) ([]string, error) {
	var names []string
	s := bufio.NewScanner(r)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) >= 2 && (fields[0] == "u" || fields[0] == "u!") {
			names = append(names, fields[1])
		}
	}
	return names, s.Err()
}

func checkUsers(rootDir, overlay string) error {
	names, err := declared(overlay)
	if err != nil || len(names) == 0 {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	// Host NSS results do not prove an account exists in the target root.
	data, err := root.ReadFile("etc/passwd")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var have []string
	for line := range strings.Lines(string(data)) {
		name, _, _ := strings.Cut(line, ":")
		have = append(have, name)
	}
	var missing []string
	for _, name := range names {
		if !slices.Contains(have, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return errors.New(setup.List("missing users", missing))
	}
	return nil
}

func addUsers(ctx context.Context, root string) error {
	_, err := execx.OSRunner{}.Output(ctx, "", "systemd-sysusers", "--root="+root)
	return err
}
