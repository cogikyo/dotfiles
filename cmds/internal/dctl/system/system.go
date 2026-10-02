package system

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

const preset = "etc/systemd/system-preset/10-dotfiles.preset"

func Group(root, overlay string) doctor.Group {
	return doctor.Group{Name: "system", Root: true, Checks: []doctor.Check{
		{
			Name:  "system-files",
			Check: func(context.Context) error { return check(root, overlay) },
			Fix:   func(context.Context) error { return apply(root, overlay) },
		},
		{
			Name:  "system-units",
			Check: func(ctx context.Context) error { return checkUnits(ctx, root, overlay) },
			Fix:   func(ctx context.Context) error { return enable(ctx, root, overlay) },
		},
		{
			Name:  "system-resolv-conf",
			Check: func(context.Context) error { return checkResolv(root) },
			Fix:   func(context.Context) error { return linkResolv(root) },
		},
	}}
}

type file struct {
	rel  string
	mode fs.FileMode
}

func files(overlay string) ([]file, error) {
	var out []file
	err := fs.WalkDir(os.DirFS(overlay), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("overlay %s: only regular files are allowed", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		out = append(out, file{rel: rel, mode: mode})
		return nil
	})
	return out, err
}

func (f file) drift(want []byte, root *os.Root) (string, error) {
	st, err := root.Lstat(f.rel)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "missing", nil
	case err != nil:
		return "", err
	case !st.Mode().IsRegular():
		return "not a regular file", nil
	case st.Mode().Perm() != f.mode:
		return fmt.Sprintf("mode %04o, want %04o", st.Mode().Perm(), f.mode), nil
	}
	got, err := root.ReadFile(f.rel)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(got, want) {
		return "content differs", nil
	}
	return "", nil
}

func check(rootDir, overlay string) error {
	list, err := files(overlay)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return doctor.Block("cannot inspect %s: %v", rootDir, err)
	}
	defer root.Close()
	var drift, unknown []string
	for _, f := range list {
		want, err := os.ReadFile(filepath.Join(overlay, f.rel))
		if err != nil {
			return err
		}
		reason, err := f.drift(want, root)
		switch {
		case errors.Is(err, fs.ErrPermission):
			unknown = append(unknown, f.rel+" (needs root to verify)")
		case err != nil:
			unknown = append(unknown, f.rel+" ("+err.Error()+")")
		case reason != "":
			drift = append(drift, f.rel+" ("+reason+")")
		}
	}
	blind := fmt.Sprintf("cannot inspect %d of %d files: %s", len(unknown), len(list), strings.Join(unknown, ", "))
	switch {
	case len(drift) > 0 && len(unknown) > 0:
		return fmt.Errorf("%d of %d files differ: %s; %s", len(drift), len(list), strings.Join(drift, ", "), blind)
	case len(drift) > 0:
		return fmt.Errorf("%d of %d files differ: %s", len(drift), len(list), strings.Join(drift, ", "))
	case len(unknown) > 0:
		return doctor.Block("%s", blind)
	}
	return nil
}

func apply(rootDir, overlay string) error {
	list, err := files(overlay)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range list {
		data, err := os.ReadFile(filepath.Join(overlay, f.rel))
		if err != nil {
			return err
		}
		reason, err := f.drift(data, root)
		if err != nil {
			return fmt.Errorf("%s: %w", f.rel, err)
		}
		if reason == "" {
			continue
		}
		if err := write(root, f.rel, data, f.mode); err != nil {
			return fmt.Errorf("%s: %w", f.rel, err)
		}
	}
	return nil
}

func write(root *os.Root, rel string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(rel)
	if err := root.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if st, err := root.Lstat(rel); err == nil && st.IsDir() {
		return errors.New("target is a directory")
	}
	tmp := filepath.Join(dir, "."+filepath.Base(rel)+".dctl-"+rand.Text()[:8])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	err = errors.Join(err, f.Chmod(mode), f.Sync(), f.Close())
	if err == nil {
		err = root.Rename(tmp, rel)
	}
	if err != nil {
		_ = root.Remove(tmp)
	}
	return err
}

func units(overlay string) ([]string, error) {
	f, err := os.Open(filepath.Join(overlay, preset))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parsePreset(f)
}

func parsePreset(r io.Reader) ([]string, error) {
	var out []string
	s := bufio.NewScanner(r)
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "enable" {
			return nil, fmt.Errorf("preset line %d: want \"enable <unit>\", got %q", n, line)
		}
		if strings.ContainsAny(fields[1], "*?[") {
			return nil, fmt.Errorf("preset line %d: globs are not supported: %s", n, fields[1])
		}
		out = append(out, fields[1])
	}
	return out, s.Err()
}

func checkUnits(ctx context.Context, root, overlay string) error {
	list, err := units(overlay)
	if err != nil || len(list) == 0 {
		return err
	}
	res, err := execx.OSRunner{}.Run(ctx, "", "systemctl", append([]string{"--root=" + root, "is-enabled"}, list...)...)
	states := strings.Split(res.Stdout, "\n")
	if len(states) != len(list) {
		return doctor.Block("systemctl is-enabled: %v: %s", err, res.Stderr)
	}
	var off []string
	for i, unit := range list {
		if states[i] != "enabled" {
			off = append(off, unit+" ("+states[i]+")")
		}
	}
	if len(off) > 0 {
		return fmt.Errorf("not enabled: %s", strings.Join(off, ", "))
	}
	return nil
}

func enable(ctx context.Context, root, overlay string) error {
	list, err := units(overlay)
	if err != nil || len(list) == 0 {
		return err
	}
	res, err := execx.OSRunner{}.Run(ctx, "", "systemctl", append([]string{"--root=" + root, "enable"}, list...)...)
	if err != nil {
		return fmt.Errorf("%w: %s", err, res.Stderr)
	}
	return nil
}
