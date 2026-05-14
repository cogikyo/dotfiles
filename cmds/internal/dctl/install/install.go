// Package install implements dotfiles install steps and their healthchecks.
//
// Responsibilities:
// - Plan and apply user-level symlinks without clobbering unknown directories.
// - Run package, service, Firefox, DNS, hibernate, and build steps behind explicit commands.
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
	"regexp"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/app"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/health"
	"dotfiles/cmds/internal/dctl/output"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/pkglist"
	"dotfiles/cmds/internal/dctl/prompt"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/tui"
	"dotfiles/cmds/internal/dctl/update"
)

type Cmd struct {
	Choose    ChooseCmd    `cmd:"" default:"1" hidden:"" help:"Choose an install command."`
	All       AllCmd       `cmd:"" help:"Run all install steps."`
	Packages  PackagesCmd  `cmd:"" help:"Install packages from saved lists."`
	Link      LinkCmd      `cmd:"" help:"Symlink configs and scripts."`
	Secrets   SecretsCmd   `cmd:"" help:"Decrypt secrets."`
	Repos     ReposCmd     `cmd:"" help:"Clone repositories."`
	System    SystemCmd    `cmd:"" help:"Install system configs."`
	Hibernate HibernateCmd `cmd:"" help:"Configure hibernate."`
	Fonts     FontsCmd     `cmd:"" help:"Install fonts."`
	Go        GoCmd        `cmd:"" name:"go" help:"Build Go binaries."`
	Eww       EwwCmd       `cmd:"" help:"Build eww."`
	Firefox   FirefoxCmd   `cmd:"" help:"Configure Firefox."`
	Shell     ShellCmd     `cmd:"" help:"Set login shell."`
	DNS       DNSCmd       `cmd:"" name:"dns" help:"Configure DNS-over-TLS."`
	List      ListCmd      `cmd:"" help:"List install steps."`
	Check     CheckCmd     `cmd:"" help:"Run healthchecks."`
}

type Options struct{ Yes, Defaults, Optional, DryRun bool }
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
type HibernateCmd StepCmd
type FontsCmd StepCmd
type GoCmd StepCmd
type EwwCmd StepCmd
type FirefoxCmd StepCmd
type ShellCmd StepCmd
type DNSCmd StepCmd
type ListCmd struct{}
type ChooseCmd struct{}
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
	{Name: "hibernate", Description: "Configure swapfile and suspend-then-hibernate", Risk: "modifies swap, fstab, boot loader, and initramfs", FixCommand: "dctl install hibernate --dry-run", Sudo: true, SupportsDryRun: true},
	{Name: "fonts", Description: "Extract fonts and optionally build Iosevka", Risk: "writes user font cache", FixCommand: "dctl install fonts --dry-run", SupportsDryRun: true},
	{Name: "go", Description: "Build Go binaries", Risk: "writes built binaries and user services", FixCommand: "dctl install go --dry-run", SupportsDryRun: true},
	{Name: "eww", Description: "Install eww widget system", Risk: "clones/builds eww and overwrites ~/.local/bin/eww", FixCommand: "dctl install eww --dry-run", SupportsDryRun: true},
	{Name: "firefox", Description: "Configure Firefox profile, theme, and preferences", Risk: "writes Firefox profile links", FixCommand: "dctl install firefox --dry-run", SupportsDryRun: true, Depends: []string{"repos"}},
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

func (c *ChooseCmd) Run(ctx *app.Context) error {
	if !prompt.Interactive() || ctx.Defaults {
		return errors.New("install command required; use `dctl install list`, `dctl install all`, or a specific step")
	}
	argv, ok, err := tui.RunLauncher(os.Stdout, installCatalog())
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return execx.ExecDctl(append([]string{"install"}, argv...))
}

