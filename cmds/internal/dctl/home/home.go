package home

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/paths"
)

func Group(r paths.Root) doctor.Group {
	scene := filepath.Join(r.Home, ".config", "obs-studio", "basic", "scenes", "Costello.json")
	return doctor.Group{Name: "home", Checks: []doctor.Check{
		{
			Name:  "home-links",
			Check: func(context.Context) error { return checkLinks(r) },
			Fix:   func(context.Context) error { return fixLinks(r, time.Now()) },
		},
		{
			Name:  "home-obs-scene",
			Check: func(context.Context) error { return checkScene(scene) },
			Fix: func(context.Context) error {
				return seedScene(r.Config("obs-studio", "basic", "scenes", "Costello.json"), scene)
			},
		},
	}}
}

type link struct{ src, dst string }

func links(r paths.Root) ([]link, error) {
	entries, err := os.ReadDir(r.Config())
	if err != nil {
		return nil, err
	}
	var out []link
	for _, e := range entries {
		if e.Name() == "firefox" || e.Name() == "obs-studio" {
			continue
		}
		out = append(out, link{r.Config(e.Name()), filepath.Join(r.Home, ".config", e.Name())})
	}
	profile := filepath.Join("obs-studio", "basic", "profiles", "Costello", "basic.ini")
	out = append(out,
		link{r.Config(profile), filepath.Join(r.Home, ".config", profile)},
		link{r.Config("zsh", "zshrc"), filepath.Join(r.Home, ".zshrc")},
		link{r.Config("zsh", "zshenv"), filepath.Join(r.Home, ".zshenv")},
	)
	bin, err := os.ReadDir(r.Bin())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range bin {
		if e.Type().IsRegular() {
			out = append(out, link{r.Bin(e.Name()), filepath.Join(r.Home, ".local", "bin", e.Name())})
		}
	}
	return out, nil
}

func (l link) ok() bool {
	src, err := filepath.EvalSymlinks(l.src)
	if err != nil {
		return false
	}
	dst, err := filepath.EvalSymlinks(l.dst)
	return err == nil && dst == src
}

func (l link) apply(dotfiles string, now time.Time) error {
	if l.ok() {
		return nil
	}
	if _, err := os.Stat(l.src); err != nil {
		return err
	}
	st, err := os.Lstat(l.dst)
	backup := l.dst + ".backup." + now.Format("20060102-150405")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		err = nil
	case err != nil:
		return err
	case st.Mode()&fs.ModeSymlink != 0:
		err = os.Remove(l.dst)
	case st.IsDir():
		managed, merr := managedDir(l.dst, dotfiles)
		if merr != nil {
			return merr
		}
		if !managed {
			return fmt.Errorf("refusing to replace %s: real directory may contain user data", l.dst)
		}
		err = os.Rename(l.dst, backup)
	default:
		err = os.Rename(l.dst, backup)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.dst), 0o755); err != nil {
		return err
	}
	return os.Symlink(l.src, l.dst)
}

func managedDir(dir, dotfiles string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	repo, err := filepath.Abs(dotfiles)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.Type()&fs.ModeSymlink == 0 {
			return false, nil
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			return false, nil
		}
		if real != repo && !strings.HasPrefix(real, repo+string(os.PathSeparator)) {
			return false, nil
		}
	}
	return true, nil
}

func checkLinks(r paths.Root) error {
	list, err := links(r)
	if err != nil {
		return err
	}
	var bad []string
	for _, l := range list {
		if !l.ok() {
			bad = append(bad, l.dst)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%d of %d links missing or wrong: %s", len(bad), len(list), strings.Join(bad, ", "))
	}
	return nil
}

func fixLinks(r paths.Root, now time.Time) error {
	list, err := links(r)
	if err != nil {
		return err
	}
	var errs []error
	for _, l := range list {
		errs = append(errs, l.apply(r.Dotfiles, now))
	}
	return errors.Join(errs...)
}

func checkScene(dst string) error {
	st, err := os.Lstat(dst)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", dst)
	}
	return nil
}

func seedScene(src, dst string) error {
	st, err := os.Lstat(dst)
	switch {
	case err == nil && st.Mode()&fs.ModeSymlink == 0:
		return nil
	case err == nil:
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".dctl-*")
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	err = errors.Join(err, out.Chmod(0o644), out.Close())
	if err == nil {
		err = os.Rename(out.Name(), dst)
	}
	if err != nil {
		_ = os.Remove(out.Name())
	}
	return err
}
