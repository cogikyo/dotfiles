// Package install implements dotfiles install steps and their healthchecks.
//
// Responsibilities:
// - Plan and apply user-level symlinks without clobbering unknown directories.
// - Run package, service, Firefox, DNS, and build steps behind explicit commands.
// - Provide dry-run paths for install steps with file or system side effects.
package install

// install.go defines install command wiring, step implementations, and shared install helpers.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/health"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/pkglist"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/ui"
	"dotfiles/cmds/internal/dctl/update"
)

type Cmd struct {
	All      AllCmd      `cmd:"" help:"Run all install steps."`
	Packages PackagesCmd `cmd:"" help:"Install packages from saved lists."`
	Link     LinkCmd     `cmd:"" help:"Symlink configs and scripts."`
	Secrets  SecretsCmd  `cmd:"" help:"Decrypt secrets."`
	Repos    ReposCmd    `cmd:"" help:"Clone repositories."`
	System   SystemCmd   `cmd:"" help:"Install system configs."`
	Fonts    FontsCmd    `cmd:"" help:"Install fonts."`
	Go       GoCmd       `cmd:"" name:"go" help:"Build Go binaries."`
	Eww      EwwCmd      `cmd:"" help:"Build eww."`
	Firefox  FirefoxCmd  `cmd:"" help:"Configure Firefox."`
	Certs    CertsCmd    `cmd:"" help:"Provision development TLS certificates."`
	Shell    ShellCmd    `cmd:"" help:"Set login shell."`
	DNS      DNSCmd      `cmd:"" name:"dns" help:"Configure DNS-over-TLS."`
	List     ListCmd     `cmd:"" help:"List install steps."`
	Check    CheckCmd    `cmd:"" help:"Run healthchecks."`
}

type Options struct{ Yes, Optional, DryRun bool }
type AllCmd struct{ Optional, DryRun bool }
type StepCmd struct {
	Optional bool `help:"Install optional packages too."`
	DryRun   bool `help:"Show planned changes."`
}
type PackagesCmd StepCmd
type LinkCmd StepCmd
type SecretsCmd StepCmd
type ReposCmd StepCmd
type SystemCmd StepCmd
type FontsCmd StepCmd
type GoCmd StepCmd
type EwwCmd StepCmd
type FirefoxCmd StepCmd
type CertsCmd StepCmd
type ShellCmd StepCmd
type DNSCmd StepCmd
type ListCmd struct{}
type CheckCmd struct {
	Steps []string `arg:"" optional:"" help:"Specific steps to check."`
}

type StepDef struct {
	Name, Description string
	Risk              string
	FixCommand        string
	Sudo              bool
	SupportsDryRun    bool
	Depends           []string
}

// stepDefs is the install contract exposed by `dctl install list` and `dctl install all`.
//
// Risk is user-facing metadata for command pickers and JSON/list output.
// Sudo marks steps that may need root privileges.
// SupportsDryRun is only true when the step implementation avoids writes and external side effects.
// Depends documents ordering assumptions but is not a dependency solver.
var stepDefs = []StepDef{
	{Name: "packages", Description: "Install packages from saved lists", Risk: "package manager changes", FixCommand: "dctl update install --dry-run", Sudo: true, SupportsDryRun: true},
	{Name: "link", Description: "Symlink configs and scripts", Risk: "home symlink changes", FixCommand: "dctl install link --dry-run", SupportsDryRun: true},
	{Name: "secrets", Description: "Decrypt age-encrypted secrets", Risk: "writes secret targets", FixCommand: "dctl secrets decrypt --dry-run"},
	{Name: "repos", Description: "Clone repositories and create directories", Risk: "network and filesystem changes", FixCommand: "dctl repos sync", Depends: []string{"secrets"}},
	{Name: "system", Description: "Install system configs and enable services", Risk: "writes /etc, /boot, and service state", FixCommand: "dctl install system --dry-run", Sudo: true, SupportsDryRun: true, Depends: []string{"link"}},
	{Name: "fonts", Description: "Extract fonts and optionally build Iosevka", Risk: "writes user font cache", FixCommand: "dctl install fonts --dry-run", SupportsDryRun: true},
	{Name: "go", Description: "Build Go binaries", Risk: "writes built binaries and user services", FixCommand: "dctl install go --dry-run", SupportsDryRun: true},
	{Name: "eww", Description: "Install eww widget system", Risk: "clones/builds eww and overwrites ~/.local/bin/eww", FixCommand: "dctl install eww --dry-run", SupportsDryRun: true},
	{Name: "firefox", Description: "Configure Firefox profile, theme, and preferences", Risk: "writes Firefox profile links", FixCommand: "dctl install firefox --dry-run", SupportsDryRun: true, Depends: []string{"repos"}},
	{Name: "certs", Description: "Provision the local development CA and shared certificate", Risk: "modifies system and Firefox trust stores and writes a private key", FixCommand: "dctl install certs --dry-run", Sudo: true, SupportsDryRun: true},
	{Name: "shell", Description: "Change default shell to zsh", Risk: "changes login shell", FixCommand: "dctl install shell --dry-run", Sudo: true, SupportsDryRun: true},
	{Name: "dns", Description: "Set up systemd-resolved with Cloudflare DNS-over-TLS", Risk: "replaces resolver config and restarts networking", FixCommand: "dctl install dns --dry-run", Sudo: true, SupportsDryRun: true, Depends: []string{"system"}},
}

