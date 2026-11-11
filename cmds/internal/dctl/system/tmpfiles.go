package system

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

const tmpfilesDir = "etc/tmpfiles.d"

type tmpEntry struct {
	kind byte
	path string
}

func tmpfiles(overlay string) (confs []string, entries []tmpEntry, err error) {
	matches, err := filepath.Glob(filepath.Join(overlay, tmpfilesDir, "*.conf"))
	if err != nil {
		return nil, nil, err
	}
	for _, match := range matches {
		f, err := os.Open(match)
		if err != nil {
			return nil, nil, err
		}
		got, err := parseTmpfiles(f)
		f.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.Join(tmpfilesDir, filepath.Base(match)), err)
		}
		confs = append(confs, "/"+filepath.Join(tmpfilesDir, filepath.Base(match)))
		entries = append(entries, got...)
	}
	return confs, entries, nil
}

func parseTmpfiles(r io.Reader) ([]tmpEntry, error) {
	var out []tmpEntry
	s := bufio.NewScanner(r)
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("line %d: want \"<type> <path> ...\", got %q", n, line)
		}
		switch fields[0] {
		case "f", "d":
		case "e", "x":
			// No paths to create or check.
			continue
		default:
			return nil, fmt.Errorf("line %d: only f, d, e, and x entries are supported, got %q", n, fields[0])
		}
		path := fields[1]
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "%*?[") {
			return nil, fmt.Errorf("line %d: want an absolute path without specifiers or globs, got %q", n, path)
		}
		out = append(out, tmpEntry{kind: fields[0][0], path: filepath.Clean(path)})
	}
	return out, s.Err()
}

func checkTmpfiles(rootDir, overlay string) error {
	_, entries, err := tmpfiles(overlay)
	if err != nil || len(entries) == 0 {
		return err
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	var missing, wrong []string
	for _, e := range entries {
		st, err := root.Lstat(strings.TrimPrefix(e.path, "/"))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			missing = append(missing, e.path)
		case err != nil:
			wrong = append(wrong, e.path+" ("+err.Error()+")")
		case e.kind == 'd' && !st.IsDir():
			wrong = append(wrong, e.path+" (not a directory)")
		case e.kind == 'f' && !st.Mode().IsRegular():
			wrong = append(wrong, e.path+" (not a regular file)")
		}
	}
	var problems []string
	if len(missing) > 0 {
		problems = append(problems, setup.List(fmt.Sprintf("%d of %d tmpfiles paths missing", len(missing), len(entries)), missing))
	}
	if len(wrong) > 0 {
		problems = append(problems, setup.List(fmt.Sprintf("%d of %d tmpfiles paths wrong", len(wrong), len(entries)), wrong))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func createTmpfiles(ctx context.Context, run execx.Runner, rootDir, overlay string) error {
	confs, _, err := tmpfiles(overlay)
	if err != nil || len(confs) == 0 {
		return err
	}
	return run.Run(ctx, "", "systemd-tmpfiles", append([]string{"--create", "--root=" + rootDir}, confs...)...)
}