func installCatalog() tui.CommandCatalog {
	children := []tui.CommandNode{{Name: "all", Help: "Run all install steps."}}
	for _, def := range stepDefs {
		children = append(children, tui.CommandNode{Name: def.Name, Help: def.Description})
	}
	children = append(children,
		tui.CommandNode{Name: "list", Help: "List install steps."},
		tui.CommandNode{Name: "check", Help: "Run healthchecks."},
	)
	return tui.CommandNode{Name: "install", Children: children}
}

func (c *AllCmd) Run(ctx *app.Context) error {
	if c.DryRun {
		for _, def := range stepDefs {
			if !def.SupportsDryRun {
				ctx.Output.Warn("[dry-run] Skipping install %s: dry-run is not supported", def.Name)
			}
		}
	}
	for _, def := range stepDefs {
		if c.DryRun && !def.SupportsDryRun {
			continue
		}
		step := def.Name
		if err := RunStep(ctx.Context, ctx.Root, ctx.Output, step, Options{Yes: ctx.Yes, Defaults: ctx.Defaults, Optional: c.Optional, DryRun: c.DryRun}); err != nil {
			return err
		}
	}
	return nil
}
func (c *PackagesCmd) Run(ctx *app.Context) error  { return run(ctx, "packages", StepCmd(*c)) }
func (c *LinkCmd) Run(ctx *app.Context) error      { return run(ctx, "link", StepCmd(*c)) }
func (c *SecretsCmd) Run(ctx *app.Context) error   { return run(ctx, "secrets", StepCmd(*c)) }
func (c *ReposCmd) Run(ctx *app.Context) error     { return run(ctx, "repos", StepCmd(*c)) }
func (c *SystemCmd) Run(ctx *app.Context) error    { return run(ctx, "system", StepCmd(*c)) }
func (c *HibernateCmd) Run(ctx *app.Context) error { return run(ctx, "hibernate", StepCmd(*c)) }
func (c *FontsCmd) Run(ctx *app.Context) error     { return run(ctx, "fonts", StepCmd(*c)) }
func (c *GoCmd) Run(ctx *app.Context) error        { return run(ctx, "go", StepCmd(*c)) }
func (c *EwwCmd) Run(ctx *app.Context) error       { return run(ctx, "eww", StepCmd(*c)) }
func (c *FirefoxCmd) Run(ctx *app.Context) error   { return run(ctx, "firefox", StepCmd(*c)) }
func (c *ShellCmd) Run(ctx *app.Context) error     { return run(ctx, "shell", StepCmd(*c)) }
func (c *DNSCmd) Run(ctx *app.Context) error       { return run(ctx, "dns", StepCmd(*c)) }
func run(ctx *app.Context, name string, cmd StepCmd) error {
	return RunStep(ctx.Context, ctx.Root, ctx.Output, name, Options{Yes: ctx.Yes, Defaults: ctx.Defaults, Optional: cmd.Optional, DryRun: cmd.DryRun})
}
func (c *ListCmd) Run(ctx *app.Context) error {
	if ctx.Output.JSONMode() {
		return ctx.Output.Emit(stepDefs)
	}
	for _, def := range stepDefs {
		ctx.Output.Info("%-10s %s", def.Name, def.Description)
		ctx.Output.KV("risk", def.Risk)
		ctx.Output.KV("dry-run", def.SupportsDryRun)
		if def.Sudo {
			ctx.Output.KV("sudo", true)
		}
		if len(def.Depends) > 0 {
			ctx.Output.KV("depends", strings.Join(def.Depends, ", "))
		}
		if def.FixCommand != "" {
			ctx.Output.KV("check", def.FixCommand)
		}
	}
	return nil
}
func (c *CheckCmd) Run(ctx *app.Context) error {
	return Check(ctx.Context, ctx.Root, ctx.Output, c.Steps)
}

