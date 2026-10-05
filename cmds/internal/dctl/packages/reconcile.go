package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"
)

func Reconcile(ctx context.Context, u *ui.UI, dir string, run execx.Runner, all bool) error {
	l, err := Load(dir)
	if err != nil {
		return err
	}
	d, err := drift(ctx, run, l)
	if err != nil {
		return err
	}
	if u.JSON() {
		return u.Emit(d)
	}
	for _, mark := range []struct {
		flag  string
		names []string
	}{{"--asexplicit", d.Implicit}, {"--asdeps", d.Required}} {
		if len(mark.names) == 0 {
			continue
		}
		if err := run.Run(ctx, "", "sudo", slices.Concat([]string{"pacman", "-D", mark.flag}, mark.names)...); err != nil {
			return err
		}
	}
	d = d.marked()
	if all || !u.Can() {
		report(u, d)
		return nil
	}
	unlisted := unique(slices.Concat(d.Repo, d.AUR, d.Orphans))
	if len(d.Missing)+len(unlisted) == 0 {
		u.OK("package lists match the system")
		return nil
	}
	if err := fill(ctx, u, run, l, d.Missing); err != nil {
		return err
	}
	return prune(ctx, u, dir, run, unlisted)
}

func fill(ctx context.Context, u *ui.UI, run execx.Runner, l Lists, missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	checked, err := u.Checklist(fmt.Sprintf("Install %d listed packages?", len(missing)), missing)
	if errors.Is(err, ui.ErrCanceled) {
		return nil
	}
	if err != nil {
		return err
	}
	var picked []string
	for i, name := range missing {
		if checked[i] {
			picked = append(picked, name)
		}
	}
	local := set(slices.Values(l.Local))
	if names := absent(picked, local); len(names) > 0 {
		if err := install(ctx, run, names); err != nil {
			return err
		}
	}
	if names := slices.DeleteFunc(picked, func(name string) bool { return !local[name] }); len(names) > 0 {
		u.Warn("build local recipes with `makepkg -si` in packages/<name>: %s", strings.Join(names, " "))
	}
	return nil
}

func prune(ctx context.Context, u *ui.UI, dir string, run execx.Runner, names []string) error {
	if len(names) == 0 {
		return nil
	}
	checked, err := u.Checklist(fmt.Sprintf("Remove %d unlisted packages?", len(names)), names)
	if errors.Is(err, ui.ErrCanceled) {
		return nil
	}
	if err != nil {
		return err
	}
	var drop, kept []string
	for i, name := range names {
		if checked[i] {
			drop = append(drop, name)
		} else {
			kept = append(kept, name)
		}
	}
	if err := keep(ctx, u, dir, run, kept); err != nil {
		return err
	}
	if len(drop) == 0 {
		return nil
	}
	return run.Run(ctx, "", "sudo", slices.Concat([]string{"pacman", "-Rns"}, drop)...)
}

func keep(ctx context.Context, u *ui.UI, dir string, run execx.Runner, names []string) error {
	if len(names) == 0 {
		return nil
	}
	foreign, err := query(ctx, run, "-Qqm")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "extra.lst")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, name := range names {
		header := "# official"
		if slices.Contains(foreign, name) {
			header = "# aur"
		}
		if lines, err = insert(lines, header, name); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	u.OK("kept in extra.lst: %s", strings.Join(names, " "))
	return run.Run(ctx, "", "sudo", slices.Concat([]string{"pacman", "-D", "--asexplicit"}, names)...)
}

func insert(lines []string, header, name string) ([]string, error) {
	start := slices.IndexFunc(lines, func(line string) bool { return strings.TrimSpace(line) == header })
	if start < 0 {
		return nil, fmt.Errorf("no %q section", header)
	}
	at := start + 1
	for ; at < len(lines); at++ {
		entry, _, _ := strings.Cut(lines[at], "#")
		if entry = strings.TrimSpace(entry); entry == "" || entry > name {
			break
		}
	}
	return slices.Insert(lines, at, name), nil
}
