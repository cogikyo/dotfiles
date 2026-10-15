package packages

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"dotfiles/cmds/internal/dctl/execx"
)

func Bun(ctx context.Context, run execx.Runner, dir string) error {
	return run.Run(ctx, dir, "bun", "install", "--frozen-lockfile", "--ignore-scripts")
}

func bunLocked(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "bun.lock"))
	if err != nil {
		return err
	}
	data, err = hujson.Standardize(data)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Join(dir, "bun.lock"), err)
	}
	var lock struct {
		Packages map[string][]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return fmt.Errorf("%s: %w", filepath.Join(dir, "bun.lock"), err)
	}
	cpu := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	var stale []string
	for _, key := range slices.Sorted(maps.Keys(lock.Packages)) {
		entry := lock.Packages[key]
		var ident string
		if len(entry) > 0 {
			_ = json.Unmarshal(entry[0], &ident)
		}
		at := strings.LastIndex(ident, "@")
		if at <= 0 {
			return fmt.Errorf("bun.lock package %q has no name@version", key)
		}
		name, version := ident[:at], ident[at+1:]
		var meta struct {
			OS   any `json:"os"`
			CPU  any `json:"cpu"`
			Libc any `json:"libc"`
		}
		for _, raw := range entry[1:] {
			if json.Unmarshal(raw, &meta) == nil {
				break
			}
		}
		if !targets(meta.OS, runtime.GOOS) || !targets(meta.CPU, cpu) || !targets(meta.Libc, "glibc") {
			continue
		}
		var installed struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		data, err := os.ReadFile(filepath.Join(dir, modulePath(key), "package.json"))
		if err != nil || json.Unmarshal(data, &installed) != nil || installed.Name != name || release(installed.Version) != release(version) {
			stale = append(stale, key)
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf("%d packages differ from bun.lock: %s", len(stale), strings.Join(stale[:min(len(stale), 5)], " "))
	}
	return nil
}

func targets(declared any, current string) bool {
	switch v := declared.(type) {
	case nil:
		return true
	case string:
		return v == current
	case []any:
		negated := len(v) > 0 && strings.HasPrefix(fmt.Sprint(v[0]), "!")
		for _, name := range v {
			if name == current || name == "!"+current {
				return !negated
			}
		}
		return negated
	}
	return false
}

func modulePath(key string) string {
	var names []string
	for part := range strings.SplitSeq(key, "/") {
		if n := len(names); n > 0 && strings.HasPrefix(names[n-1], "@") && !strings.Contains(names[n-1], "/") {
			names[n-1] += "/" + part
			continue
		}
		names = append(names, part)
	}
	return filepath.Join("node_modules", strings.Join(names, "/node_modules/"))
}

func release(version string) string {
	version, _, _ = strings.Cut(version, "+")
	return version
}
