package binaries

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
)

var Names = []string{"dctl", "hyprd", "ewwd", "newtab", "src"}

func Group(r paths.Root, offline bool, run execx.Runner) doctor.Group {
	bin := filepath.Join(r.Home, ".local", "bin")
	return doctor.Group{Name: "binaries", Checks: []doctor.Check{
		{
			Name: "binaries-installed",
			Check: func(context.Context) error {
				var gone []string
				for _, name := range absent(bin) {
					gone = append(gone, filepath.Join(bin, name))
				}
				if len(gone) > 0 {
					return fmt.Errorf("missing or not executable: %s; a running hyprd needs `hyprd rebuild` after the build", strings.Join(gone, ", "))
				}
				return nil
			},
			Fix: func(ctx context.Context) error {
				if offline {
					return doctor.Block("go build may download modules; rerun without --offline")
				}
				if _, err := exec.LookPath("go"); err != nil {
					return doctor.Block("go not found; the ISO installs the binaries, or install go via the extra group")
				}
				var errs []error
				for _, name := range absent(bin) {
					_, err := run.Run(ctx, filepath.Join(r.Dotfiles, "cmds"), "go", "build", "-o", filepath.Join(bin, name), "./cmd/"+name)
					errs = append(errs, err)
				}
				return errors.Join(errs...)
			},
		},
		{
			Name: "binaries-eww-shadow",
			Check: func(context.Context) error {
				stale := filepath.Join(bin, "eww")
				if _, err := os.Lstat(stale); errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return fmt.Errorf("%s shadows the packaged eww; delete it", stale)
			},
		},
	}}
}

func absent(bin string) []string {
	var out []string
	for _, name := range Names {
		st, err := os.Stat(filepath.Join(bin, name))
		if err != nil || st.IsDir() || st.Mode()&0o111 == 0 {
			out = append(out, name)
		}
	}
	return out
}
