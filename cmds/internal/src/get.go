package src

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type getOptions struct {
	update bool
}

func (a app) get(args []string) error {
	spec, opts, err := parseGet(args)
	if err != nil {
		return err
	}
	src, err := a.resolve(spec)
	if err != nil {
		return err
	}
	if src.kind == "go" {
		if opts.update {
			return fmt.Errorf("-u is not allowed for Go module pin %s@%s", src.module, src.version)
		}
		path, err := downloadModule(src.module, src.version)
		if err != nil {
			return err
		}
		return outputLine(a.stdout, path)
	}
	return a.getGit(src, opts.update)
}

func parseGet(args []string) (string, getOptions, error) {
	var spec string
	var opts getOptions
	for _, arg := range args {
		switch arg {
		case "-u":
			opts.update = true
		default:
			if spec != "" {
				return "", opts, errors.New("get accepts exactly one source spec")
			}
			spec = arg
		}
	}
	if spec == "" {
		return "", opts, errors.New("usage: src get <spec> [-u]")
	}
	return spec, opts, nil
}

type moduleDownload struct {
	Path    string
	Version string
	Dir     string
	Error   string
}

func downloadModule(module, version string) (string, error) {
	cmd := command("", "go", "mod", "download", "-json", module+"@"+version)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		line := oneLine(out.String())
		if line == "" {
			line = err.Error()
		}
		return "", fmt.Errorf("go mod download %s@%s: %s", module, version, line)
	}
	var dl moduleDownload
	if err := json.NewDecoder(&out).Decode(&dl); err != nil {
		return "", fmt.Errorf("decode go mod download: %w", err)
	}
	if dl.Error != "" {
		return "", fmt.Errorf("go mod download %s@%s: %s", module, version, dl.Error)
	}
	if dl.Dir == "" {
		return "", fmt.Errorf("go mod download %s@%s returned no Dir", module, version)
	}
	return dl.Dir, nil
}

func (a app) getGit(src source, update bool) error {
	dest, err := src.dest(a.cache)
	if err != nil {
		return err
	}
	if update && src.pinned {
		return fmt.Errorf("-u is only allowed for @default entries, got @%s", src.cacheRef)
	}
	if isDir(dest) && !update {
		return outputLine(a.stdout, dest)
	}
	if isDir(dest) && update {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("remove cached @default entry: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create cache parent: %w", err)
	}
	if err := clone(src, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return outputLine(a.stdout, dest)
}

func clone(src source, dest string) error {
	if shaRef.MatchString(src.ref) {
		return cloneSHA(src, dest)
	}
	args := []string{"clone", "--quiet", "--filter=blob:none", "--depth", "1", "--single-branch"}
	if src.ref != "default" {
		args = append(args, "--branch", src.ref)
	}
	args = append(args, src.url, dest)
	if err := runGit("", args...); err != nil {
		return fmt.Errorf("clone %s @%s: %w", src.url, src.ref, err)
	}
	if err := runGit(dest, "checkout", "--detach", "HEAD"); err != nil {
		return fmt.Errorf("detach checkout: %w", err)
	}
	return nil
}

func cloneSHA(src source, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create cache entry: %w", err)
	}
	steps := [][]string{
		{"init"},
		{"remote", "add", "origin", src.url},
		{"fetch", "--quiet", "--filter=blob:none", "--depth", "1", "origin", src.ref},
		{"checkout", "--detach", "FETCH_HEAD"},
	}
	for _, args := range steps {
		if err := runGit(dest, args...); err != nil {
			return fmt.Errorf("clone %s @%s: %w", src.url, src.ref, err)
		}
	}
	return nil
}

func runGit(dir string, args ...string) error {
	cmd := command(dir, "git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := oneLine(out.String())
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}
