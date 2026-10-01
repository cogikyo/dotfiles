// Package repos manages repositories declared in repos.json.
//
// Responsibilities:
// - Clone missing repositories into configured paths.
// - Fast-forward clean repositories with upstreams.
// - Avoid destructive repair of dirty, detached, or diverged checkouts.
package repos

// repos.go defines repo manifest loading, sync, update, and checkout safety helpers.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
)

type Cmd struct {
	Sync   SyncCmd   `cmd:"" help:"Clone missing repos."`
	Update UpdateCmd `cmd:"" help:"Fast-forward configured repos."`
}
type SyncCmd struct{}
type UpdateCmd struct{}

type Repo struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
	Path string `json:"path"`
}

func ManifestPath(root paths.Root) string {
	return filepath.Join(root.Dotfiles, "repos.json")
}

// LoadManifest reads repos.json and returns repos sorted by name.
//
// Each entry must define name, repo, and path.
func LoadManifest(path string) ([]Repo, error) {
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

	slices.SortFunc(repos, func(a, b Repo) int { return strings.Compare(a.Name, b.Name) })
	for _, repo := range repos {
		if repo.Name == "" || repo.Repo == "" || repo.Path == "" {
			return nil, fmt.Errorf("incomplete repo entry in %s: %#v", path, repo)
		}
	}
	return repos, nil
}

func Sync(ctx context.Context, root paths.Root, out *ui.UI, runner execx.Runner) error {
	repos, err := LoadManifest(ManifestPath(root))
	if err != nil {
		return fmt.Errorf("load repos manifest: %w", err)
	}
	if runner == nil {
		runner = execx.OSRunner{}
	}

	for _, dir := range standardDirs(root.Home) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := switchDotfilesRemote(ctx, root, runner, out); err != nil {
		return err
	}
	ensureGithubKnownHost(ctx, root, runner, out)

	cloned, skipped, failed := 0, 0, 0
	for _, repo := range repos {
		target := paths.ExpandHome(root.Home, repo.Path)
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			skipped++
			continue
		} else if err == nil {
			out.Warn("%s exists but is not a directory", target)
			failed++
			continue
		} else if !os.IsNotExist(err) {
			out.Warn("cannot inspect %s: %v", target, err)
			failed++
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out.Info("Cloning %s -> %s", repo.Repo, repo.Path)
		if _, err := runner.Run(ctx, "", "git", "clone", "git@github.com:"+repo.Repo+".git", target); err != nil {
			out.Warn("failed to clone %s: %v", repo.Name, err)
			failed++
			continue
		}
		cloned++
	}

	if err := linkVagariNvim(root); err != nil {
		return err
	}
	out.OK("Cloned %d repos (%d already exist)", cloned, skipped)
	if failed > 0 {
		return fmt.Errorf("%d repo(s) failed", failed)
	}
	return nil
}

// Update fast-forwards configured repos without repairing divergent or dirty states.
func Update(ctx context.Context, root paths.Root, out *ui.UI, runner execx.Runner) error {
	repos, err := LoadManifest(ManifestPath(root))
	if err != nil {
		return fmt.Errorf("load repos manifest: %w", err)
	}
	if runner == nil {
		runner = execx.OSRunner{}
	}
	updated, skipped, failed := 0, 0, 0
	for _, repo := range repos {
		target := paths.ExpandHome(root.Home, repo.Path)
		if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
			out.Step("%s - not cloned, skipping", repo.Name)
			skipped++
			continue
		}
		branch, err := runner.Output(ctx, target, "git", "symbolic-ref", "--short", "HEAD")
		if err != nil || strings.TrimSpace(branch) == "" {
			out.Step("%s - detached HEAD, skipping", repo.Name)
			skipped++
			continue
		}
		dirty := ""
		if status, _ := runner.Output(ctx, target, "git", "status", "--porcelain"); strings.TrimSpace(status) != "" {
			dirty = " (dirty)"
		}
		if _, err := runner.Run(ctx, target, "git", "fetch", "--quiet"); err != nil {
			out.Step("%s - fetch failed%s", repo.Name, dirty)
			failed++
			continue
		}
		if _, err := runner.Output(ctx, target, "git", "rev-parse", "--verify", "@{upstream}"); err != nil {
			out.Step("%s - no upstream configured, skipping", repo.Name)
			skipped++
			continue
		}
		localRev, _ := runner.Output(ctx, target, "git", "rev-parse", "HEAD")
		upstreamRev, _ := runner.Output(ctx, target, "git", "rev-parse", "@{upstream}")
		if strings.TrimSpace(localRev) == strings.TrimSpace(upstreamRev) {
			out.Step("%s - up to date%s", repo.Name, dirty)
			updated++
			continue
		}
		if _, err := runner.Run(ctx, target, "git", "merge", "--ff-only"); err != nil {
			out.Step("%s - diverged, cannot fast-forward%s", repo.Name, dirty)
			failed++
			continue
		}
		out.Step("%s - updated%s", repo.Name, dirty)
		updated++
	}
	out.OK("Updated %d repos (%d skipped, %d failed)", updated, skipped, failed)
	if failed > 0 {
		return fmt.Errorf("%d repo(s) failed", failed)
	}
	return nil
}

