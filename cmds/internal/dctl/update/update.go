// Package update manages Arch package updates and saved package lists.
//
// Responsibilities:
// - Update via yay.
// - Remove non-optional orphan packages.
// - Save explicit repo and AUR package lists back into the dotfiles repo.
package update

// update.go defines update commands, package-list parsing, dry-run planning, and install flows.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/app"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/output"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/pkglist"
)

type Cmd struct {
	RunCmd  RunCmd     `cmd:"" default:"1" name:"run" help:"Update system, remove orphans, and save package lists."`
	Install InstallCmd `cmd:"" help:"Install packages from saved lists."`
	Check   CheckCmd   `cmd:"" help:"Check replaceable -git packages."`
	DryRun  bool       `help:"Show what would change."`
}

type RunCmd struct {
	DryRun bool `help:"Show what would change."`
}
type InstallCmd struct {
	DryRun bool `help:"Show what would be installed."`
}
type CheckCmd struct{}

var parentDryRun bool

func (c *Cmd) AfterApply() error {
	parentDryRun = c.DryRun
	return nil
}

type Options struct {
	DryRun         bool
	NonInteractive bool
	PacmanConf     string
}

type replacement struct {
	GitPackage  string
	Alternative string
	Reason      string
}

var replacements = []replacement{
	{"dunst-git", "dunst", "repo version is newer (extra repo)"},
	{"logiops-git", "logiops", "stable AUR release, no build needed"},
	{"mpvpaper-git", "mpvpaper", "stable AUR release, no build needed"},
}

func (c *RunCmd) Run(ctx *app.Context) error {
	return Run(ctx.Context, ctx.Root, ctx.Output, execx.OSRunner{}, Options{DryRun: c.DryRun || parentDryRun})
}

func (c *InstallCmd) Run(ctx *app.Context) error {
	dryRun := c.DryRun || parentDryRun
	noninteractive := dryRun || os.Getenv("DOTFILES_INSTALL_NONINTERACTIVE") == "1" || !isTerminal(os.Stdin)
	return Install(ctx.Context, ctx.Root, ctx.Output, execx.OSRunner{}, Options{DryRun: dryRun, NonInteractive: noninteractive})
}

func (c *CheckCmd) Run(ctx *app.Context) error {
	return Check(ctx.Context, ctx.Root, ctx.Output, execx.OSRunner{})
}

// Run updates the current system, removes non-optional orphans, and saves package lists.
//
// Dry-run prints the package manager actions and list writes without mutating the system or repo files.
func Run(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, opts Options) error {
	if err := requireYay(ctx, runner); err != nil {
		return err
	}
	optional, err := readPackageList(root.Etc("packages-optional.lst"))
	if err != nil {
		return err
	}

	updateCmd := []string{"-Syu"}

	if opts.DryRun {
		out.Info("[dry-run] Would update system with: yay %s", strings.Join(updateCmd, " "))
	} else {
		out.Step("Updating system...")
		if _, err := runner.Run(ctx, "", "yay", updateCmd...); err != nil {
			return err
		}
	}

	orphans, err := commandLines(ctx, runner, "yay", "-Qdtq")
	if err != nil {
		orphans = nil
	}
	orphans, kept := filterOptional(orphanPackageNames(orphans), optional)
	for _, pkg := range kept {
		out.Warn("Keeping orphan %s (in packages-optional.lst)", pkg)
	}
	if opts.DryRun {
		if len(orphans) == 0 {
			out.Info("[dry-run] No orphans to remove")
		} else {
			out.Info("[dry-run] Would remove orphans:")
			printPackages(out, "-", orphans)
		}
		return drySaveLists(ctx, root, out, runner, optional)
	}

	out.Step("Removing orphaned packages...")
	if len(orphans) > 0 {
		args := append([]string{"-Rns"}, orphans...)
		if _, err := runner.Run(ctx, "", "yay", args...); err != nil {
			return err
		}
	} else {
		out.OK("No orphans found")
	}
	return saveLists(ctx, root, out, runner, optional)
}

