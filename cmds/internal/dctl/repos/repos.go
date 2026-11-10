package repos

import (
	"bufio"
	"context"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type Repo struct {
	Repo string
	Path string
}

func (r Repo) URL() string { return "git@github.com:" + r.Repo + ".git" }

func (r Repo) Dir(home string) string { return paths.ExpandHome(home, r.Path) }

func Read(path string) ([]Repo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	repos, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return repos, nil
}

// Parse reads one "owner/name path" pair per line; "#" starts a comment.
func Parse(r io.Reader) ([]Repo, error) {
	var repos []Repo
	seen := map[string]bool{}
	s := bufio.NewScanner(r)
	for n := 1; s.Scan(); n++ {
		line, _, _ := strings.Cut(s.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: %q: want owner/name and path", n, strings.TrimSpace(line))
		}
		repo, path := fields[0], fields[1]
		switch {
		case !github.MatchString(repo) || strings.HasSuffix(repo, "/.") || strings.HasSuffix(repo, "/.."):
			return nil, fmt.Errorf("line %d: repo %q must be a GitHub owner/name", n, repo)
		case !strings.HasPrefix(path, "~/") && !filepath.IsAbs(path):
			return nil, fmt.Errorf("line %d: path %q must start with ~/ or /", n, path)
		case seen[filepath.Clean(path)]:
			return nil, fmt.Errorf("line %d: duplicate path %q", n, path)
		}
		seen[filepath.Clean(path)] = true
		repos = append(repos, Repo{Repo: repo, Path: path})
	}
	return repos, s.Err()
}

var github = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

func list(root paths.Root) string { return filepath.Join(root.Dotfiles, "packages", "repos.lst") }

type kind int

const (
	absent kind = iota
	usable
	broken
)

type checkout struct {
	kind     kind
	detached bool
	reason   string
}

func inspect(ctx context.Context, run execx.Runner, dir, url string) (checkout, error) {
	bad := func(format string, args ...any) (checkout, error) {
		return checkout{kind: broken, reason: fmt.Sprintf(format, args...)}, nil
	}
	rejected := func(err error) (checkout, error) {
		if exit(err) <= 0 {
			return checkout{}, err
		}
		msg := err.Error()
		if _, fatal, ok := strings.Cut(msg, "fatal: "); ok {
			msg, _, _ = strings.Cut(fatal, "\n")
		}
		return bad("%s", msg)
	}
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return checkout{kind: absent}, nil
	} else if err != nil {
		return checkout{}, err
	}
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return bad("dangling symlink")
	case err != nil:
		return checkout{}, err
	case !info.IsDir():
		return bad("not a directory")
	}
	if _, err := os.Lstat(filepath.Join(dir, ".git")); errors.Is(err, fs.ErrNotExist) {
		return bad("no .git")
	} else if err != nil {
		return checkout{}, err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return checkout{}, err
	}
	git := func(args ...string) (string, error) { return run.Output(ctx, dir, "git", args...) }
	if top, err := git("rev-parse", "--show-toplevel"); err != nil {
		return rejected(err)
	} else if top != real {
		return bad("not the checkout root; Git resolved %s", top)
	}
	origin, err := git("remote", "get-url", "origin")
	switch {
	case exit(err) == 2:
		return bad("no origin remote")
	case err != nil:
		return rejected(err)
	case origin != url:
		return bad("origin is %s, want %s", origin, url)
	}
	if _, err := git("rev-parse", "--verify", "-q", "HEAD^{commit}"); exit(err) == 1 {
		return bad("no HEAD commit")
	} else if err != nil {
		return rejected(err)
	}
	if _, err := git("symbolic-ref", "-q", "HEAD"); exit(err) == 1 {
		return checkout{kind: usable, detached: true}, nil
	} else if err != nil {
		return rejected(err)
	}
	return checkout{kind: usable}, nil
}

func exit(err error) int {
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		return e.ExitCode()
	}
	return -1
}

func action(r Repo, reason string) string {
	return fmt.Sprintf("%s (%s); move %s aside, then run `dctl setup repos`", Broken, reason, r.Path)
}

