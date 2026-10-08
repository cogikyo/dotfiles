package ssh

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

var published = []string{
	"SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s",
	"SHA256:p2QAMXNIC1TJYWeIOttrVc98/R1BUFWu3/LiyKgUfQM",
	"SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU",
}

type kind int

const (
	authed kind = iota
	offline
	unknownHost
	changedHost
	noKey
	noAgent
	locked
	rejected
	other
)

type probe struct {
	kind   kind
	keys   []string
	detail string
}

func (p probe) err() error {
	names := make([]string, len(p.keys))
	for i, k := range p.keys {
		names[i] = filepath.Base(k)
	}
	switch p.kind {
	case authed:
		return nil
	case offline:
		return errors.New("cannot reach github.com over SSH; check the network")
	case unknownHost:
		return errors.New("github.com host key not in known_hosts")
	case changedHost:
		return setup.Manual("github.com host key changed; compare known_hosts with GitHub's published fingerprints")
	case noKey:
		return setup.Manual("no SSH key for github.com; run dctl setup secrets")
	case noAgent:
		return setup.Manual("SSH agent unreachable; run systemctl --user start gcr-ssh-agent.socket")
	case locked:
		return fmt.Errorf("SSH keys not loaded in the agent: %s", strings.Join(names, ", "))
	case rejected:
		return setup.Manual("GitHub rejected SSH authentication; configured keys: %s; register the intended public key at github.com/settings/keys", strings.Join(names, ", "))
	}
	return errors.New(p.detail)
}

func Stage(root paths.Root, run execx.Runner) setup.Stage {
	return setup.Stage{
		Name:  "ssh",
		About: "SSH access to GitHub",
		Items: []setup.Item{{
			Name:  "ssh-github",
			Check: func(ctx context.Context) error { return check(ctx, root, run).err() },
			Fix: func(ctx context.Context) error {
				p := check(ctx, root, run)
				if p.kind == unknownHost {
					if err := trust(root); err != nil {
						return err
					}
					p = check(ctx, root, run)
				}
				if p.kind != locked {
					return nil
				}
				for _, key := range p.keys {
					if err := execx.Interactive(execx.Reason(run, "load the GitHub key into the SSH agent")).Run(ctx, "", "ssh-add", key); err != nil {
						return err
					}
				}
				return nil
			},
		}},
	}
}

func check(ctx context.Context, root paths.Root, run execx.Runner) probe {
	_, err := run.Output(ctx, "", "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-T", "git@github.com")
	if err == nil {
		return probe{kind: authed}
	}
	msg := err.Error()
	has := func(subs ...string) bool {
		return slices.ContainsFunc(subs, func(s string) bool { return strings.Contains(msg, s) })
	}
	switch {
	case has("successfully authenticated"):
		return probe{kind: authed}
	case has("REMOTE HOST IDENTIFICATION HAS CHANGED"):
		return probe{kind: changedHost}
	case has("Host key verification failed", "host key is known"):
		return probe{kind: unknownHost}
	case has("Could not resolve hostname", "Network is unreachable", "No route to host", "timed out", "Connection refused", "Connection closed", "Connection reset", "kex_exchange_identification"):
		return probe{kind: offline}
	case has("Permission denied (publickey"):
		return keys(ctx, root, run)
	}
	line, _, _ := strings.Cut(msg, "\n")
	return probe{kind: other, detail: line}
}

func keys(ctx context.Context, root paths.Root, run execx.Runner) probe {
	out, err := run.Output(ctx, "", "ssh", "-G", "github.com")
	if err != nil {
		return probe{kind: other, detail: err.Error()}
	}
	var files []string
	for line := range strings.Lines(out) {
		key, value, _ := strings.Cut(strings.TrimSpace(line), " ")
		if key != "identityfile" {
			continue
		}
		value = strings.ReplaceAll(value, "%d", root.Home)
		path := paths.ExpandHome(root.Home, value)
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() && !slices.Contains(files, path) {
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		return probe{kind: noKey}
	}
	loaded, err := run.Output(ctx, "", "ssh-add", "-l")
	if exit(err) == 2 {
		return probe{kind: noAgent}
	}
	var closed []string
	for _, f := range files {
		fp, err := run.Output(ctx, "", "ssh-keygen", "-l", "-f", f)
		fields := strings.Fields(fp)
		if err != nil || len(fields) < 2 {
			return probe{kind: other, detail: fmt.Sprintf("read %s: %v", f, err)}
		}
		if strings.Contains(loaded, fields[1]) {
			continue
		}
		if _, err := run.Output(ctx, "", "ssh-keygen", "-y", "-P", "", "-f", f); err == nil {
			continue
		}
		closed = append(closed, f)
	}
	if len(closed) > 0 {
		return probe{kind: locked, keys: closed}
	}
	return probe{kind: rejected, keys: files}
}

func exit(err error) int {
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		return e.ExitCode()
	}
	return -1
}

func trust(root paths.Root) error {
	src := root.System("etc", "ssh", "ssh_known_hosts")
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 3 || fields[0] != "github.com" {
			continue
		}
		blob, err := base64.StdEncoding.DecodeString(fields[2])
		if err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		sum := sha256.Sum256(blob)
		if fp := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]); !slices.Contains(published, fp) {
			return fmt.Errorf("%s: github.com %s fingerprint %s is not in the pinned fingerprint list", src, fields[1], fp)
		}
		lines = append(lines, strings.Join(fields[:3], " "))
	}
	if err := s.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		return fmt.Errorf("%s: no github.com keys", src)
	}
	dir := filepath.Join(root.Home, ".ssh")
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	path := filepath.Join(dir, "known_hosts")
	have, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	existing := strings.Split(string(have), "\n")
	lines = slices.DeleteFunc(lines, func(l string) bool { return slices.Contains(existing, l) })
	if len(lines) == 0 {
		return nil
	}
	text := strings.Join(lines, "\n") + "\n"
	if len(have) > 0 && have[len(have)-1] != '\n' {
		text = "\n" + text
	}
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	_, err = out.WriteString(text)
	return errors.Join(err, out.Close())
}
