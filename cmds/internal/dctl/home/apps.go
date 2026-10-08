package home

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

func librepods(r paths.Root) setup.Item {
	dirs := []string{filepath.Join(r.Home, ".config", "librepods"), filepath.Join(r.Home, ".local", "share", "librepods")}
	settings := filepath.Join(dirs[0], "app_settings.json")
	devices := filepath.Join(dirs[1], "devices.json")
	unit := r.Config("systemd", "user", "librepods.service")
	enabled := filepath.Join(r.Home, ".config", "systemd", "user", "hyprland-session.target.wants", "librepods.service")
	tracked := r.Config("systemd", "user", "hyprland-session.target.wants", "librepods.service")
	template := func() (map[string]any, error) {
		return readObject(r.Packages("librepods-max2", "app_settings.json"))
	}
	return setup.Item{
		Name: "home-librepods",
		Check: func(context.Context) error {
			want, err := template()
			if err != nil {
				return err
			}
			var errs []error
			if !same(enabled, unit) {
				errs = append(errs, fmt.Errorf("%s does not resolve to %s", enabled, unit))
			}
			for _, dir := range dirs {
				errs = append(errs, private(dir, fs.ModeDir|0o700))
			}
			errs = append(errs, private(settings, 0o600))
			if _, err := os.Lstat(devices); err == nil {
				errs = append(errs, private(devices, 0o600))
			}
			if have, err := readObject(settings); err == nil {
				for key, value := range want {
					if !reflect.DeepEqual(have[key], value) {
						errs = append(errs, fmt.Errorf("%s: %s differs from the managed value", settings, key))
					}
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
			return errors.Join(errs...)
		},
		Fix: func(context.Context) error {
			if err := relink(tracked, "../librepods.service"); err != nil {
				return err
			}
			want, err := template()
			if err != nil {
				return err
			}
			for _, dir := range dirs {
				if st, err := os.Lstat(dir); err == nil && !st.IsDir() {
					return fmt.Errorf("%s must be a directory, not a symlink or file", dir)
				}
				if err := os.MkdirAll(dir, 0o700); err != nil {
					return err
				}
				if err := os.Chmod(dir, 0o700); err != nil {
					return err
				}
			}
			have, err := readObject(settings)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				have = map[string]any{}
			case err != nil:
				return fmt.Errorf("refusing to overwrite: %w", err)
			}
			maps.Copy(have, want)
			data, err := json.MarshalIndent(have, "", "  ")
			if err != nil {
				return err
			}
			if err := writeAtomic(settings, append(data, '\n'), 0o600); err != nil {
				return err
			}
			if err := localFile(devices); errors.Is(err, fs.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
			return os.Chmod(devices, 0o600)
		},
	}
}

func relink(path, target string) error {
	if cur, err := os.Readlink(path); err == nil && cur == target {
		return nil
	}
	st, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case st.Mode()&fs.ModeSymlink == 0:
		return fmt.Errorf("refusing to replace %s: not a symlink", path)
	default:
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, path)
}

func readObject(path string) (map[string]any, error) {
	data, err := readLocal(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("%s is not a JSON object", path)
	}
	return obj, nil
}

func private(path string, want fs.FileMode) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if st.Mode() != want {
		return fmt.Errorf("%s has mode %v, want %v", path, st.Mode(), want)
	}
	return nil
}

func localFile(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file, not %v", path, st.Mode().Type())
	}
	return nil
}

func readLocal(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%s must be a regular file, not a symlink", path)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file, not %v", path, st.Mode().Type())
	}
	return io.ReadAll(f)
}

func same(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	rb, err := filepath.EvalSymlinks(b)
	return err == nil && ra == rb
}

func legacy(path, src string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode()&fs.ModeSymlink != 0 && same(path, src)
}

func seed(src, dst string) error {
	if !legacy(dst, src) {
		if err := localFile(dst); !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeAtomic(dst, data, 0o644)
}

func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".dctl-*")
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	err = errors.Join(err, out.Chmod(mode), out.Close())
	if err == nil {
		err = os.Rename(out.Name(), path)
	}
	if err != nil {
		_ = os.Remove(out.Name())
	}
	return err
}