func clone(ctx context.Context, run execx.Runner, r Repo, dir string) (err error) {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+".clone-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(stage)) }()
	work := filepath.Join(stage, filepath.Base(dir))
	if err := execx.Remote(run).Run(ctx, "", "git", "clone", r.URL(), work); err != nil {
		return err
	}
	c, err := inspect(ctx, run, work, r.URL())
	switch {
	case err != nil:
		return err
	case c.kind != usable:
		return fmt.Errorf("fresh clone is %s (%s)", Broken, c.reason)
	}
	if err := renameNoReplace(work, dir); err != nil {
		return &os.LinkError{Op: "rename", Old: work, New: dir, Err: err}
	}
	return nil
}

type State string

const (
	Current   State = "up to date"
	Forwarded State = "fast-forwarded"
	Ahead     State = "ahead of upstream"
	Diverged  State = "diverged from upstream, not touched"
	Dirty     State = "uncommitted changes, not touched"
	Detached  State = "detached HEAD, not touched"
	Untracked State = "no upstream branch"
	Skipped   State = "skipped; update never pulls the dotfiles checkout"
	Absent    State = "not cloned"
	Broken    State = "not a usable checkout"
	Failed    State = "failed"
)

type result struct {
	repo   string
	state  State
	detail string
}

func (r result) level() ui.Level {
	switch r.state {
	case Current, Forwarded:
		return ui.OK
	case Skipped:
		return ui.Info
	case Broken, Failed:
		return ui.Err
	}
	return ui.Warn
}

func (r result) summary() string {
	if r.detail != "" {
		return r.detail
	}
	return string(r.state)
}

