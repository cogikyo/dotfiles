package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	"dotfiles/cmds/internal/dctl/system"
	"dotfiles/cmds/internal/dctl/tailscale"
	"dotfiles/cmds/internal/dctl/vpn"
	"dotfiles/cmds/internal/ui"
)

type SetupCmd struct {
	All    bool       `xor:"mode" help:"Apply every pending stage without asking; same as the global --yes."`
	Status bool       `xor:"mode" help:"Report each stage and change nothing; exit non-zero when one is pending or failed."`
	Batch  setup.Mode `hidden:""`
	Report string     `hidden:""`
	Stages []string   `arg:"" optional:"" help:"Stages to redo, including items that look done (default: plan pending stages and ask once)."`
}

func (c *SetupCmd) Help() string {
	var names []string
	for _, s := range catalog(nil, paths.Root{}) {
		if s.Root {
			names = append(names, s.Name+" (root)")
			continue
		}
		names = append(names, s.Name)
	}
	return "Stages: " + strings.Join(names, ", ") + `

Examples:
  dctl setup
  dctl setup --status
  dctl setup home firefox`
}

// catalog lists stages in execution order; consecutive root stages share one sudo child.
func catalog(u *ui.UI, root paths.Root) []setup.Stage {
	run := execx.OSRunner{Frame: u.Frame}
	clone := repos.Stage(root, run)
	clone.Online = true
	trust := certs.Stage(root, run)
	trust.Sudo = true
	return []setup.Stage{
		system.Stage("/", root.System()),
		tailscale.Stage(run),
		packages.Stage(root.Packages(), execx.OSRunner{Frame: u.Frame, Interrupt: true}),
		home.Stage(root, run),
		packages.Extra(root.Packages(), run),
		secrets.Stage(u, root),
		clone,
		home.Firefox(root),
		trust,
		vpn.Stage(u, root, run),
		keys.Stage(u, run, "/sys"),
		secureboot.Stage(run, "/"),
	}
}

func (c *SetupCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	selected, err := setup.Select(catalog(u, root), c.Stages)
	if err != nil {
		return err
	}
	mode, err := c.mode(u)
	if err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		return c.elevated(ctx, u, selected, mode)
	}
	return c.user(ctx, u, root, selected, mode)
}

func (c *SetupCmd) elevated(ctx context.Context, u *ui.UI, stages []setup.Stage, mode setup.Mode) error {
	if user := slices.DeleteFunc(slices.Clone(stages), func(s setup.Stage) bool { return s.Root }); len(user) > 0 {
		return fmt.Errorf("refusing to run user stages as root (%s): run dctl setup as your user", names(user))
	}
	reports, err := setup.Run(ctx, u, stages, mode)
	if c.Report != "" {
		err = errors.Join(err, record(c.Report, reports))
	}
	if u.JSON() {
		err = errors.Join(err, u.Emit(reports))
	}
	if err != nil || c.Batch != "" {
		return err
	}
	return finish(u, reports, mode)
}

func (c *SetupCmd) user(ctx context.Context, u *ui.UI, root paths.Root, stages []setup.Stage, mode setup.Mode) error {
	if mode == setup.Force {
		return c.apply(ctx, u, root, stages, nil, mode)
	}
	plan, err := evaluate(ctx, u, root, stages, mode)
	if err != nil {
		return err
	}
	for _, r := range plan {
		setup.Show(u, r)
	}
	if mode == setup.Status {
		if u.JSON() {
			if err := u.Emit(plan); err != nil {
				return err
			}
		}
		return finish(u, plan, mode)
	}
	var todo []setup.Stage
	pending := 0
	for i, s := range stages {
		switch plan[i].State {
		case setup.Pending:
			pending++
			todo = append(todo, s)
		case setup.ManualState:
			if !s.Root {
				todo = append(todo, s)
			}
		}
	}
	if pending == 0 {
		return c.done(u, plan, mode)
	}
	if mode == setup.Ask {
		ok, err := u.Proceed(fmt.Sprintf("Apply %d pending stages?", pending))
		if err != nil {
			return err
		}
		if !ok {
			u.Close(ui.Info, "nothing applied")
			return nil
		}
	}
	return c.apply(ctx, u, root, todo, plan, mode)
}

