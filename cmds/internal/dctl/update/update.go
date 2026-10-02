package update

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/ui"
)

type Drift struct {
	Repo    []string `json:"repo"`
	AUR     []string `json:"aur"`
	Missing []string `json:"missing"`
	Orphans []string `json:"orphans"`
}

func Run(ctx context.Context, u *ui.UI, dir string, run execx.Runner) error {
	if os.Geteuid() == 0 {
		return errors.New("run dctl update as your user; yay elevates only the pacman transaction")
	}
	u.Step("yay -Syu")
	if _, err := run.Run(ctx, "", "yay", "-Syu"); err != nil {
		return err
	}
	listed, err := lists(dir)
	if err != nil {
		return err
	}
	d, err := drift(ctx, run, listed)
	if err != nil {
		return err
	}
	if u.JSON() {
		return u.Emit(d)
	}
	report(u, d)
	return nil
}

func report(u *ui.UI, d Drift) {
	if len(d.Repo)+len(d.AUR)+len(d.Missing)+len(d.Orphans) == 0 {
		u.OK("package lists match the system")
		return
	}
	for _, row := range []struct {
		what  string
		names []string
	}{
		{"explicit repo packages in no list (base.lst, extra.lst)", d.Repo},
		{"explicit foreign packages in no list (aur.lst, packages/*/PKGBUILD)", d.AUR},
		{"listed packages not installed", d.Missing},
		{"unlisted orphans", d.Orphans},
	} {
		if len(row.names) > 0 {
			u.Warn("%d %s", len(row.names), row.what)
			u.Dim("%s", strings.Join(row.names, " "))
		}
	}
	if len(d.Orphans) > 0 {
		u.Dim("remove with: yay -Rns %s", strings.Join(d.Orphans, " "))
	}
}

func lists(dir string) (map[string]bool, error) {
	listed := map[string]bool{}
	for _, name := range []string{"base.lst", "aur.lst", "extra.lst"} {
		names, err := packages.Read(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			listed[n] = true
		}
	}
	local, err := packages.Locals(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range local {
		listed[name] = true
	}
	return listed, nil
}

func drift(ctx context.Context, run execx.Runner, listed map[string]bool) (Drift, error) {
	query := func(flags string) ([]string, error) {
		out, err := run.Output(ctx, "", "pacman", flags)
		if err != nil && (flags == "-Qq" || out != "") {
			return nil, err
		}
		return packages.Unique(strings.Fields(out)), nil
	}
	all, err := query("-Qq")
	if err != nil {
		return Drift{}, err
	}
	native, err := query("-Qqen")
	if err != nil {
		return Drift{}, err
	}
	foreign, err := query("-Qqem")
	if err != nil {
		return Drift{}, err
	}
	orphans, err := query("-Qqdt")
	if err != nil {
		return Drift{}, err
	}
	installed := map[string]bool{}
	for _, name := range all {
		installed[name] = true
	}
	return classify(listed, installed, native, foreign, orphans), nil
}

func classify(listed, installed map[string]bool, native, foreign, orphans []string) Drift {
	return Drift{
		Repo:    absent(native, listed),
		AUR:     absent(foreign, listed),
		Missing: absent(slices.Sorted(maps.Keys(listed)), installed),
		Orphans: absent(orphans, listed),
	}
}

func absent(names []string, set map[string]bool) []string {
	var out []string
	for _, name := range names {
		if !set[name] {
			out = append(out, name)
		}
	}
	return out
}