func Update(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner) error {
	repos, err := Read(list(root))
	if err != nil {
		return err
	}
	var checkouts []os.FileInfo
	for _, dir := range []string{filepath.Join(root.Home, "dotfiles"), root.Dotfiles} {
		if st, err := os.Stat(dir); err == nil {
			checkouts = append(checkouts, st)
		}
	}
	results := make([]result, len(repos))
	var wg sync.WaitGroup
	for i, r := range repos {
		wg.Go(func() {
			dir := r.Dir(root.Home)
			res := result{repo: r.Repo}
			st, err := os.Stat(dir)
			if err == nil && slices.ContainsFunc(checkouts, func(c os.FileInfo) bool { return os.SameFile(c, st) }) {
				res.state = Skipped
			} else {
				res.state, res.detail, err = update(ctx, run, dir, r.URL())
				switch {
				case err != nil:
					res.state, res.detail = Failed, err.Error()
				case res.state == Broken:
					res.detail = action(r, res.detail)
				}
			}
			results[i] = res
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	var missing, failed []string
	for _, res := range results {
		switch res.state {
		case Absent:
			missing = append(missing, res.repo)
			continue
		case Broken, Failed:
			failed = append(failed, res.repo)
		}
		u.Row(res.level(), res.repo+": "+res.summary())
	}
	if len(missing) > 0 {
		u.Warn("%d repos %s; run `dctl setup repos`", len(missing), Absent)
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d repos failed: %s", len(failed), len(repos), strings.Join(failed, ", "))
	}
	return nil
}

func update(ctx context.Context, run execx.Runner, dir, url string) (State, string, error) {
	c, err := inspect(ctx, run, dir, url)
	switch {
	case err != nil:
		return "", "", err
	case c.kind == absent:
		return Absent, "", nil
	case c.kind == broken:
		return Broken, c.reason, nil
	case c.detached:
		return Detached, "", nil
	}
	git := func(args ...string) (string, error) { return run.Output(ctx, dir, "git", args...) }
	status, err := git("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		return Dirty, "", nil
	}
	if _, err := git("rev-parse", "--verify", "-q", "@{upstream}"); exit(err) == 1 {
		return Untracked, "", nil
	} else if err != nil {
		return "", "", err
	}
	if _, err := execx.Remote(run).Output(ctx, dir, "git", "fetch", "--quiet"); err != nil {
		return "", "", err
	}
	counts, err := git("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return "", "", err
	}
	left, right, _ := strings.Cut(counts, "\t")
	ahead, err := strconv.Atoi(left)
	if err != nil {
		return "", "", fmt.Errorf("parse rev-list counts %q: %w", counts, err)
	}
	behind, err := strconv.Atoi(right)
	if err != nil {
		return "", "", fmt.Errorf("parse rev-list counts %q: %w", counts, err)
	}
	switch {
	case behind == 0 && ahead == 0:
		return Current, "", nil
	case behind == 0:
		return Ahead, "", nil
	case ahead > 0:
		return Diverged, "", nil
	}
	if _, err := git("merge", "--ff-only", "--quiet", "@{upstream}"); err != nil {
		return "", "", err
	}
	return Forwarded, "", nil
}

type plan struct {
	repo   Repo
	dir    string
	target checkout
	err    error
	from   string
	held   string
}

func survey(ctx context.Context, run execx.Runner, root paths.Root) ([]plan, error) {
	repos, err := Read(list(root))
	if err != nil {
		return nil, err
	}
	history, err := past(ctx, run, root)
	if err != nil {
		return nil, err
	}
	claims := []claim{
		{filepath.Clean(root.Dotfiles), "the dotfiles checkout"},
		{filepath.Join(root.Home, "dotfiles"), "the dotfiles checkout"},
	}
	for _, r := range repos {
		claims = append(claims, claim{filepath.Clean(r.Dir(root.Home)), "the catalog path for " + r.Repo})
	}
	plans := make([]plan, len(repos))
	for i, r := range repos {
		p := plan{repo: r, dir: filepath.Clean(r.Dir(root.Home))}
		p.target, p.err = inspect(ctx, run, p.dir, r.URL())
		if p.err == nil {
			p.from, p.held, p.err = find(ctx, run, root.Home, r, p.target.kind == usable, history[r.Repo], claims)
		}
		plans[i] = p
	}
	for changed := true; changed; {
		changed = false
		leaving := map[string]bool{}
		for _, p := range plans {
			if p.from != "" {
				leaving[p.from] = true
			}
		}
		for i := range plans {
			p := &plans[i]
			if p.from == "" || p.target.kind == absent {
				continue
			}
			if _, err := hollow(p.dir, leaving); err == nil {
				continue
			}
			p.held = fmt.Sprintf("%s, which moves %s there", action(p.repo, p.target.reason), show(root.Home, p.from))
			p.from, changed = "", true
		}
	}
	return plans, nil
}

func past(ctx context.Context, run execx.Runner, root paths.Root) (map[string][]string, error) {
	const file = "packages/repos.lst"
	revs, err := run.Output(ctx, root.Dotfiles, "git", "log", "--format=%H", "--", file)
	if err != nil {
		return nil, err
	}
	history := map[string][]string{}
	for rev := range strings.FieldsSeq(revs) {
		text, err := run.Output(ctx, root.Dotfiles, "git", "show", rev+":"+file)
		switch {
		case exit(err) > 0:
			continue
		case err != nil:
			return nil, err
		}
		repos, err := Parse(strings.NewReader(text))
		if err != nil {
			continue
		}
		for _, r := range repos {
			if !slices.Contains(history[r.Repo], r.Path) {
				history[r.Repo] = append(history[r.Repo], r.Path)
			}
		}
	}
	return history, nil
}

func staging(home string, r Repo) string {
	return filepath.Join(home, ".local", "state", "dctl", "repos", r.Repo)
}

type claim struct{ dir, by string }

func find(ctx context.Context, run execx.Runner, home string, r Repo, settled bool, old []string, claims []claim) (from, held string, err error) {
	movable := func(dir string) (string, string, error) {
		reason, err := pinned(ctx, run, home, dir, claims)
		switch {
		case err != nil:
			return "", "", err
		case reason != "":
			return "", fmt.Sprintf("checkout %s %s; move it to %s by hand", show(home, dir), reason, r.Path), nil
		}
		return dir, "", nil
	}
	stage := staging(home, r)
	c, err := inspect(ctx, run, stage, r.URL())
	switch {
	case err != nil:
		return "", "", err
	case c.kind == broken:
		return "", fmt.Sprintf("%s is %s (%s); move it aside, then run `dctl setup repos`", show(home, stage), Broken, c.reason), nil
	case c.kind == usable && settled:
		return "", fmt.Sprintf("%s duplicates %s; compare them, then remove %s", show(home, stage), r.Path, show(home, stage)), nil
	case c.kind == usable:
		return movable(stage)
	case settled:
		return "", "", nil
	}
	own := filepath.Clean(r.Dir(home))
	for _, path := range old {
		dir := filepath.Clean(paths.ExpandHome(home, path))
		if dir == own {
			continue
		}
		c, err := inspect(ctx, run, dir, r.URL())
		switch {
		case err != nil:
			return "", "", err
		case c.kind == usable:
			return movable(dir)
		}
	}
	return "", "", nil
}

func pinned(ctx context.Context, run execx.Runner, home, dir string, claims []claim) (string, error) {
	st, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		return "is a symlink to " + show(home, real), nil
	}
	for _, c := range claims {
		if _, err := os.Lstat(c.dir); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(c.dir))
		if err != nil {
			return "", err
		}
		ids := []string{filepath.Join(parent, filepath.Base(c.dir))}
		if to, err := filepath.EvalSymlinks(c.dir); err == nil {
			ids = append(ids, to)
		}
		for _, id := range ids {
			switch {
			case id == real:
				return "is " + c.by, nil
			case inside(real, id):
				return "holds " + show(home, c.dir), nil
			}
		}
	}
	out, err := run.Output(ctx, dir, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	var trees []string
	for line := range strings.Lines(out) {
		if tree, ok := strings.CutPrefix(strings.TrimSpace(line), "worktree "); ok {
			trees = append(trees, tree)
		}
	}
	switch {
	case len(trees) == 0 || trees[0] != real:
		return "is a linked worktree", nil
	case len(trees) > 1:
		return "has linked worktrees", nil
	}
	return "", nil
}

func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

func show(home, dir string) string {
	if inside(home, dir) {
		rel, _ := filepath.Rel(home, dir)
		return "~/" + rel
	}
	return dir
}

func hollow(dir string, leaving map[string]bool) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case leaving[path] && d.IsDir():
			return filepath.SkipDir
		case leaving[path]:
			return nil
		case !d.IsDir():
			return fmt.Errorf("%s holds %s", dir, path)
		}
		dirs = append(dirs, path)
		return nil
	})
	return dirs, err
}

