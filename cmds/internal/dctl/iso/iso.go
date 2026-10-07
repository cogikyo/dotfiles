package iso

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/gobuild"
)

const MaxSize int64 = 2 << 30

func path(dotfiles, rev string) string {
	return filepath.Join(dotfiles, "iso", "out", "dotfiles-"+rev[:12]+".iso")
}

// Latest returns the most recently built ISO in iso/out.
func Latest(dotfiles string) (string, error) {
	found, err := filepath.Glob(filepath.Join(dotfiles, "iso", "out", "dotfiles-*.iso"))
	if err != nil {
		return "", err
	}
	var latest string
	var built time.Time
	for _, iso := range found {
		st, err := os.Stat(iso)
		if err != nil {
			return "", err
		}
		if st.ModTime().After(built) {
			latest, built = iso, st.ModTime()
		}
	}
	if latest == "" {
		return "", errors.New("no ISO in iso/out; run dctl iso build")
	}
	return latest, nil
}

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

func gocmd(dir string) []string {
	return slices.Concat([]string{"env", "GOENV=off", "GOTOOLCHAIN=local"}, gobuild.Env, []string{"go", "-C", dir, "build"}, gobuild.Flags)
}

type resolved struct {
	Repo string
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
		if len(f) != 3 {
			return nil, fmt.Errorf("pacman -Sp: unexpected line %q", strings.TrimSpace(line))
		}
		pkgs = append(pkgs, resolved{Repo: f[0], Name: f[1], File: f[2]})
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

func signedSum(ctx context.Context, allowed, iso string) (string, error) {
	sums := iso + ".sha256"
	sig := sums + ".sig"
	data, err := os.ReadFile(sums)
	if err != nil {
		return "", err
	}
	out, err := execx.OSRunner{}.Output(ctx, "", "ssh-keygen", "-Y", "find-principals", "-f", allowed, "-s", sig)
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return "", fmt.Errorf("%s is not signed by a key in %s", sums, allowed)
	}
	if err != nil {
		return "", err
	}
	principal, _, _ := strings.Cut(out, "\n")
	_, err = execx.OSRunner{Stdin: data}.Output(ctx, "", "ssh-keygen", "-Y", "verify", "-f", allowed, "-I", principal, "-n", "file", "-s", sig)
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return "", fmt.Errorf("%s is not a valid signature of %s by %s", sig, sums, principal)
	}
	if err != nil {
		return "", err
	}
	parsed, err := parseSums(data)
	if err != nil || len(parsed) != 1 || parsed[0].name != filepath.Base(iso) {
		return "", fmt.Errorf("%s does not describe %s", sums, filepath.Base(iso))
	}
	return parsed[0].hex, nil
}
