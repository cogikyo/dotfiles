package repos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/execx"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, json, err string
	}{
		{"valid", `[{"name":"a","repo":"o/a","path":"~/a"},{"name":"b","repo":"o/b","path":"/srv/b"}]`, ""},
		{"unknown field", `[{"name":"a","repo":"o/a","path":"~/a","branch":"main"}]`, "unknown field"},
		{"missing value", `[{"name":"a","repo":"","path":"~/a"}]`, "needs name, repo, and path"},
		{"relative path", `[{"name":"a","repo":"o/a","path":"a"}]`, "must start with"},
		{"repo host injection", `[{"name":"a","repo":"@[evil.example]:owner/repo","path":"~/a"}]`, "GitHub owner/name"},
		{"repo extra slash", `[{"name":"a","repo":"o/a/b","path":"~/a"}]`, "GitHub owner/name"},
		{"repo whitespace", `[{"name":"a","repo":"o/a b","path":"~/a"}]`, "GitHub owner/name"},
		{"repo dot name", `[{"name":"a","repo":"o/..","path":"~/a"}]`, "GitHub owner/name"},
		{"duplicate name", `[{"name":"a","repo":"o/a","path":"~/a"},{"name":"a","repo":"o/b","path":"~/b"}]`, "duplicate name"},
		{"duplicate path", `[{"name":"a","repo":"o/a","path":"~/a"},{"name":"b","repo":"o/b","path":"~/a/"}]`, "duplicate path"},
		{"trailing data", `[] []`, "trailing data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "repos.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			switch {
			case tc.err == "" && err != nil:
				t.Fatalf("Load: %v", err)
			case tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)):
				t.Fatalf("Load error = %v, want %q", err, tc.err)
			}
		})
	}
}

func TestUpdateFastForwardOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")

	ctx := context.Background()
	tmp := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := execx.OSRunner{}.Output(ctx, dir, "git", args...)
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		return out
	}
	commit := func(dir, file string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, file), []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		git(dir, "add", file)
		git(dir, "commit", "-q", "-m", file)
	}
	clone := func(name string) string {
		dir := filepath.Join(tmp, name)
		git(tmp, "clone", "-q", "up.git", dir)
		return dir
	}

	git(tmp, "init", "-q", "--bare", "-b", "main", "up.git")
	seed := clone("seed")
	commit(seed, "one")
	git(seed, "push", "-q", "origin", "main")

	clean, dirty, diverged := clone("clean"), clone("dirty"), clone("diverged")
	if err := os.WriteFile(filepath.Join(dirty, "one"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(diverged, "local")
	commit(seed, "two")
	git(seed, "push", "-q", "origin", "main")
	upstream := git(seed, "rev-parse", "HEAD")

	for _, tc := range []struct {
		dir  string
		want State
		head string
	}{
		{clean, Forwarded, upstream},
		{clean, Current, upstream},
		{dirty, Dirty, git(dirty, "rev-parse", "HEAD")},
		{diverged, Diverged, git(diverged, "rev-parse", "HEAD")},
		{filepath.Join(tmp, "absent"), Absent, ""},
	} {
		got, err := update(ctx, execx.OSRunner{}, tc.dir)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(tc.dir), err)
		}
		if got != tc.want {
			t.Errorf("%s: state %q, want %q", filepath.Base(tc.dir), got, tc.want)
		}
		if tc.head != "" {
			if head := git(tc.dir, "rev-parse", "HEAD"); head != tc.head {
				t.Errorf("%s: HEAD %s, want %s", filepath.Base(tc.dir), head, tc.head)
			}
		}
	}
	if got, err := os.ReadFile(filepath.Join(dirty, "one")); err != nil || string(got) != "edited" {
		t.Errorf("dirty work tree changed: %q, %v", got, err)
	}
}
