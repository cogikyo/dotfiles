package home

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

var obsKeys = map[string]string{
	"Profile":             "Costello",
	"ProfileDir":          "Costello",
	"SceneCollection":     "Costello",
	"SceneCollectionFile": "Costello.json",
}

func obs(r paths.Root) []setup.Item {
	dir := filepath.Join(r.Home, ".config", "obs-studio")
	scene := filepath.Join(dir, "basic", "scenes", "Costello.json")
	user := filepath.Join(dir, "user.ini")
	return []setup.Item{
		{
			Name: "home-obs-dir",
			Check: func(context.Context) error {
				if legacy(dir, r.Config("obs-studio")) {
					return fmt.Errorf("%s links into the checkout; fix makes it a local directory", dir)
				}
				return localDir(dir)
			},
			Fix: func(context.Context) error {
				if !legacy(dir, r.Config("obs-studio")) {
					return localDir(dir)
				}
				if err := os.Remove(dir); err != nil {
					return err
				}
				return os.Mkdir(dir, 0o755)
			},
		},
		{
			Name:  "home-obs-scene",
			Check: func(context.Context) error { return localFile(scene) },
			Fix: func(context.Context) error {
				if err := localDir(dir); err != nil {
					return err
				}
				return seed(r.Config("obs-studio", "basic", "scenes", "Costello.json"), scene)
			},
		},
		{
			Name: "home-obs-selection",
			Check: func(context.Context) error {
				data, err := readLocal(user)
				if err != nil {
					return err
				}
				if drift := obsDrift(data); len(drift) > 0 {
					return fmt.Errorf("%s [Basic] differs: %s", user, strings.Join(drift, ", "))
				}
				return nil
			},
			Fix: func(context.Context) error {
				if err := localDir(dir); err != nil {
					return err
				}
				data, err := readLocal(user)
				if err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				return writeAtomic(user, mergeOBS(data), 0o644)
			},
		},
	}
}

func localDir(dir string) error {
	st, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) || err == nil && st.IsDir() {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%s must be a local directory, not a symlink or file", dir)
}

func basics(lines []string) [][2]int {
	var out [][2]int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		if n := len(out); n > 0 && out[n-1][1] == len(lines) {
			out[n-1][1] = i
		}
		if name, _, _ := strings.Cut(line[1:], "]"); name == "Basic" {
			out = append(out, [2]int{i, len(lines)})
		}
	}
	return out
}

func managed(line string) (string, bool) {
	key, _, ok := strings.Cut(line, "=")
	key = strings.TrimSpace(key)
	_, known := obsKeys[key]
	return key, ok && known
}

func obsDrift(data []byte) []string {
	lines := strings.Split(string(data), "\n")
	secs := basics(lines)
	type def struct {
		last  bool
		value string
	}
	have := map[string][]def{}
	for n, s := range secs {
		for _, line := range lines[s[0]+1 : s[1]] {
			if key, value, ok := strings.Cut(line, "="); ok {
				key = strings.TrimSpace(key)
				have[key] = append(have[key], def{n == len(secs)-1, strings.TrimSpace(value)})
			}
		}
	}
	var drift []string
	for _, key := range slices.Sorted(maps.Keys(obsKeys)) {
		if d := have[key]; len(d) != 1 || !d[0].last || d[0].value != obsKeys[key] {
			drift = append(drift, key)
		}
	}
	return drift
}

func mergeOBS(data []byte) []byte {
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	lines := strings.Split(text, "\n")
	secs := basics(lines)
	if len(secs) == 0 {
		if text != "" {
			lines = slices.Insert(lines, len(lines)-1, "")
		}
		lines = slices.Insert(lines, len(lines)-1, "[Basic]")
		secs = [][2]int{{len(lines) - 2, len(lines) - 1}}
	}
	var out []string
	pos := 0
	for _, s := range secs[:len(secs)-1] {
		out = append(out, lines[pos:s[0]+1]...)
		for _, line := range lines[s[0]+1 : s[1]] {
			if _, ok := managed(line); !ok {
				out = append(out, line)
			}
		}
		pos = s[1]
	}
	last := secs[len(secs)-1]
	out = append(out, lines[pos:last[0]+1]...)
	seen := map[string]bool{}
	var body []string
	for _, line := range lines[last[0]+1 : last[1]] {
		key, ok := managed(line)
		switch {
		case !ok:
			body = append(body, line)
		case !seen[key]:
			seen[key] = true
			body = append(body, key+"="+obsKeys[key])
		}
	}
	var missing []string
	for _, key := range slices.Sorted(maps.Keys(obsKeys)) {
		if !seen[key] {
			missing = append(missing, key+"="+obsKeys[key])
		}
	}
	return []byte(strings.Join(slices.Concat(out, missing, body, lines[last[1]:]), "\n"))
}
