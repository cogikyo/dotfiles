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

func (d Drift) marked() Drift {
	required := set(slices.Values(d.Required))
	return Drift{Repo: absent(d.Repo, required), AUR: absent(d.AUR, required), Missing: d.Missing, Orphans: d.Orphans}
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
	u.Info("run `dctl update packages` interactively to install or remove them")
}

func drift(ctx context.Context, run execx.Runner, l Lists) (Drift, error) {
	have, herr := installed(ctx, run)
	native, nerr := query(ctx, run, "-Qqen")
	foreign, ferr := query(ctx, run, "-Qqem")
	orphans, oerr := query(ctx, run, "-Qqdt")
	deps, derr := query(ctx, run, "-Qqd")
	leaves, lerr := query(ctx, run, "-Qqet")
	if err := errors.Join(herr, nerr, ferr, oerr, derr, lerr); err != nil {
		return Drift{}, err
	}
	listed := set(slices.Values(slices.Concat(l.Base, l.AUR, l.Extra, l.Local)))
	return classify(listed, have, native, foreign, orphans, deps, leaves), nil
}

func query(ctx context.Context, run execx.Runner, flags string) ([]string, error) {
	out, err := run.Output(ctx, "", "pacman", flags)
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		return nil, nil
	}
	return unique(strings.Fields(out)), err
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