// Install installs packages from saved lists.
//
// Dry-run reports missing packages only and skips sudo validation and package manager writes.
func Install(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, opts Options) error {
	if opts.PacmanConf == "" {
		opts.PacmanConf = "/etc/pacman.conf"
	}
	if !opts.DryRun {
		if _, err := runner.Run(ctx, "", "sudo", "-v"); err != nil {
			return fmt.Errorf("sudo access required for package installation: %w", err)
		}
	}

	repoList := root.Etc("packages.lst")
	if _, err := os.Stat(repoList); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("package list not found: %s", repoList)
		}
		return err
	}
	repoPkgs, err := readPackageList(repoList)
	if err != nil {
		return err
	}
	aurPkgs, err := readPackageList(root.Etc("packages-aur.lst"))
	if err != nil {
		return err
	}
	installed, err := installedPackages(ctx, runner)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return dryInstall(out, repoPkgs, aurPkgs, installed)
	}

	hasLocalRepo, cached := detectLocalRepo(ctx, runner, opts.PacmanConf)
	if hasLocalRepo {
		out.OK("Local package cache detected (%d packages) - installing from cache", cached)
	}
	if len(repoPkgs) > 0 {
		out.Step("Installing and upgrading %d repo packages...", len(repoPkgs))
		args := []string{"pacman", "-Syu", "--needed", "--noconfirm"}
		args = append(args, repoPkgs...)
		if _, err := runner.Run(ctx, "", "sudo", args...); err != nil {
			return err
		}
		out.OK("Repo packages done")
	}

	if rustupNeedsInit(ctx, runner) {
		out.Step("Initializing Rust toolchain via rustup...")
		if _, err := runner.Run(ctx, "", "rustup", "default", "stable"); err != nil {
			return err
		}
		out.OK("Rust stable toolchain installed")
	}
	if len(aurPkgs) > 0 {
		if err := installAUR(ctx, out, runner, aurPkgs, hasLocalRepo, opts.NonInteractive); err != nil {
			return err
		}
	}
	out.OK("Package installation complete!")
	out.Info("Remaining post-install steps:")
	out.Dim("Run: ./install.sh")
	out.Dim("Or:  ./install.sh all")
	return nil
}

func Check(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner) error {
	_ = root
	if err := requireYay(ctx, runner); err != nil {
		return err
	}
	out.Step("Checking for -git packages with better alternatives...")
	found := false
	for _, repl := range replacements {
		if isInstalled(ctx, runner, repl.GitPackage) {
			out.Warn("%s -> %s (%s)", repl.GitPackage, repl.Alternative, repl.Reason)
			found = true
		}
	}
	if !found {
		out.OK("No replaceable -git packages found")
		return nil
	}
	out.Info("To replace, run:")
	out.Dim("yay -S <alternative>  # will prompt to remove the -git version")
	return nil
}

func saveLists(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, optional []string) error {
	oldRepo, _ := readPackageList(root.Etc("packages.lst"))
	oldAUR, _ := readPackageList(root.Etc("packages-aur.lst"))
	repo, err := commandLines(ctx, runner, "yay", "-Qenq")
	if err != nil {
		return err
	}
	aur, err := commandLines(ctx, runner, "yay", "-Qemq")
	if err != nil {
		return err
	}
	repo, _ = filterOptional(cleanPackageNames(repo), optional)
	aur, _ = filterOptional(cleanPackageNames(aur), optional)
	if err := writePackageList(root.Etc("packages.lst"), repo); err != nil {
		return err
	}
	if err := writePackageList(root.Etc("packages-aur.lst"), aur); err != nil {
		return err
	}
	out.OK("Saved %d repo packages -> etc/packages.lst", len(repo))
	out.OK("Saved %d AUR packages  -> etc/packages-aur.lst", len(aur))
	printDiff(out, "repo", oldRepo, repo)
	printDiff(out, "AUR", oldAUR, aur)
	return nil
}

func drySaveLists(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, optional []string) error {
	repo, err := commandLines(ctx, runner, "yay", "-Qenq")
	if err != nil {
		return err
	}
	aur, err := commandLines(ctx, runner, "yay", "-Qemq")
	if err != nil {
		return err
	}
	repo, _ = filterOptional(cleanPackageNames(repo), optional)
	aur, _ = filterOptional(cleanPackageNames(aur), optional)
	out.Info("[dry-run] Would save package lists:")
	out.Dim("Repo (explicit): %d packages -> etc/packages.lst", len(repo))
	out.Dim("AUR  (explicit): %d packages -> etc/packages-aur.lst", len(aur))
	_ = root
	return nil
}

func dryInstall(out *output.Printer, repoPkgs, aurPkgs []string, installed map[string]bool) error {
	out.Info("[dry-run] Would install from saved lists:")
	repoMissing := filterMissing(repoPkgs, installed)
	if len(repoMissing) == 0 {
		out.OK("All repo packages already installed")
	} else {
		out.Info("Repo packages to install (%d):", len(repoMissing))
		printPackages(out, "+", repoMissing)
	}
	aurMissing := filterMissing(aurPkgs, installed)
	if len(aurMissing) == 0 {
		out.OK("All AUR packages already installed")
	} else {
		out.Info("AUR packages to install (%d):", len(aurMissing))
		printPackages(out, "+", aurMissing)
	}
	return nil
}

