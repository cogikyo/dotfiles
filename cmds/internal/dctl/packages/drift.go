package packages

import (
	"context"
	"errors"
	"maps"
	"os/exec"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"
)

type Drift struct {
	Repo    []string `json:"repo"`
	AUR     []string `json:"aur"`
	Missing []string `json:"missing"`
	Orphans []string `json:"orphans"`
}

func Report(ctx context.Context, u *ui.UI, dir string, run execx.Runner) error {
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
			u.Detail("%s", strings.Join(row.names, " "))
		}
	}
	if len(d.Orphans) > 0 {
		u.Detail("remove with `yay -Rns %s`", strings.Join(d.Orphans, " "))
	}
}

func drift(ctx context.Context, run execx.Runner, l Lists) (Drift, error) {
	query := func(flags string) ([]string, error) {
		out, err := run.Output(ctx, "", "pacman", flags)
		if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
			return nil, nil
		}
		return unique(strings.Fields(out)), err
	}
	have, herr := installed(ctx, run)
	native, nerr := query("-Qqen")
	foreign, ferr := query("-Qqem")
	orphans, oerr := query("-Qqdt")
	if err := errors.Join(herr, nerr, ferr, oerr); err != nil {
		return Drift{}, err
	}
	listed := set(slices.Values(slices.Concat(l.Base, l.AUR, l.Extra, l.Local)))
	return classify(listed, have, native, foreign, orphans), nil
}

func classify(listed, have map[string]bool, native, foreign, orphans []string) Drift {
	return Drift{
		Repo:    absent(native, listed),
		AUR:     absent(foreign, listed),
		Missing: absent(slices.Sorted(maps.Keys(listed)), have),
		Orphans: absent(orphans, listed),
	}
}
