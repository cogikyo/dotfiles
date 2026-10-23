package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

const (
	bridgeURL  = "https://github.com/u-u-z/cursor-sdk-bridge.git"
	bridgeUnit = "cursor-bridge.service"
)

type Bridge struct {
	recipe string
	dir    string
	run    execx.Runner
}

func NewBridge(root paths.Root, run execx.Runner) Bridge {
	return Bridge{
		recipe: root.Packages("cursor-sdk-bridge"),
		dir:    filepath.Join(root.Home, ".local", "share", "cursor-sdk-bridge"),
		run:    run,
	}
}

func (b Bridge) Stage() setup.Stage {
	return setup.Stage{Name: "bridge", Items: []setup.Item{{
		Name:  "bridge-running",
		Check: b.Check,
		Fix:   b.Fix,
	}}}
}

func (b Bridge) Check(ctx context.Context) error {
	pin, err := b.pin()
	if err != nil {
		return err
	}
	if err := b.source(ctx, pin); err != nil {
		return err
	}
	if err := b.deps(ctx); err != nil {
		return err
	}
	return b.active(ctx)
}

func (b Bridge) Fix(ctx context.Context) error {
	pin, err := b.pin()
	if err != nil {
		return err
	}
	changed := b.source(ctx, pin) != nil
	if changed {
		if err := b.checkout(ctx, pin); err != nil {
			return err
		}
	}
	if changed || b.deps(ctx) != nil {
		if err := b.run.Run(ctx, b.dir, "npm", "ci", "--omit=dev", "--ignore-scripts", "--replace-registry-host=always", "--no-audit", "--no-fund"); err != nil {
			return err
		}
		changed = true
	}
	if !changed && b.active(ctx) == nil {
		return nil
	}
	if err := b.run.Run(ctx, "", "systemctl", "--user", "reset-failed", bridgeUnit); err != nil {
		return err
	}
	return b.run.Run(ctx, "", "systemctl", "--user", "restart", bridgeUnit)
}

func (b Bridge) pin() (string, error) {
	path := filepath.Join(b.recipe, "commit")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	pin := strings.TrimSpace(string(data))
	if len(pin) != 40 || strings.Trim(pin, "0123456789abcdef") != "" {
		return "", fmt.Errorf("%s: %q is not a full commit hash", path, pin)
	}
	return pin, nil
}

func (b Bridge) patch() string { return filepath.Join(b.recipe, "fast-suffix.patch") }

func (b Bridge) git(ctx context.Context, args ...string) (string, error) {
	return b.run.Output(ctx, b.dir, "git", append([]string{"--git-dir", filepath.Join(b.dir, ".git")}, args...)...)
}

func (b Bridge) source(ctx context.Context, pin string) error {
	if _, err := os.Stat(b.dir); err != nil {
		return err
	}
	head, err := b.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != pin {
		return fmt.Errorf("%s is at %.12s, want %.12s", b.dir, head, pin)
	}
	diff, err := b.git(ctx, "diff", "--no-color", "--no-ext-diff", "--no-textconv", "--default-prefix", "--unified=3", "--inter-hunk-context=0", "--diff-algorithm=myers", "--indent-heuristic", "HEAD")
	if err != nil {
		return err
	}
	patch, err := os.ReadFile(b.patch())
	if err != nil {
		return err
	}
	have, err := patchID(ctx, diff)
	if err != nil {
		return err
	}
	want, err := patchID(ctx, string(patch))
	if err != nil {
		return err
	}
	if have != want {
		return fmt.Errorf("%s changes differ from %s", b.dir, filepath.Base(b.patch()))
	}
	return nil
}

func patchID(ctx context.Context, diff string) (string, error) {
	if diff == "" {
		return "", nil
	}
	out, err := execx.OSRunner{Stdin: []byte(diff)}.Output(ctx, "", "git", "patch-id", "--stable")
	id, _, _ := strings.Cut(out, " ")
	return id, err
}

func (b Bridge) deps(ctx context.Context) error {
	if _, err := b.run.Output(ctx, b.dir, "npm", "ls", "--all", "--omit=dev"); err != nil {
		return fmt.Errorf("%s dependencies differ from package-lock.json: %w", b.dir, err)
	}
	return nil
}

func (b Bridge) active(ctx context.Context) error {
	state, err := b.run.Output(ctx, "", "systemctl", "--user", "is-active", bridgeUnit)
	if err != nil {
		return fmt.Errorf("%s is %s", bridgeUnit, state)
	}
	return nil
}

func (b Bridge) checkout(ctx context.Context, pin string) error {
	if err := b.run.Run(ctx, "", "git", "init", "--quiet", b.dir); err != nil {
		return err
	}
	if _, err := b.git(ctx, "cat-file", "-e", pin+"^{commit}"); err != nil {
		fetch := []string{"--git-dir", filepath.Join(b.dir, ".git"), "fetch", "--quiet", "--depth=1", bridgeURL, pin}
		if err := execx.Remote(b.run).Run(ctx, b.dir, "git", fetch...); err != nil {
			return err
		}
	}
	if _, err := b.git(ctx, "checkout", "--quiet", "--force", "--detach", pin); err != nil {
		return err
	}
	_, err := b.git(ctx, "apply", b.patch())
	return err
}