func installAUR(ctx context.Context, out *output.Printer, runner execx.Runner, pkgs []string, localRepo bool, noninteractive bool) error {
	base := []string{}
	name := "yay"
	if localRepo {
		name = "sudo"
		base = []string{"pacman", "-S", "--needed", "--noconfirm"}
	} else {
		if err := requireYay(ctx, runner); err != nil {
			return err
		}
		base = []string{"-S", "--needed"}
		if noninteractive {
			base = append(base, "--noconfirm")
		}
	}
	installed := 0
	skipped := 0
	failed := []string{}
	for i, pkg := range pkgs {
		if isInstalled(ctx, runner, pkg) {
			skipped++
			continue
		}
		out.Step("[%d/%d] Installing %s...", i+1, len(pkgs), pkg)
		args := append(slices.Clone(base), pkg)
		if _, err := runner.Run(ctx, "", name, args...); err != nil {
			out.Error("Failed to install AUR package: %s", pkg)
			failed = append(failed, pkg)
			continue
		}
		installed++
	}
	out.OK("AUR packages: %d installed, %d already present", installed, skipped)
	if len(failed) > 0 {
		out.Warn("Failed AUR packages (%d):", len(failed))
		printPackages(out, "-", failed)
		return fmt.Errorf("failed AUR packages: %s", strings.Join(failed, ", "))
	}
	return nil
}

func requireYay(ctx context.Context, runner execx.Runner) error {
	if _, err := runner.Run(ctx, "", "yay", "--version"); err != nil {
		return fmt.Errorf("yay not found. Run install.sh packages first")
	}
	return nil
}

func commandLines(ctx context.Context, runner execx.Runner, name string, args ...string) ([]string, error) {
	out, err := runner.Output(ctx, "", name, args...)
	if err != nil {
		return nil, err
	}
	return pkglist.ParseString(out), nil
}

func readPackageList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return pkglist.ParseString(string(data)), nil
}

func writePackageList(path string, pkgs []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(pkgs, "\n")+"\n"), 0o644)
}

func parsePackageList(data string) []string {
	return pkglist.ParseString(data)
}

func cleanPackageNames(lines []string) []string {
	return pkglist.Unique(lines)
}

func orphanPackageNames(lines []string) []string {
	return cleanPackageNames(lines)
}

func filterOptional(pkgs, optional []string) ([]string, []string) {
	opt := packageSet(optional)
	out := []string{}
	kept := []string{}
	for _, pkg := range pkgs {
		if opt[pkg] {
			kept = append(kept, pkg)
			continue
		}
		out = append(out, pkg)
	}
	return out, kept
}

func filterMissing(pkgs []string, installed map[string]bool) []string {
	out := []string{}
	for _, pkg := range pkgs {
		if !installed[pkg] {
			out = append(out, pkg)
		}
	}
	return out
}

func installedPackages(ctx context.Context, runner execx.Runner) (map[string]bool, error) {
	out, err := commandLines(ctx, runner, "pacman", "-Qq")
	if err != nil {
		return map[string]bool{}, err
	}
	return packageSet(out), nil
}

func packageSet(pkgs []string) map[string]bool {
	set := map[string]bool{}
	for _, pkg := range pkgs {
		set[pkg] = true
	}
	return set
}

func isInstalled(ctx context.Context, runner execx.Runner, pkg string) bool {
	_, err := runner.Run(ctx, "", "pacman", "-Qq", pkg)
	return err == nil
}

func detectLocalRepo(ctx context.Context, runner execx.Runner, pacmanConf string) (bool, int) {
	data, err := os.ReadFile(pacmanConf)
	if err != nil || !hasLocalRepoConfig(string(data)) {
		return false, 0
	}
	lines, err := commandLines(ctx, runner, "pacman", "-Sl", "localrepo")
	if err != nil {
		return true, 0
	}
	return true, len(lines)
}

func hasLocalRepoConfig(data string) bool {
	for line := range strings.SplitSeq(data, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if strings.TrimSpace(line) == "[localrepo]" {
			return true
		}
	}
	return false
}

func rustupNeedsInit(ctx context.Context, runner execx.Runner) bool {
	if _, err := runner.Run(ctx, "", "rustup", "--version"); err != nil {
		return false
	}
	_, err := runner.Run(ctx, "", "rustup", "show", "active-toolchain")
	return err != nil
}

func printDiff(out *output.Printer, kind string, oldPkgs, newPkgs []string) {
	oldSet := packageSet(oldPkgs)
	newSet := packageSet(newPkgs)
	added := []string{}
	removed := []string{}
	for _, pkg := range newPkgs {
		if !oldSet[pkg] {
			added = append(added, pkg)
		}
	}
	for _, pkg := range oldPkgs {
		if !newSet[pkg] {
			removed = append(removed, pkg)
		}
	}
	if len(added) > 0 {
		out.Info("New %s packages:", kind)
		printPackages(out, "+", added)
	}
	if len(removed) > 0 {
		out.Info("Removed %s packages:", kind)
		printPackages(out, "-", removed)
	}
}

func printPackages(out *output.Printer, prefix string, pkgs []string) {
	for _, pkg := range pkgs {
		out.Dim("%s %s", prefix, pkg)
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
