package packages

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

func Read(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return names, nil
}

func Parse(r io.Reader) ([]string, error) {
	var names []string
	s := bufio.NewScanner(r)
	for n := 1; s.Scan(); n++ {
		line, _, _ := strings.Cut(s.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) > 1 {
			return nil, fmt.Errorf("line %d: %q: one package per line", n, strings.TrimSpace(line))
		}
		names = append(names, fields...)
	}
	return Unique(names), s.Err()
}

func Unique(names []string) []string {
	out := slices.Clone(names)
	slices.Sort(out)
	return slices.Compact(out)
}

func Group(dir string, offline bool, run execx.Runner) doctor.Group {
	return doctor.Group{Name: "packages", Root: true, Checks: []doctor.Check{{
		Name: "packages-installed",
		Check: func(ctx context.Context) error {
			official, other, err := missing(ctx, dir, run)
			if err != nil {
				return err
			}
			if len(official)+len(other) == 0 {
				return nil
			}
			return fmt.Errorf("missing %d official: %s; missing %d AUR or local: %s",
				len(official), strings.Join(official, " "), len(other), strings.Join(other, " "))
		},
		Fix: func(ctx context.Context) error {
			if offline {
				return doctor.Block("pacman -S needs the network; rerun without --offline")
			}
			official, other, err := missing(ctx, dir, run)
			if err != nil {
				return err
			}
			if len(official) > 0 {
				if err := synced(); err != nil {
					return err
				}
				if _, err := run.Run(ctx, "", "pacman", append([]string{"-S", "--needed", "--noconfirm"}, official...)...); err != nil {
					return err
				}
			}
			if len(other) > 0 {
				return doctor.Block("pacman cannot install AUR or local packages: %s; build them with yay -S or makepkg -si in packages/<name>", strings.Join(other, " "))
			}
			return nil
		},
	}}}
}

func missing(ctx context.Context, dir string, run execx.Runner) (official, other []string, err error) {
	base, err := Read(filepath.Join(dir, "base.lst"))
	if err != nil {
		return nil, nil, err
	}
	aur, err := Read(filepath.Join(dir, "aur.lst"))
	if err != nil {
		return nil, nil, err
	}
	local, err := Locals(dir)
	if err != nil {
		return nil, nil, err
	}
	have, err := installed(ctx, run)
	if err != nil {
		return nil, nil, err
	}
	return absent(base, have), absent(Unique(append(aur, local...)), have), nil
}

func Locals(dir string) ([]string, error) {
	builds, err := filepath.Glob(filepath.Join(dir, "*", "PKGBUILD"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, path := range builds {
		names = append(names, filepath.Base(filepath.Dir(path)))
	}
	return names, nil
}

var coreDB = "/var/lib/pacman/sync/core.db"

func synced() error {
	_, err := os.Stat(coreDB)
	if errors.Is(err, fs.ErrNotExist) {
		return doctor.Block("no official sync databases (%s missing); run `dctl update` as your user first", coreDB)
	}
	return err
}

func installed(ctx context.Context, run execx.Runner) (map[string]bool, error) {
	out, err := run.Output(ctx, "", "pacman", "-Qq")
	if err != nil {
		return nil, doctor.Block("pacman -Qq: %v", err)
	}
	have := map[string]bool{}
	for name := range strings.FieldsSeq(out) {
		have[name] = true
	}
	return have, nil
}

func absent(names []string, have map[string]bool) []string {
	var out []string
	for _, name := range names {
		if !have[name] {
			out = append(out, name)
		}
	}
	return out
}