func relocate(home string, plans []plan) {
	var moves []*plan
	for i := range plans {
		if plans[i].from != "" {
			moves = append(moves, &plans[i])
		}
	}
	slices.SortFunc(moves, func(a, b *plan) int { return strings.Compare(b.from, a.from) })
	for _, p := range moves {
		stage := staging(home, p.repo)
		if p.from == stage {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(stage), 0o755); err != nil {
			p.err = err
			continue
		}
		if err := renameNoReplace(p.from, stage); err != nil {
			p.err = &os.LinkError{Op: "rename", Old: p.from, New: stage, Err: err}
			continue
		}
	}
	slices.SortFunc(moves, func(a, b *plan) int { return strings.Compare(a.dir, b.dir) })
	for _, p := range moves {
		if p.err != nil {
			continue
		}
		stage := staging(home, p.repo)
		if p.from != stage {
			prune(filepath.Dir(p.from), home)
		}
		if err := place(stage, p.dir); err != nil {
			p.err = err
			continue
		}
		prune(filepath.Dir(stage), filepath.Join(home, ".local", "state", "dctl"))
	}
}

func place(stage, dir string) error {
	dirs, err := hollow(dir, nil)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, d := range slices.Backward(dirs) {
		if err := os.Remove(d); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if err := renameNoReplace(stage, dir); err != nil {
		return &os.LinkError{Op: "rename", Old: stage, New: dir, Err: err}
	}
	return nil
}

func prune(dir, stop string) {
	for ; inside(stop, dir); dir = filepath.Dir(dir) {
		st, err := os.Lstat(dir)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil || !st.IsDir() || os.Remove(dir) != nil:
			return
		}
	}
}

