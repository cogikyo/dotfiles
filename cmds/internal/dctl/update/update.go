package update

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/ui"
)

type Step struct {
	Name string
	Plan string
	Run  func(context.Context) error
}

func Steps(u *ui.UI, root paths.Root, run execx.Runner, all bool, only []string) []Step {
	noconfirm := func(args ...string) []string {
		if all {
			return append(args, "--noconfirm")
		}
		return args
	}
	prompting := run
	if !all {
		prompting = execx.Interactive(run)
	}
	return []Step{
		{
			Name: "pacman",
			Plan: "official packages",
			Run: func(ctx context.Context) error {
				return drawn(prompting.Run(ctx, "", "sudo", slices.Concat([]string{"pacman"}, packages.Upgrade(all))...))
			},
		},
		{
			Name: "aur",
			Plan: "AUR packages, excluding local PKGBUILDs",
			Run: func(ctx context.Context) error {
				l, err := packages.Load(root.Packages())
				if err != nil {
					return err
				}
				args := []string{"-Sua"}
				if len(l.Local) > 0 {
					args = append(args, "--ignore", strings.Join(l.Local, ","))
				}
				return drawn(prompting.Run(ctx, "", "yay", noconfirm(args...)...))
			},
		},
		{
			Name: "packages",
			Plan: "reconcile package lists and installed packages",
			Run: func(ctx context.Context) error {
				return packages.Reconcile(ctx, u, root.Packages(), run, all)
			},
		},
		{
			Name: "repos",
			Plan: "fast-forward clean checkouts from packages/repos.lst, except ~/dotfiles",
			Run: func(ctx context.Context) error {
				return repos.Update(ctx, u, root, run)
			},
		},
		{
			Name: "cmd",
			Plan: "build dotfiles commands; rebuild local packages when versions differ",
			Run: func(ctx context.Context) error {
				return cmd(ctx, u, root, run, all, only)
			},
		},
		{
			Name: "go",
			Plan: "Go tools installed from module releases",
			Run: func(ctx context.Context) error {
				return tools(ctx, u, run)
			},
		},
		{
			Name: "rust",
			Run: func(ctx context.Context) error {
				if _, err := exec.LookPath("rustup"); err != nil {
					u.Info("skipped: rustup is not installed")
					return nil
				}
				return drawn(run.Run(ctx, "", "rustup", "update"))
			},
		},
	}
}

func Select(all []Step, names []string) ([]Step, error) {
	var known []string
	for _, s := range all {
		known = append(known, s.Name)
	}
	for _, name := range names {
		if !slices.Contains(known, name) {
			return nil, fmt.Errorf("unknown step %q (known: %s)", name, strings.Join(known, ", "))
		}
	}
	if len(names) == 0 {
		return all, nil
	}
	return slices.DeleteFunc(slices.Clone(all), func(s Step) bool { return !slices.Contains(names, s.Name) }), nil
}

func Run(ctx context.Context, u *ui.UI, steps []Step, ask bool) error {
	var done, skipped, failed []string
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		u.Begin(s.Name, s.Plan)
		if ask {
			ok, err := u.Proceed(fmt.Sprintf("Run %s?", s.Name))
			if err != nil {
				return err
			}
			if !ok {
				skipped = append(skipped, s.Name)
				u.End(ui.Info, "skipped")
				continue
			}
		}
		if err := s.Run(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, ui.ErrCanceled) {
				return err
			}
			failed = append(failed, s.Name)
			if _, ok := errors.AsType[shown](err); ok {
				u.End(ui.Err, "")
				continue
			}
			u.End(ui.Err, "%s", err)
			continue
		}
		done = append(done, s.Name)
		u.End(ui.OK, "")
	}
	u.Begin("summary", "")
	if len(done) > 0 {
		u.OK("finished: %s", strings.Join(done, ", "))
	}
	if len(skipped) > 0 {
		u.Info("skipped: %s", strings.Join(skipped, ", "))
	}
	if len(failed) > 0 {
		u.Info("retry: `dctl update %s`", strings.Join(failed, " "))
		return errors.New("update failed: " + strings.Join(failed, ", "))
	}
	u.End(ui.OK, "")
	return nil
}

type shown struct{ error }

func (s shown) Unwrap() error { return s.error }

func drawn(err error) error {
	if err == nil {
		return nil
	}
	return shown{err}
}