func Check(ctx context.Context, root paths.Root, runner execx.Runner) []string {
	repos, err := LoadManifest(ManifestPath(root))
	if err != nil {
		return []string{err.Error()}
	}
	if runner == nil {
		runner = execx.OSRunner{}
	}
	var problems []string
	for _, repo := range repos {
		target := paths.ExpandHome(root.Home, repo.Path)
		if _, err := os.Stat(target); err != nil {
			problems = append(problems, repo.Name+": missing")
			continue
		}
		if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
			problems = append(problems, repo.Name+": not a git repo")
			continue
		}
		_, _ = runner.Output(ctx, target, "git", "symbolic-ref", "--short", "HEAD")
	}
	return problems
}

func standardDirs(home string) []string {
	return []string{
		filepath.Join(home, "downloads"),
		filepath.Join(home, "documents"),
		filepath.Join(home, "media", "screenshots"),
		filepath.Join(home, "media", "recordings"),
		filepath.Join(home, "media", "images"),
		filepath.Join(home, "media", "gifs"),
		filepath.Join(home, "agents"),
	}
}

func switchDotfilesRemote(ctx context.Context, root paths.Root, runner execx.Runner, out *ui.UI) error {
	remote, err := runner.Output(ctx, root.Dotfiles, "git", "remote", "get-url", "origin")
	if err != nil || !strings.HasPrefix(remote, "https://") {
		return nil
	}
	sshURL := strings.Replace(strings.TrimSpace(remote), "https://github.com/", "git@github.com:", 1)
	if _, err := runner.Run(ctx, root.Dotfiles, "git", "remote", "set-url", "origin", sshURL); err != nil {
		return err
	}
	out.OK("Dotfiles remote: %s", sshURL)
	return nil
}

func ensureGithubKnownHost(ctx context.Context, root paths.Root, runner execx.Runner, out *ui.UI) {
	sshDir := filepath.Join(root.Home, ".ssh")
	knownHosts := filepath.Join(sshDir, "known_hosts")
	if _, err := runner.Run(ctx, "", "ssh-keygen", "-F", "github.com", "-f", knownHosts); err == nil {
		return
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		out.Warn("cannot create ~/.ssh: %v", err)
		return
	}
	res, err := runner.Run(ctx, "", "ssh-keyscan", "-H", "github.com")
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		return
	}
	f, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		out.Warn("cannot update known_hosts: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(res.Stdout + "\n")
}

func linkVagariNvim(root paths.Root) error {
	src := filepath.Join(root.Home, "vagari", "nvim")
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return nil
	}
	dst := filepath.Join(root.Home, "nvim", "vagari.nvim")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	_ = os.Remove(dst)
	return os.Symlink(src, dst)
}

func (c *SyncCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return Sync(ctx, root, u, nil)
}
func (c *UpdateCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return Update(ctx, root, u, nil)
}
