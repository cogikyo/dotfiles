package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"dotfiles/cmds/internal/dctl/certs"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/home"
	"dotfiles/cmds/internal/dctl/keys"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/repos"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/secureboot"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/dctl/ssh"
	"dotfiles/cmds/internal/dctl/system"
	"dotfiles/cmds/internal/dctl/tailscale"
	"dotfiles/cmds/internal/dctl/vpn"
	"dotfiles/cmds/internal/ui"
)

type SetupCmd struct {
	Status bool       `help:"Show stage status and change nothing; exit non-zero for pending, failed, or blocked stages."`
	From   string     `placeholder:"STAGE" help:"Redo this stage and every later one; optional stages only when named."`
	Batch  setup.Mode `hidden:"" help:"Run the named stages directly in this mode; no plan, dependency pull-in, network check, or sudo. A named stage still waits when an earlier named stage it needs did not finish."`
	Report string     `hidden:""`
	Stages []string   `arg:"" optional:"" help:"Stages to redo, including unfinished dependencies (default: show the full plan)."`
}

func (c *SetupCmd) Help() string {
	var names []string
	for _, s := range catalog(nil, paths.Root{}) {
		var tags []string
		if s.Optional {
			tags = append(tags, "optional")
		}
		if s.Lock {
			tags = append(tags, "lock")
		}
		if s.Root {
			tags = append(tags, "root")
		}
		if len(tags) > 0 {
			names = append(names, s.Name+" ("+strings.Join(tags, ", ")+")")
			continue
		}
		names = append(names, s.Name)
	}
	return "Stages: " + strings.Join(names, ", ") + `

Lock stages need every non-optional stage OK plus their own confirmation; --yes never gives it.

Examples:
  dctl setup
  dctl setup --status
  dctl setup repos
  dctl setup --from firefox
  dctl setup tailscale`
}

// catalog lists stages in execution order; Needs must name earlier stages, and consecutive root stages share one sudo child.
func catalog(u *ui.UI, root paths.Root) []setup.Stage {
	run := execx.OSRunner{UI: u}
	stage := func(s setup.Stage, about string, needs ...string) setup.Stage {
		s.About, s.Needs = about, needs
		return s
	}
	optional := func(s setup.Stage, about string, needs ...string) setup.Stage {
		s = stage(s, about, needs...)
		s.Optional = true
		return s
	}
	lock := func(s setup.Stage, about string, needs ...string) setup.Stage {
		s = stage(s, about, needs...)
		s.Lock = true
		return s
	}
	apps := "apps, in the background"
	if l, err := packages.Load(root.Packages()); err == nil {
		apps = fmt.Sprintf("%d %s", len(l.Extra), apps)
	}
	trust := stage(certs.Stage(root, run), "local CA and certificates", "firefox")
	trust.Sudo = true
	stages := []setup.Stage{
		stage(system.Stage("/", root.System()), "overlay and services"),
		stage(setup.Network(u), "Ethernet and DNS"),
		stage(packages.Stage(root.Packages(), execx.OSRunner{UI: u, Interrupt: true}), "base, AUR, and local packages", "system", "network"),
		stage(home.Stage(root, run), "links and fonts", "packages"),
		stage(packages.Extra(root.Packages(), run), apps, "home", "network"),
		stage(secrets.Stage(u, root), "PIV PIN or recovery phrase", "system"),
		stage(ssh.Stage(root, run), "SSH access to GitHub", "secrets", "network"),
		stage(repos.Stage(root, run), "clone your repos", "ssh"),
		stage(home.Firefox(root, run), "profile and config", "packages"),
		trust,
		optional(tailscale.Stage(run), "login and SSH access", "network"),
		optional(vpn.Stage(u, root, run), "import the work VPN", "secrets"),
	}
	var phase []string
	for _, s := range stages {
		if !s.Optional {
			phase = append(phase, s.Name)
		}
	}
	return append(stages,
		lock(keys.Stage(u, run, "/"), "both YubiKeys plus a recovery key", phase...),
		lock(secureboot.Stage(run, "/"), "enroll keys and sign boot images", "luks"),
		lock(secureboot.TOTP(u, run, "/"), "seal the boot TOTP", "secureboot"),
	)
}

func (c *SetupCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	all := catalog(u, root)
	if c.Batch != "" {
		if !slices.Contains(setup.Modes, c.Batch) {
			return fmt.Errorf("unknown batch mode %q", c.Batch)
		}
		return c.direct(ctx, u, all, c.Batch)
	}
	if os.Geteuid() != 0 {
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			state = filepath.Join(root.Home, ".local", "state")
		}
		flow := setup.Flow{
			UI:     u,
			Stages: all,
			Home:   root.Home,
			Next:   filepath.Join(state, "dctl", "next"),
			Elevate: func(ctx context.Context, stages []setup.Stage, mode setup.Mode, quiet bool) ([]setup.Report, error) {
				return elevate(ctx, u, root, stages, mode, quiet)
			},
		}
		return flow.Run(ctx, setup.Request{Names: c.Stages, From: c.From, Status: c.Status})
	}
	if c.From != "" || len(c.Stages) == 0 {
		return errors.New("as root, name the root stages to run; run dctl setup as your user for the plan")
	}
	if c.Status {
		stages, err := setup.Select(all, c.Stages)
		if err != nil {
			return err
		}
		if user := slices.DeleteFunc(stages, func(s setup.Stage) bool { return s.Root }); len(user) > 0 {
			return fmt.Errorf("refusing to check user stages as root (%s): run dctl setup --status as your user", names(user))
		}
		return (&setup.Flow{UI: u, Stages: all}).Run(ctx, setup.Request{Names: c.Stages, Status: true})
	}
	return c.direct(ctx, u, all, setup.Force)
}

