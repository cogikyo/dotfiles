package packages

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

func Brew(path string, run execx.Runner) setup.Stage {
	return setup.Stage{Name: "packages", Items: []setup.Item{{
		Name: "packages-installed",
		Check: func(ctx context.Context) error {
			formulae, casks, err := brewMissing(ctx, path, run)
			if err != nil {
				return err
			}
			if len(formulae)+len(casks) == 0 {
				return nil
			}
			return fmt.Errorf("missing Homebrew packages — formulae (%d): %s; casks (%d): %s",
				len(formulae), strings.Join(formulae, " "), len(casks), strings.Join(casks, " "))
		},
		Fix: func(ctx context.Context) error {
			formulae, casks, err := brewMissing(ctx, path, run)
			if err != nil {
				return err
			}
			if len(formulae) > 0 {
				if err := run.Run(ctx, "", "brew", slices.Concat([]string{"install", "--formula"}, formulae)...); err != nil {
					return err
				}
			}
			if len(casks) > 0 {
				return run.Run(ctx, "", "brew", slices.Concat([]string{"install", "--cask"}, casks)...)
			}
			return nil
		},
	}}}
}

func brewMissing(ctx context.Context, path string, run execx.Runner) (formulae, casks []string, err error) {
	names, err := Read(path)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range names {
		if cask, ok := strings.CutPrefix(name, "cask:"); ok {
			casks = append(casks, cask)
		} else {
			formulae = append(formulae, name)
		}
	}
	if formulae, err = brewAbsent(ctx, run, "--formula", formulae); err != nil {
		return nil, nil, err
	}
	casks, err = brewAbsent(ctx, run, "--cask", casks)
	return formulae, casks, err
}

func brewAbsent(ctx context.Context, run execx.Runner, kind string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out, err := run.Output(ctx, "", "brew", "list", kind, "-1")
	if err != nil {
		return nil, fmt.Errorf("brew list %s: %w", kind, err)
	}
	return absent(names, set(strings.FieldsSeq(out))), nil
}
