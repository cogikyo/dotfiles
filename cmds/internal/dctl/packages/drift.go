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
	Repo     []string `json:"repo"`
	AUR      []string `json:"aur"`
	Missing  []string `json:"missing"`
	Orphans  []string `json:"orphans"`
	Implicit []string `json:"implicit"`
	Required []string `json:"required"`
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
	if len(d.Repo)+len(d.AUR)+len(d.Missing)+len(d.Orphans)+len(d.Implicit) == 0 {
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
		{"listed packages installed as dependencies", d.Implicit},
		{"unlisted orphans", d.Orphans},
	} {
		if len(row.names) > 0 {
			u.Warn("%d %s", len(row.names), row.what)
			u.Detail("%s", strings.Join(row.names, " "))
		}
	}
	unlisted := slices.Concat(d.Repo, d.AUR)
	remove := slices.Sorted(slices.Values(slices.Concat(absent(unlisted, set(slices.Values(d.Required))), d.Orphans)))
	if len(d.Implicit)+len(d.Required)+len(remove) == 0 {
		return
	}
	u.Info("reconcile in this order")
	for _, fix := range []struct {
		what, flags string
		names       []string
	}{
		{"keep listed", "-D --asexplicit", d.Implicit},
		{"demote unlisted but required", "-D --asdeps", d.Required},
		{"remove unlisted", "-Rns", remove},
	} {
		if len(fix.names) > 0 {
			u.Detail("%s: `sudo pacman %s %s`", fix.what, fix.flags, strings.Join(fix.names, " "))
		}
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
	deps, derr := query("-Qqd")
	leaves, lerr := query("-Qqett")
	if err := errors.Join(herr, nerr, ferr, oerr, derr, lerr); err != nil {
		return Drift{}, err
	}
	listed := set(slices.Values(slices.Concat(l.Base, l.AUR, l.Extra, l.Local)))
	return classify(listed, have, native, foreign, orphans, deps, leaves), nil
}

func classify(listed, have map[string]bool, native, foreign, orphans, deps, leaves []string) Drift {
	repo, aur := absent(native, listed), absent(foreign, listed)
	return Drift{
		Repo:     repo,
		AUR:      aur,
		Missing:  absent(slices.Sorted(maps.Keys(listed)), have),
		Orphans:  absent(orphans, listed),
		Implicit: slices.DeleteFunc(slices.Clone(deps), func(name string) bool { return !listed[name] }),
		Required: slices.Sorted(slices.Values(absent(slices.Concat(repo, aur), set(slices.Values(leaves))))),
	}
}