func (c *SetupCmd) apply(ctx context.Context, u *ui.UI, root paths.Root, todo []setup.Stage, plan []setup.Report, mode setup.Mode) error {
	online, err := networked(ctx, u, root, todo, plan)
	if err != nil {
		return err
	}
	batch := setup.All
	if mode == setup.Force {
		batch = mode
	}
	results := slices.Clone(plan)
	merge := func(rs ...setup.Report) {
		for _, r := range rs {
			if i := slices.IndexFunc(results, func(p setup.Report) bool { return p.Stage == r.Stage }); i >= 0 {
				results[i] = r
				continue
			}
			results = append(results, r)
		}
	}
	waitCtx, stopWaits := context.WithCancel(ctx)
	defer stopWaits()
	var waits []func(context.Context) (setup.Report, error)
	collect := func() error {
		var errs []error
		for _, wait := range waits {
			r, err := wait(waitCtx)
			merge(r)
			errs = append(errs, err)
		}
		waits = nil
		return errors.Join(errs...)
	}
	var errs []error
	var offline string
	checked := len(online) == 0
	stopped := false
	for i := 0; i < len(todo) && !stopped && ctx.Err() == nil; {
		s := todo[i]
		if !checked && online[s.Name] {
			checked = true
			if err := setup.Online(ctx); err != nil {
				if ctx.Err() != nil {
					break
				}
				offline = err.Error()
				u.Node(ui.Err, fmt.Sprintf("%-10s", "network"), offline)
				rest := todo[i:]
				todo = todo[:i:i]
				for _, t := range rest {
					if online[t.Name] {
						merge(setup.Unreached([]setup.Stage{t}, setup.Failed, "needs network: "+offline)...)
						continue
					}
					todo = append(todo, t)
				}
				continue
			}
		}
		switch {
		case s.Root:
			if err := collect(); err != nil {
				errs = append(errs, err)
			}
			j := i + 1
			for j < len(todo) && todo[j].Root && (checked || !online[todo[j].Name]) {
				j++
			}
			rs, err := elevate(ctx, u, root, todo[i:j], batch, false)
			if errors.Is(err, setup.ErrRefused) {
				rs, err = refused(todo[i:j], batch), nil
				for _, r := range rs {
					setup.Show(u, r)
				}
			}
			merge(rs...)
			i = j
			if err != nil {
				errs = append(errs, err)
			}
		case s.Background:
			i++
			if batch != setup.Force {
				if r := s.Evaluate(ctx); r.State != setup.Pending {
					merge(r)
					continue
				}
			}
			wait, err := setup.Start(ctx, u, s, batch == setup.Force, root.Home)
			switch {
			case errors.Is(err, context.Canceled):
				errs = append(errs, err)
			case err != nil:
				r := setup.Unreached([]setup.Stage{s}, setup.Failed, err.Error())[0]
				setup.Show(u, r)
				merge(r)
			default:
				waits = append(waits, wait)
			}
		default:
			i++
			if s.Sudo {
				if err := collect(); err != nil {
					errs = append(errs, err)
				}
			}
			rs, err := setup.Run(ctx, u, []setup.Stage{s}, batch)
			merge(rs...)
			if err != nil {
				errs = append(errs, err)
			}
		}
		stopped = slices.ContainsFunc(errs, func(err error) bool { return errors.Is(err, ui.ErrCanceled) })
		if stopped {
			stopWaits()
		}
	}
	errs = append(errs, collect())
	if err := errors.Join(errs...); ctx.Err() != nil || errors.Is(err, ui.ErrCanceled) || errors.Is(err, context.Canceled) {
		return ui.ErrCanceled
	}
	if u.JSON() {
		errs = append(errs, u.Emit(results))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	err = finish(u, results, mode)
	if err != nil && offline != "" {
		err = fmt.Errorf("%w · %s", err, offline)
	}
	return err
}

func networked(ctx context.Context, u *ui.UI, root paths.Root, todo []setup.Stage, plan []setup.Report) (map[string]bool, error) {
	stages := slices.DeleteFunc(slices.Clone(todo), func(s setup.Stage) bool { return !s.Online })
	if plan == nil && len(stages) > 0 {
		var err error
		if plan, err = evaluate(ctx, u, root, stages, setup.Force); err != nil {
			return nil, err
		}
	}
	online := map[string]bool{}
	for _, s := range stages {
		if st := state(plan, s.Name); st == setup.Pending || st == setup.Unknown {
			online[s.Name] = true
		}
	}
	return online, nil
}

func (c *SetupCmd) done(u *ui.UI, reports []setup.Report, mode setup.Mode) error {
	if u.JSON() {
		if err := u.Emit(reports); err != nil {
			return err
		}
	}
	return finish(u, reports, mode)
}

func finish(u *ui.UI, reports []setup.Report, mode setup.Mode) error {
	level, msg := setup.Outcome(reports, mode)
	if setup.Incomplete(reports, mode) {
		return errors.New(msg)
	}
	u.Close(level, "%s", msg)
	return nil
}

func state(reports []setup.Report, name string) setup.State {
	if i := slices.IndexFunc(reports, func(r setup.Report) bool { return r.Stage == name }); i >= 0 {
		return reports[i].State
	}
	return ""
}

func evaluate(ctx context.Context, u *ui.UI, root paths.Root, stages []setup.Stage, mode setup.Mode) ([]setup.Report, error) {
	elevated := slices.DeleteFunc(slices.Clone(stages), func(s setup.Stage) bool { return !s.Root })
	var fromRoot []setup.Report
	if len(elevated) > 0 {
		rs, err := elevate(ctx, u, root, elevated, setup.Status, true)
		if errors.Is(err, ui.ErrCanceled) || ctx.Err() != nil {
			return nil, errors.Join(err, ctx.Err())
		}
		switch {
		case errors.Is(err, setup.ErrRefused):
			rs = refused(elevated, mode)
		case err != nil:
			rs = setup.Unreached(elevated, setup.Unknown, err.Error())
		}
		fromRoot = rs
	}
	out := make([]setup.Report, 0, len(stages))
	for _, s := range stages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !s.Root {
			out = append(out, s.Evaluate(ctx))
			continue
		}
		if i := slices.IndexFunc(fromRoot, func(r setup.Report) bool { return r.Stage == s.Name }); i >= 0 {
			out = append(out, fromRoot[i])
			continue
		}
		out = append(out, setup.Unreached([]setup.Stage{s}, setup.Unknown, "no report from the root check")[0])
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func refused(stages []setup.Stage, mode setup.Mode) []setup.Report {
	if mode == setup.Status {
		return setup.Unreached(stages, setup.Unknown, "sudo refused")
	}
	return setup.Unreached(stages, setup.Failed, setup.ErrRefused.Error())
}

func names(stages []setup.Stage) string {
	out := make([]string, len(stages))
	for i, s := range stages {
		out[i] = s.Name
	}
	return strings.Join(out, ", ")
}

func (c *SetupCmd) mode(u *ui.UI) (setup.Mode, error) {
	switch {
	case c.Batch != "":
		if !slices.Contains(setup.Modes, c.Batch) {
			return "", fmt.Errorf("unknown batch mode %q", c.Batch)
		}
		return c.Batch, nil
	case c.Status:
		return setup.Status, nil
	case len(c.Stages) > 0:
		return setup.Force, nil
	case c.All || u.Yes():
		return setup.All, nil
	}
	return setup.Ask, nil
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
		if canceled(err) {
			return nil, ui.ErrCanceled
		}
		var reports []setup.Report
		if jerr := json.Unmarshal(out.Bytes(), &reports); jerr != nil {
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
	if canceled(err) {
		return nil, ui.ErrCanceled
	}
	data, rerr := os.ReadFile(path)
	var reports []setup.Report
	if jerr := json.Unmarshal(data, &reports); rerr != nil || jerr != nil {
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
