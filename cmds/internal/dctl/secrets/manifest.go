package secrets

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"
)

type Entry struct {
	Name   string      `json:"name"`
	Target string      `json:"target"`
	Mode   fs.FileMode `json:"mode"`
	Staged bool        `json:"staged,omitempty"`
}

func (e Entry) Path() string { return "~/" + e.Target }

func (e Entry) ciphertext(root paths.Root) string { return root.Secrets(e.Name + ".age") }

var (
	nameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	modeRE = regexp.MustCompile(`^[0-7]{3,4}$`)
)

func Manifest(root paths.Root) ([]Entry, error) {
	f, err := os.Open(root.Secrets("manifest"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := parseManifest(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name(), err)
	}
	return entries, nil
}

func parseManifest(r io.Reader) ([]Entry, error) {
	var entries []Entry
	names, targets := map[string]bool{}, map[string]bool{}
	s := bufio.NewScanner(r)
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		e, err := parseEntry(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		if names[e.Name] {
			return nil, fmt.Errorf("line %d: duplicate name %q", n, e.Name)
		}
		if targets[e.Target] {
			return nil, fmt.Errorf("line %d: duplicate target %s", n, e.Path())
		}
		names[e.Name], targets[e.Target] = true, true
		entries = append(entries, e)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("no entries")
	}
	return entries, nil
}

func parseEntry(line string) (Entry, error) {
	fields := strings.Split(line, ":")
	if len(fields) < 3 || len(fields) > 4 {
		return Entry{}, fmt.Errorf("want name:~/target:mode[:staged], got %q", line)
	}
	name, target, mode := fields[0], fields[1], fields[2]
	if !nameRE.MatchString(name) {
		return Entry{}, fmt.Errorf("invalid name %q", name)
	}
	rel, ok := strings.CutPrefix(target, "~/")
	if !ok || !filepath.IsLocal(rel) || filepath.Clean(rel) != rel {
		return Entry{}, fmt.Errorf("%s: target must be a clean path under ~/", name)
	}
	perm, err := strconv.ParseUint(mode, 8, 32)
	if !modeRE.MatchString(mode) || err != nil || perm > 0o777 {
		return Entry{}, fmt.Errorf("%s: invalid mode %q", name, mode)
	}
	e := Entry{Name: name, Target: rel, Mode: fs.FileMode(perm)}
	if len(fields) == 4 {
		if fields[3] != "staged" {
			return Entry{}, fmt.Errorf("%s: unknown flag %q", name, fields[3])
		}
		e.Staged = true
	}
	return e, nil
}
