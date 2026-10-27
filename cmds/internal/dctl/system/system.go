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
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/machine"
)

const preset = "etc/systemd/system-preset/10-dotfiles.preset"

// Preset units may be oneshots or condition-skipped, so need not stay active.
var running = []string{"sshd.service"}

func Stage(root, overlay string) setup.Stage {
	return setup.Stage{Name: "system", Root: true, Items: []setup.Item{
		{
			Name:  "system-files",
			Check: func(context.Context) error { return check(root, overlay) },
			Fix:   func(context.Context) error { return apply(root, overlay) },
		},
		{
			Name:  "system-users",
			Check: func(context.Context) error { return checkUsers(root, overlay) },
			Fix:   func(ctx context.Context) error { return addUsers(ctx, root) },
		},
		{
			Name:  "system-units",
			Check: func(ctx context.Context) error { return checkUnits(ctx, root, overlay) },
			Fix:   func(ctx context.Context) error { return enable(ctx, root, overlay) },
		},
		{
			Name:  "system-running",
			Check: func(ctx context.Context) error { return checkRunning(ctx, execx.OSRunner{}) },
			Fix:   func(ctx context.Context) error { return start(ctx, execx.OSRunner{}) },
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
	data []byte
}

func files(rootDir, overlay string) ([]file, error) {
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
		data, err := os.ReadFile(filepath.Join(overlay, rel))
		if err != nil {
			return err
		}
		out = append(out, file{rel: rel, mode: mode, data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	p, err := machine.Read(filepath.Join(rootDir, machine.Path))
	if err != nil {
		return nil, err
	}
	for _, r := range p.Files() {
		if slices.ContainsFunc(out, func(f file) bool { return f.rel == r.Path }) {
			return nil, fmt.Errorf("overlay %s: rendered from %s; remove it from the overlay", r.Path, machine.Path)
		}
		out = append(out, file{rel: r.Path, mode: 0o644, data: r.Data})
	}
	return out, nil
}

func (f file) drift(root *os.Root) (string, error) {
	st, err := root.Lstat(f.rel)
	switch {
	case errors.Is(err, fs.ErrNotExist) && f.data == nil:
		return "", nil
	case errors.Is(err, fs.ErrNotExist):
		return "missing", nil
	case err != nil:
		return "", err
	case f.data == nil:
		return "unwanted", nil
	case !st.Mode().IsRegular():
		return "not a regular file", nil
	case st.Mode().Perm() != f.mode:
		return fmt.Sprintf("mode %04o, want %04o", st.Mode().Perm(), f.mode), nil
	}
	got, err := root.ReadFile(f.rel)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(got, f.data) {
		return "content differs", nil
	}
	return "", nil
}

func check(rootDir, overlay string) error {
	list, err := files(rootDir, overlay)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	var drift, unknown []string
	for _, f := range list {
		reason, err := f.drift(root)
		switch {
		case err != nil:
			unknown = append(unknown, f.rel+" ("+err.Error()+")")
		case reason != "":
			drift = append(drift, f.rel+" ("+reason+")")
		}
	}
	var problems []string
	if len(drift) > 0 {
		problems = append(problems, setup.List(fmt.Sprintf("%d of %d files differ", len(drift), len(list)), drift))
	}
	if len(unknown) > 0 {
		problems = append(problems, setup.List(fmt.Sprintf("cannot inspect %d of %d files", len(unknown), len(list)), unknown))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func apply(rootDir, overlay string) error {
	list, err := files(rootDir, overlay)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range list {
		reason, err := f.drift(root)
		if err != nil {
			return fmt.Errorf("%s: %w", f.rel, err)
		}
		if reason == "" {
			continue
		}
		if f.data == nil {
			err = root.Remove(f.rel)
		} else {
			err = write(root, f.rel, f.data, f.mode)
		}
		if err != nil {
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
	out, err := execx.OSRunner{}.Output(ctx, "", "systemctl", append([]string{"--root=" + root, "is-enabled"}, list...)...)
	states := strings.Split(out, "\n")
	if len(states) != len(list) {
		return fmt.Errorf("systemctl is-enabled: %w", err)
	}
	var off []string
	for i, unit := range list {
		if states[i] != "enabled" {
			off = append(off, unit+" ("+states[i]+")")
		}
	}
	if len(off) > 0 {
		return errors.New(setup.List("not enabled", off))
	}
	return nil
}

func enable(ctx context.Context, root, overlay string) error {
	list, err := units(overlay)
	if err != nil || len(list) == 0 {
		return err
	}
	_, err = execx.OSRunner{}.Output(ctx, "", "systemctl", append([]string{"--root=" + root, "enable"}, list...)...)
	return err
}

func checkRunning(ctx context.Context, run execx.Runner) error {
	out, err := run.Output(ctx, "", "systemctl", append([]string{"is-active"}, running...)...)
	if err == nil {
		return nil
	}
	states := strings.Split(out, "\n")
	if len(states) != len(running) {
		return fmt.Errorf("systemctl is-active: %w", err)
	}
	var off []string
	for i, unit := range running {
		if states[i] != "active" {
			off = append(off, unit+" ("+states[i]+")")
		}
	}
	if len(off) == 0 {
		return fmt.Errorf("systemctl is-active: %w", err)
	}
	return errors.New(setup.List("not running", off))
}

func start(ctx context.Context, run execx.Runner) error {
	_, err := run.Output(ctx, "", "systemctl", append([]string{"start"}, running...)...)
	return err
}
