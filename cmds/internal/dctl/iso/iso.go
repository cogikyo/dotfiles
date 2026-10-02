package iso

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
)

const (
	Payload = "/opt/dctl/payload"
	Targets = "/opt/dctl/targets"
	Bundle  = "/opt/dctl/dotfiles.bundle"
	Bin     = "/usr/local/bin"
	Repo    = "dctl"
	Sums    = "SHA256SUMS"

	MaxSize int64 = 2 << 30
)

func revision(ctx context.Context, run execx.Runner, dir string, as []string) (string, error) {
	git := func(args ...string) (string, error) {
		argv := append(slices.Clone(as), append([]string{"git", "-C", dir}, args...)...)
		return run.Output(ctx, "", argv[0], argv[1:]...)
	}
	branch, err := git("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	if branch != "master" {
		return "", fmt.Errorf("on branch %q; the ISO is built and released from master", branch)
	}
	status, err := git("status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", fmt.Errorf("worktree is dirty; commit or stash first:\n%s", status)
	}
	return git("rev-parse", "HEAD")
}

type resolved struct {
	Name string
	File string
}

func parseResolved(out string) ([]resolved, error) {
	var pkgs []resolved
	for line := range strings.Lines(out) {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			return nil, fmt.Errorf("pacman -Sp: unexpected line %q", strings.TrimSpace(line))
		}
		pkgs = append(pkgs, resolved{Name: f[0], File: f[1]})
	}
	return pkgs, nil
}

func unresolved(targets []string, pkgs []resolved) []string {
	var out []string
	for _, t := range targets {
		if !slices.ContainsFunc(pkgs, func(p resolved) bool { return p.Name == t }) {
			out = append(out, t)
		}
	}
	return out
}

func writeTargets(file string, names []string) error {
	return os.WriteFile(file, []byte(strings.Join(names, "\n")+"\n"), 0o644)
}

type sized struct {
	Name string
	Size int64
}

func oversize(total int64, pkgs []sized) error {
	if total <= MaxSize {
		return nil
	}
	top := slices.SortedFunc(slices.Values(pkgs), func(a, b sized) int { return cmp.Compare(b.Size, a.Size) })
	var b strings.Builder
	fmt.Fprintf(&b, "%s exceeds the %s release limit; largest payload packages:", mib(total), mib(MaxSize))
	for _, p := range top[:min(20, len(top))] {
		fmt.Fprintf(&b, "\n  %10s  %s", mib(p.Size), p.Name)
	}
	return errors.New(b.String())
}

func mib(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20)) }

func digest(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func signedSum(ctx context.Context, allowed, iso string) (string, error) {
	sums := iso + ".sha256"
	sig := sums + ".sig"
	data, err := os.ReadFile(sums)
	if err != nil {
		return "", err
	}
	out, err := execx.OSRunner{}.Output(ctx, "", "ssh-keygen", "-Y", "find-principals", "-f", allowed, "-s", sig)
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return "", fmt.Errorf("%s is not signed by a key in %s: %w", sums, allowed, err)
	}
	if err != nil {
		return "", err
	}
	principal, _, _ := strings.Cut(out, "\n")
	if _, err := (execx.OSRunner{Stdin: data}).Output(ctx, "", "ssh-keygen", "-Y", "verify", "-f", allowed, "-I", principal, "-n", "file", "-s", sig); err != nil {
		return "", fmt.Errorf("signature check failed: %w", err)
	}
	sum, name, ok := strings.Cut(strings.TrimSpace(string(data)), "  ")
	if !ok || name != filepath.Base(iso) {
		return "", fmt.Errorf("%s does not describe %s", sums, filepath.Base(iso))
	}
	return sum, nil
}

func writeSums(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		sum, err := digest(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, e.Name())
	}
	return os.WriteFile(filepath.Join(dir, Sums), []byte(b.String()), 0o644)
}
