package update

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

const plugin = "caret"

func plugins(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner, dirty bool) error {
	for _, tool := range []string{"pkg-config", "make", cmp.Or(os.Getenv("CXX"), "g++")} {
		if _, err := exec.LookPath(tool); err != nil {
			u.Info("skipped: %s is not installed", tool)
			return nil
		}
	}
	// Keep these pkg-config checks aligned with PKGS in cmds/plugins/caret/Makefile.
	installed, err := run.Output(ctx, "", "pkg-config", "--modversion", "hyprland")
	if err != nil {
		u.Info("skipped: Hyprland headers are not installed")
		return nil
	}
	if _, err := run.Output(ctx, "", "pkg-config", "--exists", "pixman-1", "libdrm"); err != nil {
		u.Info("skipped: pixman and libdrm headers are not installed")
		return nil
	}
	src := filepath.Join("cmds", "plugins", plugin)
	status, err := run.Output(ctx, root.Dotfiles, "git", "status", "--porcelain", "--untracked-files=all", "--", src)
	if err != nil {
		return err
	}
	if status != "" {
		if !dirty {
			u.Warn("%s has uncommitted changes; pass --dirty to build them", src)
			return nil
		}
		u.Warn("%s has uncommitted changes; the build includes them", src)
	}

	dir := filepath.Join(root.Home, ".local", "lib", "hyprland")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dir, ".dctl-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	fresh := filepath.Join(tmp, plugin+".so")
	err = u.Spin(ctx, fmt.Sprintf("build %s plugin for Hyprland %s", plugin, installed), func(ctx context.Context) error {
		_, err := run.Output(ctx, filepath.Join(root.Dotfiles, src), "make", "OUT="+fresh)
		return err
	})
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, fmt.Sprintf("%s-%s.so", plugin, installed))
	if same(fresh, dst) {
		u.OK("%s plugin for Hyprland %s unchanged", plugin, installed)
	} else {
		if err := os.Rename(fresh, dst); err != nil {
			return err
		}
		u.OK("replaced %s", dst)
	}

	running, ok := runningHyprland(ctx, run)
	if !ok {
		return nil
	}
	if running != installed {
		u.Info("Hyprland %s is running; %s loads after a Hyprland restart", running, filepath.Base(dst))
	}
	stale, err := filepath.Glob(filepath.Join(dir, plugin+"-*.so"))
	if err != nil {
		return err
	}
	for _, path := range stale {
		version := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), plugin+"-"), ".so")
		if version == installed || version == running {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		u.OK("removed %s", path)
	}
	return nil
}

func runningHyprland(ctx context.Context, run execx.Runner) (string, bool) {
	out, err := run.Output(ctx, "", "hyprctl", "version", "-j")
	if err != nil {
		return "", false
	}
	var v struct {
		Version string `json:"version"`
	}
	if json.Unmarshal([]byte(out), &v) != nil || v.Version == "" {
		return "", false
	}
	return v.Version, true
}
