package packages

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
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
	return unique(names), s.Err()
}

type Lists struct {
	Base, AUR, Extra, Local []string
}

// Load requires base.lst, aur.lst, and extra.lst and uses local PKGBUILD directory names as package names.
func Load(dir string) (Lists, error) {
	base, berr := Read(filepath.Join(dir, "base.lst"))
	aur, aerr := Read(filepath.Join(dir, "aur.lst"))
	extra, eerr := Read(filepath.Join(dir, "extra.lst"))
	builds, gerr := filepath.Glob(filepath.Join(dir, "*", "PKGBUILD"))
	l := Lists{Base: base, AUR: aur, Extra: extra}
	for _, path := range builds {
		l.Local = append(l.Local, filepath.Base(filepath.Dir(path)))
	}
	return l, errors.Join(berr, aerr, eerr, gerr)
}

// Payload returns sorted, unique base, AUR, and local package names, excluding Extra.
func (l Lists) Payload() []string {
	return unique(slices.Concat(l.Base, l.AUR, l.Local))
}

func unique(names []string) []string {
	out := slices.Clone(names)
	slices.Sort(out)
	return slices.Compact(out)
}

func Upgrade(noconfirm bool, names ...string) []string {
	args := []string{"-Syu"}
	if noconfirm {
		args = append(args, "--noconfirm")
	}
	if len(names) > 0 {
		args = append(append(args, "--needed"), names...)
	}
	return args
}

func Stage(dir string, run execx.Runner) setup.Stage {
	return setup.Stage{Name: "packages", Root: true, Online: true, Items: []setup.Item{{
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
			official, other, err := missing(ctx, dir, run)
			if err != nil {
				return err
			}
			if len(official) > 0 {
				if err := run.Run(ctx, "", "pacman", Upgrade(true, official...)...); err != nil {
					return err
				}
			}
			if len(other) > 0 {
				return setup.Manual("pacman cannot install AUR or local packages: %s; build them with yay -S or makepkg -si in packages/<name>", strings.Join(other, " "))
			}
			return nil
		},
	}}}
}

func missing(ctx context.Context, dir string, run execx.Runner) (official, other []string, err error) {
	l, err := Load(dir)
	if err != nil {
		return nil, nil, err
	}
	have, err := installed(ctx, run)
	if err != nil {
		return nil, nil, err
	}
	return absent(l.Base, have), absent(unique(slices.Concat(l.AUR, l.Local)), have), nil
}

var coreDB = "/var/lib/pacman/sync/core.db"

func synced() error {
	_, err := os.Stat(coreDB)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no official sync databases (%s missing); run `dctl update pacman`", coreDB)
	}
	return err
}

func installed(ctx context.Context, run execx.Runner) (map[string]bool, error) {
	out, err := run.Output(ctx, "", "pacman", "-Qq")
	if err != nil {
		return nil, fmt.Errorf("pacman -Qq: %w", err)
	}
	return set(strings.FieldsSeq(out)), nil
}

func set(names iter.Seq[string]) map[string]bool {
	out := map[string]bool{}
	for name := range names {
		out[name] = true
	}
	return out
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
