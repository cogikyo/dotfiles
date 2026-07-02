package src

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type findOptions struct {
	dir string
	all bool
}

type hit struct {
	tier string
	path string
}

func (a app) find(args []string) error {
	query, opts, err := parseFind(args)
	if err != nil {
		return err
	}

	hits, tried := a.findHits(query, opts.dir)
	if len(hits) == 0 {
		return fmt.Errorf("no source for %q; tried %s", query, strings.Join(tried, ", "))
	}

	if !opts.all {
		return outputLine(a.stdout, hits[0].path)
	}

	for _, h := range hits {
		if err := outputLine(a.stdout, h.path); err != nil {
			return err
		}
	}
	return nil
}

func parseFind(args []string) (string, findOptions, error) {
	opts := findOptions{dir: "."}
	var query string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-a":
			opts.all = true
		case "-C":
			i++
			if i >= len(args) {
				return "", opts, errors.New("find -C needs a directory")
			}
			opts.dir = args[i]
		default:
			if after, ok := strings.CutPrefix(args[i], "-C="); ok {
				opts.dir = after
				continue
			}
			if query != "" {
				return "", opts, errors.New("find accepts exactly one query")
			}
			query = args[i]
		}
	}
	if query == "" {
		return "", opts, errors.New("usage: src find <query> [-C dir] [-a]")
	}
	return query, opts, nil
}

func (a app) findHits(query, dir string) ([]hit, []string) {
	var hits []hit
	var tried []string

	goTier := "go module"
	if path, status, ok := goModuleDir(query, dir); ok {
		hits = append(hits, hit{tier: "go module", path: path})
	} else if status != "" {
		goTier += " (" + status + ")"
	}
	tried = append(tried, goTier)

	tried = append(tried, "node_modules")
	if path, ok := nodeModuleDir(query, dir); ok {
		hits = append(hits, hit{tier: "node_modules", path: path})
	}

	tried = append(tried, "~/repos")
	if bareName(query) {
		path := filepath.Join(a.home, "repos", query)
		if isDir(path) {
			hits = append(hits, hit{tier: "~/repos", path: path})
		}
	}

	tried = append(tried, "cache")
	for _, path := range a.cacheHits(query) {
		hits = append(hits, hit{tier: "cache", path: path})
	}

	if filepath.IsAbs(query) {
		tried = append(tried, "pacman owner")
		for _, path := range a.pacmanHits(query) {
			hits = append(hits, hit{tier: "pacman owner", path: path})
		}
	}

	return hits, tried
}

func goModuleDir(query, dir string) (string, string, bool) {
	gomod := command(dir, "go", "env", "GOMOD")
	var gomodOut bytes.Buffer
	gomod.Stdout = &gomodOut
	if err := gomod.Run(); err != nil {
		return "", "", false
	}
	if strings.TrimSpace(gomodOut.String()) == "" || strings.TrimSpace(gomodOut.String()) == os.DevNull {
		if looksLikeFindGoModule(query) {
			return "", "no go.mod context", false
		}
		return "", "", false
	}

	cmd := command(dir, "go", "list", "-m", "-f", "{{.Dir}}", query)
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", "", false
	}
	path := strings.TrimSpace(out.String())
	if path == "" || !isDir(path) {
		return "", "", false
	}
	return path, "", true
}

func looksLikeFindGoModule(query string) bool {
	if strings.Contains(query, "://") || !strings.Contains(query, "/") {
		return false
	}
	host, _, _ := strings.Cut(query, "/")
	return strings.Contains(host, ".")
}

func nodeModuleDir(query, dir string) (string, bool) {
	start, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		path := filepath.Join(start, "node_modules", filepath.FromSlash(query))
		if isDir(path) {
			return path, true
		}
		parent := filepath.Dir(start)
		if parent == start {
			return "", false
		}
		start = parent
	}
}

func (a app) cacheHits(query string) []string {
	var hits []string
	filepath.WalkDir(a.cache, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == a.cache {
			return nil
		}
		name := d.Name()
		at := strings.LastIndex(name, "@")
		if at < 1 {
			return nil
		}
		if name[:at] == query || strings.TrimSuffix(name[:at], ".git") == query {
			hits = append(hits, path)
		}
		return filepath.SkipDir
	})
	return hits
}

func (a app) pacmanHits(path string) []string {
	cmd := command("", "pacman", "-Qo", "--", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	fields := strings.Fields(out.String())
	if len(fields) < 5 {
		return nil
	}
	pkg := fields[len(fields)-2]
	var hits []string
	archPath := filepath.Join(a.cache, "gitlab.archlinux.org", "archlinux", "packaging", "packages")
	for _, path := range a.cacheHits(pkg) {
		if strings.HasPrefix(path, archPath) {
			hits = append(hits, path)
		}
	}
	return hits
}

func bareName(query string) bool {
	return query != "" && !strings.Contains(query, "/") && !strings.Contains(query, string(filepath.Separator))
}