var steps = stepNames(stepDefs)

func stepNames(defs []StepDef) []string {
	out := make([]string, 0, len(defs))
	for _, def := range defs {
		out = append(out, def.Name)
	}
	return out
}
func FindStep(name string) (StepDef, bool) {
	for _, def := range stepDefs {
		if def.Name == name {
			return def, true
		}
	}
	return StepDef{}, false
}

func (c *AllCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if c.DryRun {
		for _, def := range stepDefs {
			if !def.SupportsDryRun {
				u.Warn("[dry-run] Skipping install %s: dry-run is not supported", def.Name)
			}
		}
	}
	for _, def := range stepDefs {
		if c.DryRun && !def.SupportsDryRun {
			continue
		}
		step := def.Name
		if err := RunStep(ctx, root, u, step, Options{Yes: u.Yes(), Optional: c.Optional, DryRun: c.DryRun}); err != nil {
			return err
		}
	}
	return nil
}
func (c *PackagesCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "packages", StepCmd(*c))
}
func (c *LinkCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "link", StepCmd(*c))
}
func (c *SecretsCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "secrets", StepCmd(*c))
}
func (c *ReposCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "repos", StepCmd(*c))
}
func (c *SystemCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "system", StepCmd(*c))
}
func (c *FontsCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "fonts", StepCmd(*c))
}
func (c *GoCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "go", StepCmd(*c))
}
func (c *EwwCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "eww", StepCmd(*c))
}
func (c *FirefoxCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "firefox", StepCmd(*c))
}
func (c *CertsCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "certs", StepCmd(*c))
}
func (c *ShellCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "shell", StepCmd(*c))
}
func (c *DNSCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return run(ctx, u, root, "dns", StepCmd(*c))
}
func run(ctx context.Context, u *ui.UI, root paths.Root, name string, cmd StepCmd) error {
	return RunStep(ctx, root, u, name, Options{Yes: u.Yes(), Optional: cmd.Optional, DryRun: cmd.DryRun})
}
func (c *ListCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	if u.JSON() {
		return u.Emit(stepDefs)
	}
	for _, def := range stepDefs {
		u.Info("%-10s %s", def.Name, def.Description)
		u.KV("risk", def.Risk)
		u.KV("dry-run", def.SupportsDryRun)
		if def.Sudo {
			u.KV("sudo", true)
		}
		if len(def.Depends) > 0 {
			u.KV("depends", strings.Join(def.Depends, ", "))
		}
		if def.FixCommand != "" {
			u.KV("check", def.FixCommand)
		}
	}
	return nil
}
func (c *CheckCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	return Check(ctx, root, u, c.Steps)
}

