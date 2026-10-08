package repos

import (
	"bufio"
	"context"
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

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"

	"golang.org/x/sys/unix"
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
	if err := run.Run(ctx, "", "git", "clone", r.URL(), work); err != nil {
		return err
	}
	c, err := inspect(ctx, run, work, r.URL())
	switch {
	case err != nil:
		return err
	case c.kind != usable:
		return fmt.Errorf("fresh clone is %s (%s)", Broken, c.reason)
	}
	if err := unix.Renameat2(unix.AT_FDCWD, work, unix.AT_FDCWD, dir, unix.RENAME_NOREPLACE); err != nil {
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
	var missing, failed []string
	for _, r := range repos {
		if err := ctx.Err(); err != nil {
			return err
		}
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
		switch res.state {
		case Absent:
			missing = append(missing, r.Repo)
			continue
		case Broken, Failed:
			failed = append(failed, r.Repo)
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
	if _, err := git("fetch", "--quiet"); err != nil {
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

func Stage(root paths.Root, run execx.Runner) setup.Stage {
	return setup.Stage{Name: "repos", Items: []setup.Item{{
		Name: "repos-cloned",
		Check: func(ctx context.Context) error {
			repos, err := Read(list(root))
			if err != nil {
				return err
			}
			var missing, problems, moves []string
			for _, r := range repos {
				c, err := inspect(ctx, run, r.Dir(root.Home), r.URL())
				switch {
				case err != nil:
					problems = append(problems, fmt.Sprintf("%s: %v", r.Repo, err))
				case c.kind == absent:
					missing = append(missing, r.Repo)
				case c.kind == broken:
					moves = append(moves, r.Repo+": "+action(r, c.reason))
				}
			}
			if len(missing) > 0 {
				problems = slices.Insert(problems, 0, fmt.Sprintf("%d repos %s: %s", len(missing), Absent, strings.Join(missing, ", ")))
			}
			switch {
			case len(problems) > 0:
				return errors.New(strings.Join(append(problems, moves...), "\n"))
			case len(moves) > 0:
				return setup.Manual("%s", strings.Join(moves, "\n"))
			}
			return nil
		},
		Fix: func(ctx context.Context) error {
			repos, err := Read(list(root))
			if err != nil {
				return err
			}
			var errs []error
			var moves []string
			for _, r := range repos {
				if err := ctx.Err(); err != nil {
					return errors.Join(append(errs, err)...)
				}
				dir := r.Dir(root.Home)
				c, err := inspect(ctx, run, dir, r.URL())
				switch {
				case err != nil:
					errs = append(errs, fmt.Errorf("%s: %w", r.Repo, err))
				case c.kind == broken:
					moves = append(moves, r.Repo+": "+action(r, c.reason))
				case c.kind == absent:
					if err := clone(ctx, run, r, dir); err != nil {
						errs = append(errs, fmt.Errorf("%s: clone: %w", r.Repo, err))
					}
				}
			}
			switch {
			case len(errs) > 0:
				if len(moves) > 0 {
					errs = append(errs, errors.New(strings.Join(moves, "\n")))
				}
				return errors.Join(errs...)
			case len(moves) > 0:
				return setup.Manual("%s", strings.Join(moves, "\n"))
			}
			return nil
		},
	}}}
}
