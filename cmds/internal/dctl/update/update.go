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
	return []Step{
		{
			Name: "pacman",
			Plan: "official repos via sudo pacman -Syu, then report package-list drift",
			Run: func(ctx context.Context) error {
				if err := run.Run(ctx, "", "sudo", noconfirm("pacman", "-Syu")...); err != nil {
					return err
				}
				return packages.Report(ctx, u, root.Packages(), run)
			},
		},
		{
			Name: "aur",
			Plan: "AUR packages via yay -Sua",
			Run: func(ctx context.Context) error {
				return run.Run(ctx, "", "yay", noconfirm("-Sua")...)
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
			Plan: "dotfiles commands and packages/ PKGBUILDs, rebuilt when changed",
			Run: func(ctx context.Context) error {
				return cmd(ctx, u, root, run, all, only)
			},
		},
		{
			Name: "go",
			Plan: "go install @latest for module-proxy tools in GOBIN",
			Run: func(ctx context.Context) error {
				return tools(ctx, u, run)
			},
		},
		{
			Name: "rust",
			Plan: "rustup update",
			Run: func(ctx context.Context) error {
				if _, err := exec.LookPath("rustup"); err != nil {
					u.Dim("rustup is not installed")
					return nil
				}
				return run.Run(ctx, "", "rustup", "update")
			},
		},
		{
			Name: "firmware",
			Plan: "device firmware (BIOS, SSD, USB) from LVFS via fwupd; flashes only after asking",
			Run: func(ctx context.Context) error {
				return firmware(ctx, u, run, all)
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
	var names, failed []string
	for _, s := range steps {
		names = append(names, s.Name)
	}
	u.Open("update %s", strings.Join(names, ", "))
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		u.Section(s.Name, s.Plan)
		if ask {
			ok, err := u.Proceed(fmt.Sprintf("Run %s?", s.Name))
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}
		if err := s.Run(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			u.Row(ui.Err, err.Error())
			failed = append(failed, s.Name)
			continue
		}
		u.OK("%s done", s.Name)
	}
	if len(failed) > 0 {
		return errors.New("update failed: " + strings.Join(failed, ", "))
	}
	u.Close(ui.OK, "update done")
	return nil
}