func RunStep(ctx context.Context, root paths.Root, out *ui.UI, name string, opts Options) error {
	def, ok := FindStep(name)
	if !ok {
		return fmt.Errorf("unknown install step %s", name)
	}
	if opts.DryRun && !def.SupportsDryRun {
		return fmt.Errorf("install %s does not support --dry-run", name)
	}
	runner := execx.OSRunner{IO: true}
	switch name {
	case "packages":
		return installPackages(ctx, root, out, opts, execx.OSRunner{IO: !opts.DryRun})
	case "link":
		return installLink(ctx, root, out, opts)
	case "secrets":
		return installSecrets(ctx, root, out)
	case "repos":
		out.Header("Cloning repositories and creating directories")
		return repos.Sync(ctx, root, out, runner)
	case "system":
		return installSystem(ctx, root, out, opts, runner)
	case "go":
		return installGo(ctx, root, out, opts, runner)
	case "fonts":
		return installFonts(ctx, root, out, opts, runner)
	case "eww":
		return installEww(ctx, root, out, opts, runner)
	case "firefox":
		return installFirefox(ctx, root, out, opts)
	case "certs":
		return installCerts(ctx, root, out, opts, runner)
	case "shell":
		return installShell(ctx, root, out, opts, runner)
	case "dns":
		return installDNS(ctx, root, out, opts, runner)
	}
	return fmt.Errorf("unknown install step %s", name)
}

func Check(ctx context.Context, root paths.Root, out *ui.UI, selected []string) error {
	if len(selected) == 0 {
		selected = steps
	}
	var checks []health.Check
	for _, step := range selected {
		checks = append(checks, healthFor(ctx, root, step)...)
	}
	if err := health.Print(out, checks); err != nil {
		return err
	}
	if health.HasFailure(checks) {
		return errors.New("healthcheck failed")
	}
	return nil
}

type LinkAction string

const (
	LinkUnchanged LinkAction = "unchanged"
	LinkCreate    LinkAction = "create"
	LinkReplace   LinkAction = "replace"
	LinkBackup    LinkAction = "backup"
	LinkRefuse    LinkAction = "refuse"
)

type LinkPlan struct {
	Source, Target string
	Action         LinkAction
	Reason         string
}
type fileMapping struct{ src, dst string }

func PlanSymlink(source, target, repoRoot string) (LinkPlan, error) {
	plan := LinkPlan{Source: source, Target: target, Action: LinkCreate}
	src, err := filepath.EvalSymlinks(source)
	if err != nil {
		return plan, err
	}
	st, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		return plan, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		dst, err := filepath.EvalSymlinks(target)
		if err == nil && dst == src {
			plan.Action = LinkUnchanged
			return plan, nil
		}
		plan.Action = LinkReplace
		plan.Reason = "symlink points elsewhere"
		return plan, nil
	}
	if st.IsDir() {
		managed, err := repoManagedDir(target, repoRoot)
		if err != nil {
			return plan, err
		}
		if managed {
			plan.Action = LinkBackup
			plan.Reason = "directory contains only repo-managed links"
			return plan, nil
		}
		plan.Action = LinkRefuse
		plan.Reason = "real directory may contain user data"
		return plan, nil
	}
	plan.Action = LinkBackup
	plan.Reason = "existing file will be backed up"
	return plan, nil
}

// repoManagedDir reports whether a real directory can be replaced safely.
//
// Only empty directories or directories containing symlinks back into this repo are considered managed.
func repoManagedDir(dir, repoRoot string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return true, nil
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		st, err := os.Lstat(p)
		if err != nil {
			return false, err
		}
		if st.Mode()&os.ModeSymlink == 0 {
			return false, nil
		}
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			return false, nil
		}
		if real != repoRoot && !strings.HasPrefix(real, repoRoot+string(os.PathSeparator)) {
			return false, nil
		}
	}
	return true, nil
}