func Stage(root paths.Root, run execx.Runner) setup.Stage {
	return setup.Stage{Name: "repos", Items: []setup.Item{{
		Name: "repos-cloned",
		Check: func(ctx context.Context) error {
			plans, err := survey(ctx, run, root)
			if err != nil {
				return err
			}
			var missing, problems, manual []string
			for _, p := range plans {
				switch {
				case p.err != nil:
					problems = append(problems, fmt.Sprintf("%s: %v", p.repo.Repo, p.err))
				case p.from != "":
					problems = append(problems, fmt.Sprintf("%s: move %s to %s", p.repo.Repo, show(root.Home, p.from), p.repo.Path))
				case p.held != "":
					manual = append(manual, p.repo.Repo+": "+p.held)
				case p.target.kind == absent:
					missing = append(missing, p.repo.Repo)
				case p.target.kind == broken:
					manual = append(manual, p.repo.Repo+": "+action(p.repo, p.target.reason))
				}
			}
			if len(missing) > 0 {
				problems = slices.Insert(problems, 0, fmt.Sprintf("%d repos %s: %s", len(missing), Absent, strings.Join(missing, ", ")))
			}
			switch {
			case len(problems) > 0:
				return errors.New(strings.Join(append(problems, manual...), "\n"))
			case len(manual) > 0:
				return setup.Manual("%s", strings.Join(manual, "\n"))
			}
			return nil
		},
		Fix: func(ctx context.Context) error {
			plans, err := survey(ctx, run, root)
			if err != nil {
				return err
			}
			relocate(root.Home, plans)
			var errs []error
			var manual []string
			for _, p := range plans {
				if err := ctx.Err(); err != nil {
					return errors.Join(append(errs, err)...)
				}
				switch {
				case p.err != nil:
					errs = append(errs, fmt.Errorf("%s: %w", p.repo.Repo, p.err))
					continue
				case p.held != "":
					manual = append(manual, p.repo.Repo+": "+p.held)
					continue
				case p.from != "" || p.target.kind == usable:
					continue
				}
				c, err := inspect(ctx, run, p.dir, p.repo.URL())
				switch {
				case err != nil:
					errs = append(errs, fmt.Errorf("%s: %w", p.repo.Repo, err))
				case c.kind == broken:
					manual = append(manual, p.repo.Repo+": "+action(p.repo, c.reason))
				case c.kind == absent:
					if err := clone(ctx, run, p.repo, p.dir); errors.Is(err, execx.ErrStalled) {
						return errors.Join(append(errs, fmt.Errorf("%s: clone stalled; skipped the remaining repos: %w", p.repo.Repo, err))...)
					} else if err != nil {
						errs = append(errs, fmt.Errorf("%s: clone: %w", p.repo.Repo, err))
					}
				}
			}
			switch {
			case len(errs) > 0:
				if len(manual) > 0 {
					errs = append(errs, errors.New(strings.Join(manual, "\n")))
				}
				return errors.Join(errs...)
			case len(manual) > 0:
				return setup.Manual("%s", strings.Join(manual, "\n"))
			}
			return nil
		},
	}, {
		Name: "repos-history",
		Check: func(ctx context.Context) error {
			switch partial, err := shallow(ctx, run, root); {
			case err != nil:
				return err
			case partial:
				return errors.New("dotfiles has only the commit the ISO installed")
			}
			return nil
		},
		Fix: func(ctx context.Context) error {
			if partial, err := shallow(ctx, run, root); err != nil || !partial {
				return err
			}
			fetch := execx.Remote(execx.Reason(run, "fetch the history the ISO left out"))
			return fetch.Run(ctx, root.Dotfiles, "git", "fetch", "--unshallow", "--progress")
		},
	}}}
}

func shallow(ctx context.Context, run execx.Runner, root paths.Root) (bool, error) {
	out, err := run.Output(ctx, root.Dotfiles, "git", "rev-parse", "--is-shallow-repository")
	return out == "true", err
}
