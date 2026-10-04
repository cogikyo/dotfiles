package home

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

var dirs = []string{"downloads", "documents", "media/screenshots", "media/recordings", "media/images", "media/gifs", "agents"}

func Stage(r paths.Root, run execx.Runner) setup.Stage {
	sshDir := filepath.Join(r.Home, ".ssh")
	keys := linkCheck("home-ssh-keys", r.Dotfiles, "", func() ([]link, error) {
		pubs, err := filepath.Glob(r.Share("ssh", "*.pub"))
		if err != nil {
			return nil, err
		}
		var out []link
		for _, src := range pubs {
			out = append(out, link{src, filepath.Join(sshDir, filepath.Base(src))})
		}
		return out, nil
	})
	linkKeys := keys.Fix
	keys.Fix = func(ctx context.Context) error {
		if err := os.Mkdir(sshDir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		return linkKeys(ctx)
	}
	zoom := filepath.Join(r.Home, ".config", "zoomus.conf")
	return setup.Stage{Name: "home", Items: slices.Concat(obs(r), []setup.Item{
		linkCheck("home-links", r.Dotfiles, "", func() ([]link, error) { return links(r) }),
		keys,
		{
			Name: "home-dirs",
			Check: func(context.Context) error {
				var gone []string
				for _, d := range dirs {
					if st, err := os.Stat(filepath.Join(r.Home, d)); err != nil || !st.IsDir() {
						gone = append(gone, "~/"+d)
					}
				}
				if len(gone) > 0 {
					return errors.New(setup.List("missing", gone))
				}
				return nil
			},
			Fix: func(context.Context) error {
				var errs []error
				for _, d := range dirs {
					errs = append(errs, os.MkdirAll(filepath.Join(r.Home, d), 0o755))
				}
				return errors.Join(errs...)
			},
		},
		{
			Name:  "home-zoom",
			Check: func(context.Context) error { return localFile(zoom) },
			Fix:   func(context.Context) error { return seed(r.Share("zoom", "zoomus.conf"), zoom) },
		},
		librepods(r),
		{
			Name: "home-shell",
			Check: func(ctx context.Context) error {
				entry, err := run.Output(ctx, "", "getent", "passwd", strconv.Itoa(os.Getuid()))
				if err != nil {
					return fmt.Errorf("getent passwd: %w", err)
				}
				fields := strings.Split(entry, ":")
				if len(fields) < 7 {
					return fmt.Errorf("unexpected passwd entry for uid %d", os.Getuid())
				}
				if filepath.Base(fields[6]) != "zsh" {
					return setup.Manual("login shell is %s, want zsh: run chsh -s /usr/bin/zsh", fields[6])
				}
				return nil
			},
		},
	})}
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
		link{r.Share("fonts"), filepath.Join(r.Home, ".local", "share", "fonts")},
	)
	apps, err := filepath.Glob(r.Share("applications", "*.desktop"))
	if err != nil {
		return nil, err
	}
	for _, src := range apps {
		out = append(out, link{src, filepath.Join(r.Home, ".local", "share", "applications", filepath.Base(src))})
	}
	return out, nil
}

func linkCheck(name, dotfiles, note string, links func() ([]link, error)) setup.Item {
	return setup.Item{
		Name: name,
		Check: func(context.Context) error {
			list, err := links()
			if err != nil {
				return err
			}
			var bad []string
			for _, l := range list {
				if !l.ok() {
					bad = append(bad, l.dst)
				}
			}
			if len(bad) == 0 {
				return nil
			}
			msg := setup.List(fmt.Sprintf("%d of %d links missing or wrong", len(bad), len(list)), bad)
			if note != "" {
				msg += "\n" + note
			}
			return errors.New(msg)
		},
		Fix: func(context.Context) error {
			list, err := links()
			if err != nil {
				return err
			}
			var errs []error
			for _, l := range list {
				errs = append(errs, l.apply(dotfiles, time.Now()))
			}
			return errors.Join(errs...)
		},
	}
}

func (l link) ok() bool {
	return same(l.dst, l.src)
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
