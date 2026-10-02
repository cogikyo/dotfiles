package repos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
)

type Repo struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
	Path string `json:"path"`
}

func (r Repo) URL() string { return "git@github.com:" + r.Repo + ".git" }

func (r Repo) Dir(home string) string { return paths.ExpandHome(home, r.Path) }

func Load(path string) ([]Repo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var repos []Repo
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&repos); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s: trailing data after the repo list", path)
	}
	names, dirs := map[string]bool{}, map[string]bool{}
	for i, r := range repos {
		switch {
		case r.Name == "" || r.Repo == "" || r.Path == "":
			return nil, fmt.Errorf("%s: entry %d needs name, repo, and path", path, i)
		case !github.MatchString(r.Repo) || strings.HasSuffix(r.Repo, "/.") || strings.HasSuffix(r.Repo, "/.."):
			return nil, fmt.Errorf("%s: %s: repo %q must be a GitHub owner/name", path, r.Name, r.Repo)
		case !strings.HasPrefix(r.Path, "~/") && !filepath.IsAbs(r.Path):
			return nil, fmt.Errorf("%s: %s: path %q must start with ~/ or /", path, r.Name, r.Path)
		case names[r.Name]:
			return nil, fmt.Errorf("%s: duplicate name %q", path, r.Name)
		case dirs[filepath.Clean(r.Path)]:
			return nil, fmt.Errorf("%s: duplicate path %q", path, r.Path)
		}
		names[r.Name], dirs[filepath.Clean(r.Path)] = true, true
	}
	return repos, nil
}

var github = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

func load(root paths.Root) ([]Repo, error) {
	if os.Geteuid() == 0 {
		return nil, errors.New("run as your user: git as root would run hooks and config from user-owned repos")
	}
	return Load(filepath.Join(root.Dotfiles, "repos.json"))
}

func Sync(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner) error {
	repos, err := load(root)
	if err != nil {
		return err
	}
	var errs []error
	cloned := 0
	for _, r := range repos {
		dir := r.Dir(root.Home)
		if _, err := os.Lstat(dir); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		u.Step("clone %s -> %s", r.Repo, r.Path)
		if err := clone(ctx, run, r, dir); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
			continue
		}
		cloned++
	}
	u.OK("cloned %d of %d repos", cloned, len(repos))
	return errors.Join(errs...)
}

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
	Absent    State = "not cloned; run dctl repos sync"
)

func Update(ctx context.Context, u *ui.UI, root paths.Root, run execx.Runner) error {
	repos, err := load(root)
	if err != nil {
		return err
	}
	var errs []error
	for _, r := range repos {
		state, err := update(ctx, run, r.Dir(root.Home))
		switch {
		case err != nil:
			u.Row(ui.Err, r.Name+": "+err.Error())
			errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
		case state == Current || state == Forwarded:
			u.OK("%s: %s", r.Name, state)
		default:
			u.Warn("%s: %s", r.Name, state)
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

func Group(root paths.Root, run execx.Runner) doctor.Group {
	return doctor.Group{Name: "repos", Online: true, Checks: []doctor.Check{{
		Name: "repos-cloned",
		Check: func(ctx context.Context) error {
			repos, err := Load(filepath.Join(root.Dotfiles, "repos.json"))
			if err != nil {
				return err
			}
			var problems []string
			for _, r := range repos {
				dir := r.Dir(root.Home)
				if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
					problems = append(problems, fmt.Sprintf("%s: not cloned at %s", r.Name, r.Path))
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
					problems = append(problems, fmt.Sprintf("%s: %s is not a git checkout", r.Name, r.Path))
					continue
				}
				origin, err := run.Output(ctx, dir, "git", "remote", "get-url", "origin")
				switch {
				case err != nil:
					problems = append(problems, fmt.Sprintf("%s: no origin remote", r.Name))
				case origin != r.URL():
					problems = append(problems, fmt.Sprintf("%s: origin is %s, want %s", r.Name, origin, r.URL()))
				}
			}
			if len(problems) == 0 {
				return nil
			}
			return errors.New(strings.Join(problems, "; "))
		},
		Fix: func(ctx context.Context) error {
			repos, err := Load(filepath.Join(root.Dotfiles, "repos.json"))
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
					errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
				}
			}
			return errors.Join(errs...)
		},
	}}}
}