func installLink(ctx context.Context, root paths.Root, out *ui.UI, opts Options) error {
	out.Header("Linking configs and scripts")
	mappings, err := linkMappings(root)
	if err != nil {
		return err
	}
	updated, unchanged := 0, 0
	for _, m := range mappings {
		plan, err := PlanSymlink(m.src, m.dst, root.Dotfiles)
		if err != nil {
			return err
		}
		if err := applyLink(plan, out, opts); err != nil {
			return err
		}
		if plan.Action == LinkUnchanged {
			unchanged++
		} else {
			updated++
		}
		if !opts.DryRun {
			if err := verifySymlink(m.src, m.dst); err != nil {
				return err
			}
		}
	}
	if err := ensureOBSScene(root, opts); err != nil {
		return err
	}
	script := filepath.Join(root.Dotfiles, "skills", "link.sh")
	if !opts.DryRun {
		if _, err := os.Stat(script); err == nil {
			if _, err := (execx.OSRunner{IO: true}).Run(ctx, root.Dotfiles, script, "user"); err != nil {
				return err
			}
		}
	}
	out.OK("Linking complete (updated: %d, unchanged: %d)", updated, unchanged)
	return nil
}
func linkMappings(root paths.Root) ([]fileMapping, error) {
	var out []fileMapping
	items, err := os.ReadDir(root.Config())
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		name := item.Name()
		if slices.Contains([]string{"firefox", "obs-studio"}, name) {
			continue
		}
		out = append(out, fileMapping{root.Config(name), filepath.Join(root.Home, ".config", name)})
	}
	out = append(out, fileMapping{root.Config("obs-studio", "basic", "profiles", "Costello", "basic.ini"), filepath.Join(root.Home, ".config", "obs-studio", "basic", "profiles", "Costello", "basic.ini")}, fileMapping{root.Config("zsh", "zshrc"), filepath.Join(root.Home, ".zshrc")}, fileMapping{root.Config("zsh", "zshenv"), filepath.Join(root.Home, ".zshenv")})
	bin, err := os.ReadDir(root.Bin())
	if err == nil {
		for _, item := range bin {
			if item.Type().IsRegular() {
				src := root.Bin(item.Name())
				_ = os.Chmod(src, 0o755)
				out = append(out, fileMapping{src, filepath.Join(root.Home, ".local", "bin", item.Name())})
			}
		}
	}
	return out, nil
}

