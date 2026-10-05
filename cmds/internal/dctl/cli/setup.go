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
	Stages []string   `arg:"" optional:"" help:"Stages to redo, including items that look done (default: ask for each pending stage)."`
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

// catalog orders stages within each privilege batch; root stages must not depend on user stages.
func catalog(u *ui.UI, root paths.Root) []setup.Stage {
	run := execx.OSRunner{Frame: u.Frame}
	return []setup.Stage{
		system.Stage("/", root.System()),
		packages.Stage(root.Packages(), run),
		home.Stage(root, run),
		packages.Extra(root.Packages(), run),
		secrets.Stage(u, root),
		repos.Stage(root, run),
		home.Firefox(root),
		certs.Stage(root, run),
		vpn.Stage(u, root, run),
		tailscale.Stage(run),
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
	var elevated, user []setup.Stage
	for _, s := range selected {
		if s.Root {
			elevated = append(elevated, s)
		} else {
			user = append(user, s)
		}
	}
	asRoot := os.Geteuid() == 0
	if asRoot && len(user) > 0 {
		return fmt.Errorf("refusing to run user stages as root (%s): run dctl setup as your user", names(user))
	}
	reports := []setup.Report{}
	var batchErr error
	switch {
	case len(elevated) == 0:
	case asRoot:
		rs, err := setup.Run(ctx, u, elevated, mode)
		reports = append(reports, rs...)
		if err != nil {
			return err
		}
	default:
		rs, err := elevate(ctx, u, root, elevated, mode)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, ui.ErrCanceled) {
			return err
		}
		reports, batchErr = append(reports, rs...), err
	}
	rs, err := setup.Run(ctx, u, user, mode)
	reports = append(reports, rs...)
	if err != nil {
		return err
	}

	if u.JSON() {
		if err := u.Emit(reports); err != nil {
			return err
		}
	}
	var errs []error
	if batchErr != nil {
		errs = append(errs, batchErr)
	}
	if setup.Incomplete(reports, mode) {
		errs = append(errs, errors.New("setup: items pending or failed; see above"))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	u.Close(ui.OK, "setup done")
	return nil
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

func elevate(ctx context.Context, u *ui.UI, root paths.Root, stages []setup.Stage, mode setup.Mode) ([]setup.Report, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	validate := command(ctx, "sudo", "-v")
	validate.Stdout = os.Stderr
	if err := validate.Run(); err != nil {
		return setup.Unreached(u, stages, "sudo refused: "+err.Error()), nil
	}
	cmd := command(ctx, batch(exe, root.Dotfiles, u.Env(), globals(u), mode, stages)...)
	if !u.JSON() {
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			if exit, ok := errors.AsType[*exec.ExitError](err); ok {
				if exit.ExitCode() == 130 {
					return nil, ui.ErrCanceled
				}
				return nil, errors.New("root stages incomplete; see above")
			}
			return nil, fmt.Errorf("root stages: %w", err)
		}
		return nil, nil
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	var reports []setup.Report
	if jerr := json.Unmarshal(out.Bytes(), &reports); jerr != nil {
		return nil, fmt.Errorf("root stages: %w", errors.Join(err, jerr))
	}
	return reports, nil
}

func command(ctx context.Context, argv ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 10 * time.Second
	cmd.Stdin, cmd.Stderr = os.Stdin, os.Stderr
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

func batch(exe, dotfiles string, env, flags []string, mode setup.Mode, stages []setup.Stage) []string {
	argv := slices.Concat([]string{"sudo", "env", "DOTFILES=" + dotfiles}, env, []string{exe}, flags, []string{"setup", "--batch=" + string(mode)})
	for _, s := range stages {
		argv = append(argv, s.Name)
	}
	return argv
}
