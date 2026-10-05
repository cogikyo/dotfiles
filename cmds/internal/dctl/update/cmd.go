package update

import (
	"bytes"
	"cmp"
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/gobuild"
	"dotfiles/cmds/internal/ui"
)

var (
	services = []string{"ewwd", "newtab"}
	// refusals matches the text protocol returned by hyprd's rebuildBlocked.
	refusals = []string{"error: full lock active", "error: opencode refresh job "}
)

func cmd(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner, all bool, only []string) error {
	lists, err := packages.Load(root.Packages())
	if err != nil {
		return err
	}
	err = commands(ctx, u, root, run, all, pick(binaries.Names, only))
	if errors.Is(err, ui.ErrCanceled) {
		return err
	}
	return errors.Join(err, recipes(ctx, u, root, run, all, pick(lists.Local, only)))
}

func pick(names, only []string) []string {
	if len(only) == 0 {
		return names
	}
	return slices.DeleteFunc(slices.Clone(names), func(n string) bool { return !slices.Contains(only, n) })
}

func commands(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner, all bool, names []string) error {
	if len(names) == 0 {
		return nil
	}
	dirty, err := run.Output(ctx, root.Dotfiles, "git", "status", "--porcelain", "--untracked-files=all", "--", "cmds")
	if err != nil {
		return err
	}
	if dirty != "" {
		if all {
			u.Warn("cmds/ has uncommitted changes; --all skips the dotfiles build")
			return nil
		}
		u.Warn("cmds/ has uncommitted changes; the build would include them")
		ok, err := u.Confirm("Build dotfiles commands from the working tree?")
		if err != nil || !ok {
			return err
		}
	}
	bin := filepath.Join(root.Home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(bin, ".dctl-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	args := slices.Concat(gobuild.Env, []string{"go", "build"}, gobuild.Flags, []string{"-o", tmp + "/"})
	for _, name := range names {
		args = append(args, "./cmd/"+name)
	}
	if err := run.Run(ctx, filepath.Join(root.Dotfiles, "cmds"), "env", args...); err != nil {
		return err
	}
	var errs []error
	changed := 0
	for _, name := range names {
		fresh, dst := filepath.Join(tmp, name), filepath.Join(bin, name)
		if same(fresh, dst) {
			continue
		}
		changed++
		if name == "hyprd" {
			// The daemon owns state preservation and lock checks around its binary swap.
			errs = append(errs, rebuild(ctx, u, run))
			continue
		}
		if err := os.Rename(fresh, dst); err != nil {
			errs = append(errs, err)
			continue
		}
		u.OK("replaced %s", dst)
		if slices.Contains(services, name) {
			_, err := run.Output(ctx, "", "systemctl", "--user", "restart", name+".service")
			errs = append(errs, err)
		}
	}
	if changed == 0 {
		u.OK("dotfiles commands unchanged")
	}
	return errors.Join(errs...)
}

func same(a, b string) bool {
	x, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	y, err := os.ReadFile(b)
	return err == nil && bytes.Equal(x, y)
}

func rebuild(ctx context.Context, u *ui.UI, run execx.Runner) error {
	out, err := run.Output(ctx, "", "hyprd", "rebuild")
	if err == nil {
		u.OK("hyprd: %s", out)
		return nil
	}
	if strings.Contains(err.Error(), "hyprd: daemon not running") || slices.ContainsFunc(refusals, func(r string) bool { return strings.HasPrefix(out, r) }) {
		u.Warn("hyprd rebuild skipped: %s", strings.TrimPrefix(cmp.Or(out, "daemon not running"), "error: "))
		return nil
	}
	return fmt.Errorf("hyprd rebuild: %w", err)
}

// recipes rebuilds installed packages/<name> recipes whose PKGBUILD version differs from the installed one.
func recipes(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner, all bool, names []string) error {
	if len(names) == 0 {
		return nil
	}
	out, err := run.Output(ctx, "", "pacman", "-Q")
	if err != nil {
		return err
	}
	installed := map[string]string{}
	for l := range strings.Lines(out) {
		if name, ver, ok := strings.Cut(strings.TrimSpace(l), " "); ok {
			installed[name] = ver
		}
	}
	var errs []error
	var current, absent []string
	for _, name := range names {
		have, ok := installed[name]
		if !ok {
			absent = append(absent, name)
			continue
		}
		dir := filepath.Join(root.Packages(), name)
		info, err := run.Output(ctx, dir, "makepkg", "--printsrcinfo")
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		want := version(info)
		if have == want {
			current = append(current, name)
			continue
		}
		u.Info("makepkg %s %s → %s", name, have, want)
		args := []string{"-sfi"}
		if all {
			args = append(args, "--noconfirm")
		}
		if err := run.Run(ctx, dir, "makepkg", args...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	if len(current) > 0 {
		u.OK("local packages current: %s", strings.Join(current, ", "))
	}
	if len(absent) > 0 {
		u.Info("skipped, not installed: %s", strings.Join(absent, ", "))
	}
	return errors.Join(errs...)
}

// version reads epoch:pkgver-pkgrel from .SRCINFO, where the pkgbase section comes first.
func version(srcinfo string) string {
	fields := map[string]string{}
	for l := range strings.Lines(srcinfo) {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), " = "); ok && fields[k] == "" {
			fields[k] = v
		}
	}
	v := fields["pkgver"] + "-" + fields["pkgrel"]
	if e := fields["epoch"]; e != "" && e != "0" {
		v = e + ":" + v
	}
	return v
}

func tools(ctx context.Context, u *ui.UI, run execx.Runner) error {
	dir, err := run.Output(ctx, "", "go", "env", "GOBIN")
	if err != nil {
		return err
	}
	if dir == "" {
		gopath, err := run.Output(ctx, "", "go", "env", "GOPATH")
		if err != nil {
			return err
		}
		dir = filepath.Join(filepath.SplitList(gopath)[0], "bin")
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	var local []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := buildinfo.ReadFile(filepath.Join(dir, e.Name()))
		// A module-proxy checksum distinguishes installed releases from local checkout builds.
		if err != nil || !strings.HasPrefix(info.Main.Sum, "h1:") {
			local = append(local, e.Name())
			continue
		}
		u.Info("go install %s@latest", info.Path)
		if err := run.Run(ctx, "", "go", "install", info.Path+"@latest"); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
		}
	}
	if len(local) > 0 {
		u.Info("skipped, not from a module proxy: %s", strings.Join(local, ", "))
	}
	return errors.Join(errs...)
}
