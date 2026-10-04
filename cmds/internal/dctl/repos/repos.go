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

func clone(ctx context.Context, run execx.Runner, r Repo, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	return run.Run(ctx, "", "git", "clone", r.URL(), dir)
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
	Absent    State = "not cloned; run dctl setup repos"
)

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
	var errs []error
	for _, r := range repos {
		st, err := os.Stat(r.Dir(root.Home))
		if err == nil && slices.ContainsFunc(checkouts, func(c os.FileInfo) bool { return os.SameFile(c, st) }) {
			u.Dim("%s: skipped; update never pulls the dotfiles checkout", r.Repo)
			continue
		}
		state, err := update(ctx, run, r.Dir(root.Home))
		switch {
		case err != nil:
			u.Row(ui.Err, r.Repo+": "+err.Error())
			errs = append(errs, fmt.Errorf("%s: %w", r.Repo, err))
		case state == Current || state == Forwarded:
			u.OK("%s: %s", r.Repo, state)
		default:
			u.Warn("%s: %s", r.Repo, state)
		}
	}
	return errors.Join(errs...)
}

func update(ctx context.Context, run execx.Runner, dir string) (State, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return Absent, nil
	}
	git := func(args ...string) (string, error) { return run.Output(ctx, dir, "git", args...) }
	exited := func(err error, s State) (State, error) {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
			return s, nil
		}
		return "", err
	}
	if _, err := git("symbolic-ref", "-q", "HEAD"); err != nil {
		return exited(err, Detached)
	}
	status, err := git("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", err
	}
	if status != "" {
		return Dirty, nil
	}
	if _, err := git("rev-parse", "--verify", "-q", "@{upstream}"); err != nil {
		return exited(err, Untracked)
	}
	if _, err := git("fetch", "--quiet"); err != nil {
		return "", err
	}
	counts, err := git("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return "", err
	}
	left, right, _ := strings.Cut(counts, "\t")
	ahead, err := strconv.Atoi(left)
	if err != nil {
		return "", fmt.Errorf("parse rev-list counts %q: %w", counts, err)
	}
	behind, err := strconv.Atoi(right)
	if err != nil {
		return "", fmt.Errorf("parse rev-list counts %q: %w", counts, err)
	}
	switch {
	case behind == 0 && ahead == 0:
		return Current, nil
	case behind == 0:
		return Ahead, nil
	case ahead > 0:
		return Diverged, nil
	}
	if _, err := git("merge", "--ff-only", "--quiet", "@{upstream}"); err != nil {
		return "", err
	}
	return Forwarded, nil
}

func Stage(root paths.Root, run execx.Runner) setup.Stage {
	return setup.Stage{Name: "repos", Items: []setup.Item{{
		Name: "repos-cloned",
		Check: func(ctx context.Context) error {
			repos, err := Read(list(root))
			if err != nil {
				return err
			}
			var problems []string
			for _, r := range repos {
				dir := r.Dir(root.Home)
				if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
					problems = append(problems, fmt.Sprintf("%s: not cloned at %s", r.Repo, r.Path))
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
					problems = append(problems, fmt.Sprintf("%s: %s is not a git checkout", r.Repo, r.Path))
					continue
				}
				origin, err := run.Output(ctx, dir, "git", "remote", "get-url", "origin")
				switch {
				case err != nil:
					problems = append(problems, fmt.Sprintf("%s: no origin remote", r.Repo))
				case origin != r.URL():
					problems = append(problems, fmt.Sprintf("%s: origin is %s, want %s", r.Repo, origin, r.URL()))
				}
			}
			if len(problems) == 0 {
				return nil
			}
			return errors.New(strings.Join(problems, "; "))
		},
		Fix: func(ctx context.Context) error {
			repos, err := Read(list(root))
			if err != nil {
				return err
			}
			var errs []error
			for _, r := range repos {
				dir := r.Dir(root.Home)
				if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err := clone(ctx, run, r, dir); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", r.Repo, err))
				}
			}
			return errors.Join(errs...)
		},
	}}}
}
