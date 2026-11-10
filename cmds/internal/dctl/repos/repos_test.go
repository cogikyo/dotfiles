package repos

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name, list, err string
	}{
		{"valid", "# owner/repo path\no/a ~/a\n\no/b /srv/b # comment\n", ""},
		{"missing path", "o/a\n", "want owner/name and path"},
		{"extra field", "o/a ~/a main\n", "want owner/name and path"},
		{"relative path", "o/a a\n", "must start with"},
		{"repo host injection", "@[evil.example]:owner/repo ~/a\n", "GitHub owner/name"},
		{"repo extra slash", "o/a/b ~/a\n", "GitHub owner/name"},
		{"repo dot name", "o/.. ~/a\n", "GitHub owner/name"},
		{"duplicate path", "o/a ~/a\no/b ~/a/\n", "duplicate path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.list))
			switch {
			case tc.err == "" && err != nil:
				t.Fatalf("Parse: %v", err)
			case tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)):
				t.Fatalf("Parse error = %v, want %q", err, tc.err)
			}
		})
	}
}

func testGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")
}

func TestUpdateFastForwardOnly(t *testing.T) {
	testGit(t)

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
	origin := git(clean, "remote", "get-url", "origin")

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
		got, _, err := update(ctx, execx.OSRunner{}, tc.dir, origin)
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

type offline struct{ execx.OSRunner }

func (o offline) Run(ctx context.Context, dir, name string, args ...string) error {
	if name == "git" && len(args) > 0 && (args[0] == "clone" || args[0] == "fetch") {
		return errors.New("offline: git " + args[0])
	}
	return o.OSRunner.Run(ctx, dir, name, args...)
}

type layout struct {
	t    *testing.T
	ctx  context.Context
	root paths.Root
}

func newLayout(t *testing.T, lists ...string) *layout {
	t.Helper()
	testGit(t)
	home := t.TempDir()
	l := &layout{t: t, ctx: context.Background(), root: paths.Root{Dotfiles: filepath.Join(home, "dotfiles"), Home: home}}
	l.git(home, "init", "-q", "-b", "main", l.root.Dotfiles)
	for i, list := range lists {
		l.write(list, filepath.Join("dotfiles", "packages", "repos.lst"))
		l.git(l.root.Dotfiles, "add", "-A")
		l.git(l.root.Dotfiles, "commit", "-q", "-m", fmt.Sprint("layout ", i))
	}
	return l
}

func (l *layout) git(dir string, args ...string) string {
	l.t.Helper()
	out, err := execx.OSRunner{}.Output(l.ctx, dir, "git", args...)
	if err != nil {
		l.t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func (l *layout) path(rel string) string { return filepath.Join(l.root.Home, rel) }

func (l *layout) write(text, rel string) {
	l.t.Helper()
	if err := os.MkdirAll(filepath.Dir(l.path(rel)), 0o755); err != nil {
		l.t.Fatal(err)
	}
	if err := os.WriteFile(l.path(rel), []byte(text), 0o644); err != nil {
		l.t.Fatal(err)
	}
}

func (l *layout) checkout(repo, rel string) string {
	l.t.Helper()
	dir := l.path(rel)
	l.git(l.root.Home, "init", "-q", "-b", "main", dir)
	l.git(dir, "remote", "add", "origin", Repo{Repo: repo}.URL())
	l.write(repo, filepath.Join(rel, "name"))
	l.git(dir, "add", "name")
	l.git(dir, "commit", "-q", "-m", repo)
	return l.git(dir, "rev-parse", "HEAD")
}

func (l *layout) apply() setup.Report {
	l.t.Helper()
	r, err := Stage(l.root, offline{}).Apply(l.ctx, true)
	if err != nil {
		l.t.Fatalf("Apply: %v", err)
	}
	return r
}

func (l *layout) want(r setup.Report, state setup.State) {
	l.t.Helper()
	for _, it := range r.Items {
		want := setup.Done
		if it.Item == "repos-cloned" {
			want = state
		}
		if it.State != want {
			l.t.Errorf("%s: %s (%s), want %s", it.Item, it.State, it.Detail, want)
		}
	}
}

func (l *layout) at(rel, repo, head string) {
	l.t.Helper()
	dir := l.path(rel)
	c, err := inspect(l.ctx, execx.OSRunner{}, dir, Repo{Repo: repo}.URL())
	if err != nil || c.kind != usable {
		l.t.Fatalf("%s: %+v, %v; want a usable %s checkout", rel, c, err, repo)
	}
	if got := l.git(dir, "rev-parse", "HEAD"); got != head {
		l.t.Errorf("%s: HEAD %s, want %s", rel, got, head)
	}
}

func (l *layout) gone(rels ...string) {
	l.t.Helper()
	for _, rel := range rels {
		if _, err := os.Lstat(l.path(rel)); !errors.Is(err, fs.ErrNotExist) {
			l.t.Errorf("%s still exists (%v)", rel, err)
		}
	}
}

func (l *layout) tree() []string {
	l.t.Helper()
	var all []string
	err := filepath.WalkDir(l.root.Home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		all = append(all, path)
		return nil
	})
	if err != nil {
		l.t.Fatal(err)
	}
	return all
}

func TestStageMovesOldCheckouts(t *testing.T) {
	l := newLayout(t,
		"o/a ~/box/a\no/box ~/box/box\no/c ~/old/c\no/d ~/d\n",
		"o/a ~/a\no/box ~/box\no/c ~/new/deep/c\no/d ~/d\n",
	)
	a, box, c, d := l.checkout("o/a", "box/a"), l.checkout("o/box", "box/box"), l.checkout("o/c", "old/c"), l.checkout("o/d", "d")
	l.write("edited", "box/a/name")
	l.write("untracked", "old/c/notes")
	if err := os.MkdirAll(l.path("box/empty/nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	status := Stage(l.root, offline{}).Evaluate(l.ctx)
	if status.State != setup.Pending {
		t.Fatalf("status %s, want %s: %+v", status.State, setup.Pending, status.Items)
	}
	for _, move := range []string{"o/a: move ~/box/a to ~/a", "o/box: move ~/box/box to ~/box", "o/c: move ~/old/c to ~/new/deep/c"} {
		if !strings.Contains(status.Items[0].Detail, move) {
			t.Errorf("status detail %q lacks %q", status.Items[0].Detail, move)
		}
	}

	l.want(l.apply(), setup.Done)
	l.at("a", "o/a", a)
	l.at("box", "o/box", box)
	l.at("new/deep/c", "o/c", c)
	l.at("d", "o/d", d)
	l.gone("box/a", "box/box", "box/empty", "old", ".local/state/dctl/repos")
	if got, err := os.ReadFile(l.path("a/name")); err != nil || string(got) != "edited" {
		t.Errorf("a/name = %q, %v; want the uncommitted edit", got, err)
	}
	if _, err := os.Stat(l.path("new/deep/c/notes")); err != nil {
		t.Errorf("untracked file lost: %v", err)
	}

	before := l.tree()
	l.want(l.apply(), setup.Done)
	if after := l.tree(); !slices.Equal(before, after) {
		t.Errorf("second run changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestStageResumesStagedCheckout(t *testing.T) {
	l := newLayout(t, "o/a ~/old/a\n", "o/a ~/a\n")
	head := l.checkout("o/a", ".local/state/dctl/repos/o/a")
	status := Stage(l.root, offline{}).Evaluate(l.ctx)
	if want := "o/a: move ~/.local/state/dctl/repos/o/a to ~/a"; !strings.Contains(status.Items[0].Detail, want) {
		t.Errorf("status detail %q lacks %q", status.Items[0].Detail, want)
	}
	l.want(l.apply(), setup.Done)
	l.at("a", "o/a", head)
	l.gone(".local/state/dctl/repos")
}

func TestStageLeavesBlockedCheckouts(t *testing.T) {
	l := newLayout(t, "o/a ~/old/a\no/w ~/old/w\n", "o/a ~/a\no/w ~/w\n")
	l.checkout("o/a", "old/a")
	l.write("keep", "a/notes")
	l.checkout("o/w", "old/w")
	l.git(l.path("old/w"), "worktree", "add", "-q", "-b", "side", l.path("side"))

	r := l.apply()
	l.want(r, setup.ManualState)
	detail := r.Items[0].Detail
	for _, want := range []string{
		"o/a: " + string(Broken) + " (no .git); move ~/a aside, then run `dctl setup repos`, which moves ~/old/a there",
		"o/w: checkout ~/old/w has linked worktrees; move it to ~/w by hand",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q lacks %q", detail, want)
		}
	}
	if _, err := os.Stat(l.path("a/notes")); err != nil {
		t.Errorf("occupied target changed: %v", err)
	}
	if _, err := os.Stat(l.path("old/a/name")); err != nil {
		t.Errorf("blocked checkout moved: %v", err)
	}
	l.gone("w")
}

func TestHistoryFixOnFullClone(t *testing.T) {
	l := newLayout(t, "o/a ~/a\n")
	history := Stage(l.root, offline{}).Items[1]
	if err := history.Fix(l.ctx); err != nil {
		t.Fatalf("Fix on a full clone: %v", err)
	}
	if err := history.Check(l.ctx); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestStageRefusesUnsafeCandidates(t *testing.T) {
	for _, tc := range []struct {
		name, old, now, detail string
		build                  func(*layout)
		kept, gone             []string
	}{{
		name:   "dotfiles behind a symlinked parent",
		old:    "o/df ~/alias/dotfiles\n",
		now:    "o/df ~/new\n",
		detail: "o/df: checkout ~/alias/dotfiles is the dotfiles checkout; move it to ~/new by hand",
		build: func(l *layout) {
			l.git(l.root.Dotfiles, "remote", "add", "origin", Repo{Repo: "o/df"}.URL())
			if err := os.Symlink(".", l.path("alias")); err != nil {
				l.t.Fatal(err)
			}
		},
		kept: []string{"dotfiles/.git", "dotfiles/packages/repos.lst"},
		gone: []string{"new"},
	}, {
		name:   "symlinked checkout",
		old:    "o/a ~/old\n",
		now:    "o/a ~/deep/new\n",
		detail: "o/a: checkout ~/old is a symlink to ~/actual; move it to ~/deep/new by hand",
		build: func(l *layout) {
			l.checkout("o/a", "actual")
			if err := os.Symlink("actual", l.path("old")); err != nil {
				l.t.Fatal(err)
			}
		},
		kept: []string{"old/name", "actual/name"},
		gone: []string{"deep"},
	}, {
		name:   "old path now claimed by another repo",
		old:    "o/a ~/old\n",
		now:    "o/a ~/new\no/b ~/old\n",
		detail: "o/a: checkout ~/old is the catalog path for o/b; move it to ~/new by hand",
		build:  func(l *layout) { l.checkout("o/a", "old") },
		kept:   []string{"old/name"},
		gone:   []string{"new"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayout(t, tc.old, tc.now)
			tc.build(l)
			status := Stage(l.root, offline{}).Evaluate(l.ctx)
			l.want(status, setup.ManualState)
			r := l.apply()
			l.want(r, setup.ManualState)
			for _, detail := range []string{status.Items[0].Detail, r.Items[0].Detail} {
				if !strings.Contains(detail, tc.detail) {
					t.Errorf("detail %q lacks %q", detail, tc.detail)
				}
			}
			for _, rel := range tc.kept {
				if _, err := os.Lstat(l.path(rel)); err != nil {
					t.Errorf("%s moved: %v", rel, err)
				}
			}
			l.gone(tc.gone...)
			l.gone(".local/state/dctl/repos")
		})
	}
}
