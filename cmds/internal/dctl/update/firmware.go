package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/ui"
)

const (
	nothingToDo = 9
	current     = 2
)

type fwupdError struct {
	Domain  string
	Code    int
	Message string
}

func firmware(ctx context.Context, u *ui.UI, run execx.Runner, all bool) error {
	if _, err := exec.LookPath("fwupdmgr"); err != nil {
		return errors.New("fwupdmgr not found; install fwupd from base.lst")
	}
	if err := run.Run(ctx, "", "fwupdmgr", "refresh", "--no-remote-check"); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); !ok || exit.ExitCode() != current {
			return err
		}
	}
	out, err := run.Output(ctx, "", "fwupdmgr", "get-remotes", "--json")
	if err != nil {
		return err
	}
	ok, err := download(out)
	if err != nil {
		return err
	}
	if !ok {
		u.Warn("firmware skipped: no download remote is enabled, so no metadata; run fwupdmgr enable-remote lvfs")
		return nil
	}
	out, err = run.Output(ctx, "", "fwupdmgr", "get-updates", "--json")
	updates, err := pending([]byte(out), err)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		u.OK("firmware is current")
		return nil
	}
	u.Warn("firmware updates: %s", strings.Join(updates, ", "))
	if all {
		u.Hint("--all never flashes firmware; run dctl update firmware to apply")
		return nil
	}
	ok, err = u.Confirm("Flash these firmware updates?")
	if err != nil || !ok {
		return err
	}
	return run.Run(ctx, "", "fwupdmgr", "update")
}

func download(remotes string) (bool, error) {
	type remote struct {
		Kind    string
		Enabled any
	}
	var report struct{ Remotes []remote }
	if err := json.Unmarshal([]byte(remotes), &report); err != nil {
		return false, fmt.Errorf("parse fwupdmgr get-remotes: %w", err)
	}
	return slices.ContainsFunc(report.Remotes, func(r remote) bool {
		return r.Kind == "download" && fmt.Sprint(r.Enabled) == "true"
	}), nil
}

func pending(data []byte, runErr error) ([]string, error) {
	var report struct {
		Devices []struct {
			Name     string
			Releases []struct{ Version string }
		}
		Error *fwupdError
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse fwupdmgr get-updates: %w", errors.Join(err, runErr))
	}
	if e := report.Error; e != nil {
		if e.Code == nothingToDo && strings.Contains(strings.ToLower(e.Domain), "fwupd") {
			return nil, nil
		}
		return nil, fmt.Errorf("fwupdmgr get-updates: %s (%s %d)", e.Message, e.Domain, e.Code)
	}
	if runErr != nil {
		return nil, fmt.Errorf("fwupdmgr get-updates: %w", runErr)
	}
	var out []string
	for _, d := range report.Devices {
		if len(d.Releases) > 0 {
			out = append(out, d.Name+" "+d.Releases[0].Version)
		}
	}
	return out, nil
}