// applyLink refuses real directories unless PlanSymlink proved they only contain repo-managed links.
//
// Dry-run prints the planned action and does not create, rename, remove, or symlink anything.
func applyLink(plan LinkPlan, out *ui.UI, opts Options) error {
	if plan.Action == LinkUnchanged {
		return nil
	}
	if plan.Action == LinkRefuse {
		return fmt.Errorf("refusing to replace %s: %s", plan.Target, plan.Reason)
	}
	if opts.DryRun {
		out.Info("[dry-run] %s %s -> %s", plan.Action, plan.Target, plan.Source)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(plan.Target), 0o755); err != nil {
		return err
	}
	if plan.Action == LinkBackup {
		if err := os.Rename(plan.Target, plan.Target+".backup."+time.Now().Format("20060102-150405")); err != nil {
			return err
		}
	} else if plan.Action == LinkReplace {
		if err := os.Remove(plan.Target); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return os.Symlink(plan.Source, plan.Target)
}
func verifySymlink(src, dst string) error {
	sr, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	dr, err := filepath.EvalSymlinks(dst)
	if err != nil {
		return err
	}
	if sr != dr {
		return fmt.Errorf("%s is not linked to %s", dst, src)
	}
	return nil
}
func ensureOBSScene(root paths.Root, opts Options) error {
	dst := filepath.Join(root.Home, ".config", "obs-studio", "basic", "scenes", "Costello.json")
	if opts.DryRun {
		return nil
	}
	if st, err := os.Lstat(dst); err == nil && st.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	return copyFile(root.Config("obs-studio", "basic", "scenes", "Costello.json"), dst, 0o644)
}

type goBinary struct {
	name, moduleDir, buildPath, outputDir string
	daemon                                bool
}

var goBinaries = []goBinary{{"dctl", "cmds", "./cmd/dctl", "", false}, {"hyprd", "cmds", "./cmd/hyprd", "", true}, {"ewwd", "cmds", "./cmd/ewwd", "", false}, {"newtab", "cmds", "./cmd/newtab", "", true}, {"src", "cmds", "./cmd/src", "", false}}

func installGo(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Building Go binaries")
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go not found; install packages first")
	}
	failed := 0
	for _, b := range goBinaries {
		if err := buildGo(ctx, root, out, opts, runner, b); err != nil {
			out.Warn("%s build failed: %v", b.name, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d Go binary build(s) failed", failed)
	}
	return installGoServices(ctx, root, out, opts)
}
func buildGo(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner, b goBinary) error {
	dir := filepath.Join(root.Dotfiles, b.moduleDir)
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return fmt.Errorf("module not found: %s", dir)
	}
	installDir := filepath.Join(root.Home, ".local", "bin")
	if b.outputDir != "" {
		installDir = filepath.Join(root.Dotfiles, b.outputDir)
	}
	if opts.DryRun {
		out.Info("[dry-run] go build -o %s %s", filepath.Join(installDir, b.name), b.buildPath)
		return nil
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	_, err := runner.Run(ctx, dir, "go", "build", "-o", filepath.Join(installDir, b.name), b.buildPath)
	return err
}
func installGoServices(ctx context.Context, root paths.Root, out *ui.UI, opts Options) error {
	dir := filepath.Join(root.Home, ".config", "systemd", "user")
	if opts.DryRun {
		out.Info("[dry-run] Would install user service files into %s", dir)
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, b := range goBinaries {
		if !b.daemon {
			continue
		}
		src := filepath.Join(root.Dotfiles, b.moduleDir, "cmd", b.name, b.name+".service")
		if _, err := os.Stat(src); err != nil {
			src = filepath.Join(root.Dotfiles, b.moduleDir, b.name+".service")
		}
		if _, err := os.Stat(src); err != nil {
			out.Warn("service file not found for %s", b.name)
			continue
		}
		if err := copyFile(src, filepath.Join(dir, b.name+".service"), 0o644); err != nil {
			return err
		}
	}
	runner := execx.OSRunner{}
	if _, err := runner.Run(ctx, "", "systemctl", "--user", "daemon-reload"); err != nil {
		out.Warn("skipping user service reload: %v", err)
		return nil
	}
	for _, b := range goBinaries {
		if !b.daemon {
			continue
		}
		if b.name == "hyprd" {
			_, _ = runner.Run(ctx, "", "systemctl", "--user", "disable", b.name)
			continue
		}
		_, _ = runner.Run(ctx, "", "systemctl", "--user", "enable", "--now", b.name)
	}
	return nil
}

func installFonts(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Installing fonts")
	archive := root.Share("fonts.tar.gz")
	if _, err := os.Stat(archive); err != nil {
		return fmt.Errorf("font archive not found: %s", archive)
	}
	fontDir := filepath.Join(root.Home, ".local", "share", "fonts")
	if !dirPopulated(fontDir) || opts.Yes {
		if opts.DryRun {
			out.Info("[dry-run] Would extract %s into ~/.local/share", archive)
			return nil
		}
		if err := os.MkdirAll(filepath.Join(root.Home, ".local", "share"), 0o755); err != nil {
			return err
		}
		if _, err := runner.Run(ctx, "", "tar", "-xzf", archive, "-C", filepath.Join(root.Home, ".local", "share")); err != nil {
			return err
		}
	} else {
		out.OK("Font directory already populated: %s", fontDir)
	}
	if !opts.DryRun {
		if _, err := runner.Run(ctx, "", "fc-cache", "-f"); err != nil {
			return err
		}
	}
	out.OK("Fonts installed")
	return nil
}
func dirPopulated(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func installFirefox(ctx context.Context, root paths.Root, out *ui.UI, opts Options) error {
	out.Header("Configuring Firefox")
	profile, err := detectFirefoxProfile(root.Home)
	if err != nil {
		return err
	}
	css, err := filepath.Glob(filepath.Join(root.Home, "vagari", "firefox", "css", "*"))
	if err != nil || len(css) == 0 {
		return errors.New("vagari.firefox CSS missing; run repos first")
	}
	chrome := filepath.Join(profile, "chrome")
	if opts.DryRun {
		out.Info("[dry-run] Would link Firefox CSS into %s and user.js into %s", chrome, profile)
		return nil
	}
	if err := os.MkdirAll(chrome, 0o755); err != nil {
		return err
	}
	for _, src := range css {
		plan, err := PlanSymlink(src, filepath.Join(chrome, filepath.Base(src)), root.Dotfiles)
		if err != nil {
			return err
		}
		if err := applyLink(plan, out, opts); err != nil {
			return err
		}
	}
	plan, err := PlanSymlink(root.Config("firefox", "user.js"), filepath.Join(profile, "user.js"), root.Dotfiles)
	if err != nil {
		return err
	}
	if err := applyLink(plan, out, opts); err != nil {
		return err
	}
	out.OK("Firefox configured")
	out.Warn("Restart Firefox for changes to take effect")
	_ = ctx
	return nil
}
func detectFirefoxProfile(home string) (string, error) {
	for _, root := range []string{filepath.Join(home, ".mozilla", "firefox"), filepath.Join(home, ".config", "mozilla", "firefox")} {
		p, err := profileFromINI(root)
		if err == nil {
			return p, nil
		}
	}
	return "", errors.New("Firefox Developer Edition profile not found; launch it once first")
}
func profileFromINI(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "profiles.ini"))
	if err != nil {
		return "", err
	}
	name, path := "", ""
	flush := func() (string, bool) {
		if name == "dev-edition-default" && path != "" {
			p := path
			if !filepath.IsAbs(p) {
				p = filepath.Join(root, p)
			}
			st, err := os.Stat(p)
			return p, err == nil && st.IsDir()
		}
		return "", false
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			if p, ok := flush(); ok {
				return p, nil
			}
			name, path = "", ""
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "Name" {
			name = strings.TrimSpace(v)
		} else if strings.TrimSpace(k) == "Path" {
			path = strings.TrimSpace(v)
		}
	}
	if p, ok := flush(); ok {
		return p, nil
	}
	return "", errors.New("dev-edition-default profile missing")
}

func installPackages(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Installing packages")
	if err := update.Install(ctx, root, out, runner, update.Options{NonInteractive: opts.Yes, DryRun: opts.DryRun}); err != nil {
		return err
	}
	if !opts.Optional {
		return nil
	}
	optional, err := readSimpleList(root.Packages("extra.lst"))
	if errors.Is(err, os.ErrNotExist) || len(optional) == 0 {
		out.Warn("Optional package list not found or empty")
		return nil
	}
	if opts.DryRun {
		out.Info("[dry-run] Would install %d optional packages", len(optional))
		return nil
	}
	args := append([]string{"-S", "--needed"}, optional...)
	_, err = runner.Run(ctx, "", "yay", args...)
	return err
}

func installSecrets(ctx context.Context, root paths.Root, out *ui.UI) error {
	out.Header("Decrypting secrets")
	cmd := secrets.DecryptCmd{}
	return cmd.Run(out, root)
}

var systemFiles = []string{
	"/etc/bluetooth/main.conf",
	"/etc/libinput/local-overrides.quirks",
	"/etc/logid.cfg",
	"/etc/modules-load.d/i2c-dev.conf",
	"/etc/nftables.conf",
	"/etc/pacman.d/hooks/firefox-autoconfig.hook",
	"/etc/pam.d/hyprlock",
	"/etc/sddm.conf.d/autologin.conf",
	"/etc/security/faillock.conf",
	"/etc/sysctl.d/99-memory-pressure.conf",
	"/etc/systemd/resolved.conf",
	"/etc/systemd/system.conf.d/cpu-lanes.conf",
	"/etc/systemd/system/bluetooth.service.d/cpu-lane.conf",
	"/etc/systemd/system/earlyoom.service.d/memory-pressure.conf",
	"/etc/systemd/system/logid-restart.service",
	"/etc/systemd/system/logid.service.d/restart.conf",
	"/etc/systemd/system/rtkit-daemon.service.d/cpu-lane.conf",
	"/etc/systemd/zram-generator.conf",
	"/etc/udev/rules.d/81-bluetooth-hci.rules",
	"/etc/udev/rules.d/91-logid-restart.rules",
	"/etc/udev/rules.d/92-viia.rules",
	"/usr/lib/firefox-developer-edition/defaults/pref/autoconfig.js",
	"/usr/lib/firefox-developer-edition/firefox.cfg",
}

func installSystem(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Installing system configs")
	if err := confirmRisk("install system files and enable services", opts); err != nil {
		return err
	}
	installed, skipped := 0, 0
	for _, dst := range systemFiles {
		src := root.System(dst)
		if _, err := os.Stat(src); err != nil {
			out.Warn("Source missing: %s", src)
			continue
		}
		if sameSystemFileBytes(ctx, runner, src, dst) {
			skipped++
			continue
		}
		out.Info("%s -> %s", src, dst)
		if !opts.DryRun {
			if _, err := runner.Run(ctx, "", "sudo", "mkdir", "-p", filepath.Dir(dst)); err != nil {
				return err
			}
			if _, err := runner.Run(ctx, "", "sudo", "cp", src, dst); err != nil {
				return err
			}
		}
		installed++
	}
	out.OK("Installed %d system configs (%d already up to date)", installed, skipped)
	if opts.DryRun {
		return nil
	}
	if _, err := runner.Run(ctx, "", "sudo", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "sudo", "sysctl", "--system"); err != nil {
		out.Warn("sysctl settings were installed but not applied: %v", err)
	}
	var failures []string
	for _, svc := range []string{"bluetooth", "sddm", "earlyoom", "logid", "tailscaled", "nftables"} {
		if _, err := runner.Run(ctx, "", "sudo", "systemctl", "enable", svc); err != nil {
			out.Warn("enable %s failed: %v", svc, err)
			failures = append(failures, "enable "+svc)
		}
	}
	for _, svc := range []string{"bluetooth", "earlyoom", "logid", "tailscaled", "nftables"} {
		if _, err := runner.Run(ctx, "", "sudo", "systemctl", "start", svc); err != nil {
			out.Warn("start %s failed: %v", svc, err)
			failures = append(failures, "start "+svc)
		}
	}
	if _, err := runner.Run(ctx, "", "sudo", "systemctl", "restart", "earlyoom"); err != nil {
		out.Warn("restart earlyoom failed: %v", err)
		failures = append(failures, "restart earlyoom")
	}
	if _, err := runner.Run(ctx, "", "tailscale", "version"); err == nil {
		out.Info("Enabling Tailscale SSH")
		if _, err := runner.Run(ctx, "", "sudo", "tailscale", "set", "--ssh=true"); err != nil {
			out.Warn("Tailscale SSH not enabled; run 'sudo tailscale set --ssh=true' after logging in: %v", err)
		} else {
			out.OK("Tailscale SSH enabled")
		}
	} else {
		out.Warn("tailscale command not found; install tailscale first")
	}
	if _, err := runner.Run(ctx, "", "sudo", "udevadm", "control", "--reload-rules"); err != nil {
		failures = append(failures, "udev reload")
	}
	if _, err := runner.Run(ctx, "", "sudo", "udevadm", "trigger"); err != nil {
		failures = append(failures, "udev trigger")
	}
	if len(failures) > 0 {
		return fmt.Errorf("system post-install actions failed: %s", strings.Join(failures, ", "))
	}
	return nil
}

func installEww(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Installing eww")
	cache := filepath.Join(root.Home, ".cache", "eww")
	if opts.DryRun {
		out.Info("[dry-run] Would clone/update eww, apply patch, cargo build --release, install ~/.local/bin/eww")
		return nil
	}
	if _, err := os.Stat(cache); errors.Is(err, os.ErrNotExist) {
		if _, err := runner.Run(ctx, filepath.Dir(cache), "git", "clone", "https://github.com/elkowar/eww.git", cache); err != nil {
			return err
		}
	} else {
		status, err := runner.Output(ctx, cache, "git", "status", "--porcelain")
		if err != nil {
			return err
		}
		if strings.TrimSpace(status) != "" && !opts.Yes {
			return fmt.Errorf("eww cache has local changes at %s; rerun with --yes to discard", cache)
		}
		if strings.TrimSpace(status) != "" {
			if _, err := runner.Run(ctx, cache, "git", "checkout", "--", "."); err != nil {
				return err
			}
		}
		if _, err := runner.Run(ctx, cache, "git", "pull", "--ff-only"); err != nil {
			return err
		}
	}
	if _, err := runner.Run(ctx, cache, "git", "apply", root.Packages("eww", "poll-interval.patch")); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, cache, "cargo", "build", "--release", "--locked"); err != nil {
		return err
	}
	outPath := filepath.Join(root.Home, ".local", "bin", "eww")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "strip", filepath.Join(cache, "target", "release", "eww")); err != nil {
		out.Warn("strip failed: %v", err)
	}
	return copyFile(filepath.Join(cache, "target", "release", "eww"), outPath, 0o755)
}