func (c *SetupCmd) direct(ctx context.Context, u *ui.UI, all []setup.Stage, mode setup.Mode) error {
	stages, err := setup.Select(all, c.Stages)
	if err != nil {
		return err
	}
	root := os.Geteuid() == 0
	if wrong := slices.DeleteFunc(slices.Clone(stages), func(s setup.Stage) bool { return s.Root == root }); len(wrong) > 0 {
		if root {
			return fmt.Errorf("refusing to run user stages as root (%s): run dctl setup as your user", names(wrong))
		}
		return fmt.Errorf("root stages need root (%s): run them with sudo, or run dctl setup without --batch", names(wrong))
	}
	reports, err := setup.Run(ctx, u, stages, mode)
	if c.Report != "" {
		err = errors.Join(err, record(c.Report, reports))
	}
	if u.JSON() {
		err = errors.Join(err, u.Emit(reports))
	}
	if err != nil || c.Report != "" || c.Batch == setup.Status {
		return err
	}
	return finish(u, reports, mode)
}

func finish(u *ui.UI, reports []setup.Report, mode setup.Mode) error {
	level, msg := setup.Outcome(reports)
	if setup.Incomplete(reports, mode) {
		return errors.New(msg)
	}
	u.Close(level, "%s", msg)
	return nil
}

func names(stages []setup.Stage) string {
	out := make([]string, len(stages))
	for i, s := range stages {
		out[i] = s.Name
	}
	return strings.Join(out, ", ")
}

func elevate(ctx context.Context, u *ui.UI, root paths.Root, stages []setup.Stage, mode setup.Mode, quiet bool) ([]setup.Report, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	validate := command(ctx, "sudo", "-v")
	validate.Stdout = os.Stderr
	if err := validate.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, setup.ErrRefused
	}
	flags := globals(u)
	if quiet || u.JSON() {
		if !u.JSON() {
			flags = append([]string{"--json"}, flags...)
		}
		var out bytes.Buffer
		cmd := child(ctx, batch(exe, root.Dotfiles, u.Env(), flags, mode, "", stages)...)
		cmd.Stdout = &out
		err := cmd.Run()
		var reports []setup.Report
		jerr := json.Unmarshal(out.Bytes(), &reports)
		switch {
		case canceled(err) && jerr == nil:
			return reports, ui.ErrCanceled
		case canceled(err):
			return nil, ui.ErrCanceled
		case jerr != nil:
			return nil, fmt.Errorf("root stages: %w", errors.Join(err, jerr))
		}
		return reports, nil
	}
	f, err := os.CreateTemp("", "dctl-setup-*.json")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)
	cmd := child(ctx, batch(exe, root.Dotfiles, u.Env(), flags, mode, path, stages)...)
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	data, rerr := os.ReadFile(path)
	var reports []setup.Report
	jerr := json.Unmarshal(data, &reports)
	switch {
	case canceled(err) && rerr == nil && jerr == nil:
		return reports, ui.ErrCanceled
	case canceled(err):
		return nil, ui.ErrCanceled
	case rerr != nil || jerr != nil:
		return setup.Unreached(stages, setup.Unknown, "root stages stopped without a report"), fmt.Errorf("root stages: %w", errors.Join(err, rerr, jerr))
	}
	return reports, nil
}

func canceled(err error) bool {
	exit, ok := errors.AsType[*exec.ExitError](err)
	return ok && exit.ExitCode() == 130
}

func record(path string, reports []setup.Report) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	return errors.Join(json.NewEncoder(f).Encode(reports), f.Close())
}

func command(ctx context.Context, argv ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	cmd.Stdin, cmd.Stderr = os.Stdin, os.Stderr
	return cmd
}

func child(ctx context.Context, argv ...string) *exec.Cmd {
	cmd := command(ctx, argv...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGINT) }
	cmd.WaitDelay = 0
	return cmd
}

func globals(u *ui.UI) []string {
	var flags []string
	if u.JSON() {
		flags = append(flags, "--json")
	}
	if u.Plain() {
		flags = append(flags, "--plain")
	}
	if u.Yes() {
		flags = append(flags, "--yes")
	}
	return flags
}

func batch(exe, dotfiles string, env, flags []string, mode setup.Mode, report string, stages []setup.Stage) []string {
	argv := slices.Concat([]string{"sudo", "env", "DOTFILES=" + dotfiles}, env, []string{exe}, flags, []string{"setup", "--batch=" + string(mode)})
	if report != "" {
		argv = append(argv, "--report="+report)
	}
	for _, s := range stages {
		argv = append(argv, s.Name)
	}
	return argv
}