func RunStep(ctx context.Context, root paths.Root, out *output.Printer, name string, opts Options) error {
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
	case "hibernate":
		return installHibernate(ctx, root, out, opts, runner)
	case "go":
		return installGo(ctx, root, out, opts, runner)
	case "fonts":
		return installFonts(ctx, root, out, opts, runner)
	case "eww":
		return installEww(ctx, root, out, opts, runner)
	case "firefox":
		return installFirefox(ctx, root, out, opts)
	case "shell":
		return installShell(ctx, root, out, opts, runner)
	case "dns":
		return installDNS(ctx, root, out, opts, runner)
	}
	return fmt.Errorf("unknown install step %s", name)
}

func Check(ctx context.Context, root paths.Root, out *output.Printer, selected []string) error {
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

func installLink(ctx context.Context, root paths.Root, out *output.Printer, opts Options) error {
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
		if slices.Contains([]string{"claude", "firefox", "obs-studio"}, name) {
			continue
		}
		out = append(out, fileMapping{root.Config(name), filepath.Join(root.Home, ".config", name)})
	}
	out = append(out, fileMapping{root.Config("claude", "settings.json"), filepath.Join(root.Home, ".config", "claude", "settings.json")}, fileMapping{root.Config("opencode", "AGENTS.md"), filepath.Join(root.Home, ".claude", "CLAUDE.md")}, fileMapping{root.Config("obs-studio", "basic", "profiles", "Costello", "basic.ini"), filepath.Join(root.Home, ".config", "obs-studio", "basic", "profiles", "Costello", "basic.ini")}, fileMapping{root.Config("zsh", "zshrc"), filepath.Join(root.Home, ".zshrc")}, fileMapping{root.Config("zsh", "zshenv"), filepath.Join(root.Home, ".zshenv")})
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
func applyLink(plan LinkPlan, out *output.Printer, opts Options) error {
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

var goBinaries = []goBinary{{"dctl", "cmds", "./cmd/dctl", "", false}, {"hyprd", "cmds", "./cmd/hyprd", "", true}, {"ewwd", "cmds", "./cmd/ewwd", "", false}, {"statusline", "cmds", "./cmd/statusline", "", false}, {"newtab", "cmds", "./cmd/newtab", "", true}}

func installGo(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
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
func buildGo(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner, b goBinary) error {
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
func installGoServices(ctx context.Context, root paths.Root, out *output.Printer, opts Options) error {
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

func installFonts(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
	out.Header("Installing fonts")
	archive := root.Etc("fonts.tar.gz")
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

func installFirefox(ctx context.Context, root paths.Root, out *output.Printer, opts Options) error {
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
			p := filepath.Join(root, path)
			st, err := os.Stat(p)
			return p, err == nil && st.IsDir()
		}
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
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

func installPackages(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
	out.Header("Installing packages")
	if err := update.Install(ctx, root, out, runner, update.Options{NonInteractive: opts.Defaults || opts.Yes, DryRun: opts.DryRun}); err != nil {
		return err
	}
	if !opts.Optional {
		return nil
	}
	optional, err := readSimpleList(root.Etc("packages-optional.lst"))
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

func installSecrets(ctx context.Context, root paths.Root, out *output.Printer) error {
	out.Header("Decrypting secrets")
	cmd := secrets.DecryptCmd{}
	return cmd.Run(&app.Context{Context: ctx, Root: root, Output: out})
}

var systemFiles = map[string]string{
	"bluetooth/main.conf":                                     "/etc/bluetooth/main.conf",
	"udev/81-bluetooth-hci.rules":                             "/etc/udev/rules.d/81-bluetooth-hci.rules",
	"udev/91-logid-restart.rules":                             "/etc/udev/rules.d/91-logid-restart.rules",
	"udev/92-viia.rules":                                      "/etc/udev/rules.d/92-viia.rules",
	"sddm.conf.d/autologin.conf":                              "/etc/sddm.conf.d/autologin.conf",
	"pam.d/hyprlock":                                          "/etc/pam.d/hyprlock",
	"systemd/resolved.conf":                                   "/etc/systemd/resolved.conf",
	"systemd/sleep.conf.d/hibernate.conf":                     "/etc/systemd/sleep.conf.d/hibernate.conf",
	"systemd/hibernate-zram.conf":                             "/etc/systemd/system/systemd-hibernate.service.d/zram.conf",
	"systemd/system.conf.d/cpu-lanes.conf":                    "/etc/systemd/system.conf.d/cpu-lanes.conf",
	"systemd/bluetooth.service.d/cpu-lane.conf":               "/etc/systemd/system/bluetooth.service.d/cpu-lane.conf",
	"systemd/rtkit-daemon.service.d/cpu-lane.conf":            "/etc/systemd/system/rtkit-daemon.service.d/cpu-lane.conf",
	"security/faillock.conf":                                  "/etc/security/faillock.conf",
	"loader.conf":                                             "/boot/loader/loader.conf",
	"logid.cfg":                                               "/etc/logid.cfg",
	"systemd/logid.service.d/restart.conf":                    "/etc/systemd/system/logid.service.d/restart.conf",
	"systemd/logid-restart.service":                           "/etc/systemd/system/logid-restart.service",
	"libinput/local-overrides.quirks":                         "/etc/libinput/local-overrides.quirks",
	"nftables.conf":                                           "/etc/nftables.conf",
	"firefox-developer-edition/autoconfig.js":                 "/usr/lib/firefox-developer-edition/defaults/pref/autoconfig.js",
	"firefox-developer-edition/firefox.cfg":                   "/usr/lib/firefox-developer-edition/firefox.cfg",
	"pacman.d/hooks/firefox-autoconfig.hook":                  "/etc/pacman.d/hooks/firefox-autoconfig.hook",
	"systemd/hibernate-zram.conf#suspend-then-hibernate-zram": "/etc/systemd/system/systemd-suspend-then-hibernate.service.d/zram.conf",
}

func installSystem(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
	out.Header("Installing system configs")
	if err := confirmRisk("install system files and enable services", opts); err != nil {
		return err
	}
	installed, skipped := 0, 0
	keys := make([]string, 0, len(systemFiles))
	for k := range systemFiles {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, key := range keys {
		srcRel, _, _ := strings.Cut(key, "#")
		src := root.Etc(srcRel)
		dst := systemFiles[key]
		if _, err := os.Stat(src); err != nil {
			out.Warn("Source missing: %s", src)
			continue
		}
		if sameSystemFileBytes(ctx, runner, src, dst) {
			skipped++
			continue
		}
		out.SubStep("info", "%s -> %s", srcRel, dst)
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
	if _, err := runner.Run(ctx, "", "tailscale", "version"); err == nil {
		out.SubStep("info", "Enabling Tailscale SSH")
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

func installHibernate(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
	out.Header("Configuring hibernation")
	if err := confirmRisk("modify swap, fstab, boot loader, and initramfs", opts); err != nil {
		return err
	}
	fsType, _ := runner.Output(ctx, "", "findmnt", "-no", "FSTYPE", "/")
	if strings.TrimSpace(fsType) != "btrfs" {
		return fmt.Errorf("root filesystem is %s, not btrfs", strings.TrimSpace(fsType))
	}
	if opts.DryRun {
		out.Info("[dry-run] Would create /swap btrfs subvolume, swapfile, fstab entries, resume boot params, and rebuild initramfs")
		return nil
	}
	rootUUID, err := runner.Output(ctx, "", "findmnt", "-no", "UUID", "/")
	if err != nil {
		return err
	}
	rootDev, err := runner.Output(ctx, "", "findmnt", "-no", "SOURCE", "/")
	if err != nil {
		return err
	}
	rootDev, _, _ = strings.Cut(strings.TrimSpace(rootDev), "[")
	if err := ensureSwapSubvolume(ctx, runner, strings.TrimSpace(rootDev), strings.TrimSpace(rootUUID)); err != nil {
		return err
	}
	if err := ensureSwapfile(ctx, runner); err != nil {
		return err
	}
	resumeOffset, err := runner.Output(ctx, "", "sudo", "filefrag", "-v", "/swap/swapfile")
	if err != nil {
		return err
	}
	offset := parseFilefragOffset(resumeOffset)
	if offset == "" {
		return errors.New("could not determine resume_offset from filefrag")
	}
	if err := patchRootFile(ctx, runner, "/boot/loader/entries/arch.conf", func(s string) (string, error) {
		return updateLoaderResume(s, strings.TrimSpace(rootUUID), offset)
	}); err != nil {
		return err
	}
	if err := patchRootFile(ctx, runner, "/etc/mkinitcpio.conf", func(s string) (string, error) {
		out, _ := ensureResumeHook(s)
		return out, nil
	}); err != nil {
		return err
	}
	_, err = runner.Run(ctx, "", "sudo", "mkinitcpio", "-P")
	return err
}

func installEww(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
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
	if _, err := runner.Run(ctx, cache, "git", "apply", root.Etc("eww-poll-interval.patch")); err != nil {
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

func installShell(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
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

func installDNS(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
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
	if _, err := runner.Run(ctx, "", "sudo", "cp", root.Etc("systemd", "resolved.conf"), "/etc/systemd/resolved.conf"); err != nil {
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
	if opts.Yes || opts.Defaults || opts.DryRun {
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

func ensureSwapSubvolume(ctx context.Context, runner execx.Runner, rootDev, rootUUID string) error {
	tmp, err := os.MkdirTemp("", "dctl-btrfs-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err := runner.Run(ctx, "", "sudo", "mount", "-o", "subvolid=5", rootDev, tmp); err != nil {
		return err
	}
	defer runner.Run(ctx, "", "sudo", "umount", tmp)
	if _, err := os.Stat(filepath.Join(tmp, "@swap")); errors.Is(err, os.ErrNotExist) {
		if _, err := runner.Run(ctx, "", "sudo", "btrfs", "subvolume", "create", filepath.Join(tmp, "@swap")); err != nil {
			return err
		}
	}
	if err := patchRootFile(ctx, runner, "/etc/fstab", func(s string) (string, error) {
		return upsertSwapSubvolFstab(s, rootUUID, "/swap"), nil
	}); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "sudo", "mkdir", "-p", "/swap"); err != nil {
		return err
	}
	if _, err := runner.Run(ctx, "", "sudo", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err = runner.Run(ctx, "", "sudo", "mount", "/swap")
	return err
}

func ensureSwapfile(ctx context.Context, runner execx.Runner) error {
	if _, err := runner.Run(ctx, "", "sudo", "chattr", "+C", "/swap"); err != nil {
		return err
	}
	if _, err := os.Stat("/swap/swapfile"); errors.Is(err, os.ErrNotExist) {
		size := swapSize()
		if _, err := runner.Run(ctx, "", "sudo", "btrfs", "filesystem", "mkswapfile", "--size", size, "/swap/swapfile"); err != nil {
			if _, err := runner.Run(ctx, "", "sudo", "truncate", "-s", "0", "/swap/swapfile"); err != nil {
				return err
			}
			if _, err := runner.Run(ctx, "", "sudo", "chattr", "+C", "/swap/swapfile"); err != nil {
				return err
			}
			if _, err := runner.Run(ctx, "", "sudo", "dd", "if=/dev/zero", "of=/swap/swapfile", "bs=1G", "count="+strings.TrimSuffix(size, "G"), "status=progress"); err != nil {
				return err
			}
			if _, err := runner.Run(ctx, "", "sudo", "chmod", "600", "/swap/swapfile"); err != nil {
				return err
			}
			if _, err := runner.Run(ctx, "", "sudo", "mkswap", "/swap/swapfile"); err != nil {
				return err
			}
		}
	}
	if err := patchRootFile(ctx, runner, "/etc/fstab", func(s string) (string, error) {
		return upsertSwapFileFstab(s, "/swap/swapfile"), nil
	}); err != nil {
		return err
	}
	_, _ = runner.Run(ctx, "", "sudo", "swapon", "/swap/swapfile")
	return nil
}

func swapSize() string {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "32G"
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			var kb int
			if _, err := fmt.Sscanf(fields[1], "%d", &kb); err == nil && kb > 0 {
				gb := kb/1048576 + 1
				return fmt.Sprintf("%dG", gb)
			}
		}
	}
	return "32G"
}

func parseFilefragOffset(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.HasSuffix(fields[0], ":") && fields[3] != "0" {
			return strings.TrimSuffix(fields[3], "..")
		}
	}
	return ""
}

func patchRootFile(ctx context.Context, runner execx.Runner, path string, fn func(string) (string, error)) error {
	b, err := runner.Output(ctx, "", "sudo", "cat", path)
	if err != nil {
		return err
	}
	next, err := fn(b)
	if err != nil || next == b {
		return err
	}
	return writeRootFile(ctx, runner, path, next)
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

func upsertIgnorePkg(content string, pkgs []string) string {
	line := "IgnorePkg = " + strings.Join(pkgs, " ")
	lines := strings.Split(content, "\n")
	for n, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "IgnorePkg") || strings.HasPrefix(t, "#IgnorePkg") {
			lines[n] = line
			return strings.Join(lines, "\n")
		}
	}
	return appendLine(content, line)
}
func upsertSwapSubvolFstab(content, uuid, mount string) string {
	if strings.Contains(content, "subvol=/@swap") {
		return content
	}
	return appendLine(content, fmt.Sprintf("UUID=%s %s btrfs subvol=/@swap,noatime 0 0", uuid, mount))
}
func upsertSwapFileFstab(content, swapfile string) string {
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && !strings.HasPrefix(strings.TrimSpace(line), "#") && f[0] == swapfile && f[2] == "swap" {
			return content
		}
	}
	return appendLine(content, fmt.Sprintf("%s none swap defaults,pri=10 0 0", swapfile))
}
func appendLine(content, line string) string {
	if content == "" {
		return line + "\n"
	}
	if strings.HasSuffix(content, "\n") {
		return content + line + "\n"
	}
	return content + "\n" + line + "\n"
}
func updateLoaderResume(content, uuid, offset string) (string, error) {
	if uuid == "" || offset == "" {
		return "", errors.New("resume UUID and offset are required")
	}
	lines := strings.Split(content, "\n")
	found := false
	for n, line := range lines {
		if !strings.HasPrefix(line, "options") {
			continue
		}
		found = true
		fields := strings.Fields(line)
		kept := fields[:1]
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "resume=UUID=") || strings.HasPrefix(f, "resume_offset=") {
				continue
			}
			kept = append(kept, f)
		}
		lines[n] = strings.Join(append(kept, "resume=UUID="+uuid, "resume_offset="+offset), " ")
	}
	if !found {
		return "", errors.New("loader entry has no options line")
	}
	return strings.Join(lines, "\n"), nil
}
func ensureResumeHook(content string) (string, bool) {
	re := regexp.MustCompile(`HOOKS=\(([^)]*)\)`)
	loc := re.FindStringSubmatchIndex(content)
	if loc == nil {
		return content, false
	}
	fields := strings.Fields(content[loc[2]:loc[3]])
	if slices.Contains(fields, "resume") {
		return content, false
	}
	out := make([]string, 0, len(fields)+1)
	inserted := false
	for _, f := range fields {
		out = append(out, f)
		if f == "filesystems" {
			out = append(out, "resume")
			inserted = true
		}
	}
	if !inserted {
		out = append(out, "resume")
	}
	return content[:loc[2]] + strings.Join(out, " ") + content[loc[3]:], true
}
func networkManagerDNSDropin() string { return "[main]\ndns=systemd-resolved\n" }