func installShell(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Changing shell")
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		if opts.DryRun {
			out.Info("[dry-run] Would install zsh with pacman")
		} else if _, err := runner.Run(ctx, "", "sudo", "pacman", "-S", "--needed", "--noconfirm", "zsh"); err != nil {
			return err
		}
		zsh = "/usr/bin/zsh"
	}
	if opts.DryRun {
		out.Info("[dry-run] Would run chsh -s %s %s", zsh, os.Getenv("USER"))
		return nil
	}
	_, err = runner.Run(ctx, "", "chsh", "-s", zsh)
	_ = root
	return err
}

func installDNS(ctx context.Context, root paths.Root, out *ui.UI, opts Options, runner execx.Runner) error {
	out.Header("Configuring DNS")
	if err := confirmRisk("replace /etc/resolv.conf and restart resolved/NetworkManager", opts); err != nil {
		return err
	}
	if opts.DryRun {
		out.Info("[dry-run] Would install resolved.conf, NetworkManager DNS drop-in, and symlink resolv.conf")
		return nil
	}
	if _, err := runner.Run(ctx, "", "sudo", "mkdir", "-p", "/etc/NetworkManager/conf.d"); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "sudo", "cp", root.System("etc", "systemd", "resolved.conf"), "/etc/systemd/resolved.conf"); err != nil {
		return err
	}
	if err := writeRootFile(ctx, runner, "/etc/NetworkManager/conf.d/10-dotfiles-dns.conf", networkManagerDNSDropin()); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "sudo", "ln", "-sfn", "/run/systemd/resolve/stub-resolv.conf", "/etc/resolv.conf"); err != nil {
		return err
	}
	_, _ = runner.Run(ctx, "", "sudo", "systemctl", "enable", "--now", "systemd-resolved")
	_, _ = runner.Run(ctx, "", "sudo", "systemctl", "restart", "NetworkManager")
	return nil
}

func confirmRisk(action string, opts Options) error {
	if opts.Yes || opts.DryRun {
		return nil
	}
	return errors.New(action + " requires --yes or --dry-run")
}

func readSimpleList(path string) ([]string, error) {
	return pkglist.Read(path)
}

func sameFileBytes(a, b string) bool {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	bb, err := os.ReadFile(b)
	return err == nil && string(ab) == string(bb)
}

func sameSystemFileBytes(ctx context.Context, runner execx.Runner, a, b string) bool {
	if sameFileBytes(a, b) {
		return true
	}
	_, err := runner.Run(ctx, "", "sudo", "cmp", "-s", a, b)
	return err == nil
}

func writeRootFile(ctx context.Context, runner execx.Runner, path string, content string) error {
	tmp, err := os.CreateTemp("", "dctl-root-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "sudo", "mkdir", "-p", filepath.Dir(path)); err != nil {
		return err
	}
	_, err = runner.Run(ctx, "", "sudo", "cp", tmp.Name(), path)
	return err
}

func executable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode()&0o111 != 0
}
func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func networkManagerDNSDropin() string { return "[main]\ndns=systemd-resolved\n" }
