package hardware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

const (
	nothingToDo = 9
	maxAge      = 30 * 24 * time.Hour
)

type remote struct {
	Id    string
	Mtime int64
}

type fwupdError struct {
	Domain  string
	Code    int
	Message string
}

func Firmware(run execx.Runner) doctor.Group {
	ready := func() error {
		if _, err := exec.LookPath("fwupdmgr"); err != nil {
			return doctor.Block("fwupdmgr not found; install fwupd from base.lst")
		}
		return nil
	}
	return doctor.Group{Name: "fwupd", Online: true, Checks: []doctor.Check{
		{
			Name: "fwupd-metadata",
			Check: func(ctx context.Context) error {
				if err := ready(); err != nil {
					return err
				}
				out, err := run.Output(ctx, "", "fwupdmgr", "get-remotes", "--json")
				if err != nil {
					return fmt.Errorf("fwupdmgr get-remotes: %w", err)
				}
				return fresh([]byte(out), time.Now())
			},
			Fix: func(ctx context.Context) error {
				_, err := run.Run(ctx, "", "fwupdmgr", "refresh")
				return err
			},
		},
		{
			Name: "fwupd-updates",
			Check: func(ctx context.Context) error {
				if err := ready(); err != nil {
					return err
				}
				out, err := run.Output(ctx, "", "fwupdmgr", "get-updates", "--json")
				updates, perr := pending([]byte(out), err)
				if perr != nil {
					return perr
				}
				if len(updates) > 0 {
					return fmt.Errorf("firmware updates available: %s; flash manually with fwupdmgr update", strings.Join(updates, ", "))
				}
				return nil
			},
		},
	}}
}

func fresh(data []byte, now time.Time) error {
	var report struct {
		Remotes []remote
		Error   *fwupdError
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse fwupdmgr get-remotes: %w", err)
	}
	if report.Error != nil {
		return fmt.Errorf("fwupdmgr get-remotes: %s", report.Error.Message)
	}
	i := slices.IndexFunc(report.Remotes, func(r remote) bool { return r.Id == "lvfs" })
	if i < 0 {
		return errors.New("LVFS remote not configured")
	}
	mtime := report.Remotes[i].Mtime
	if mtime <= 0 {
		return errors.New("LVFS metadata was never downloaded")
	}
	if now.Sub(time.Unix(mtime, 0)) > maxAge {
		return fmt.Errorf("LVFS metadata is older than %s", maxAge)
	}
	return nil
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
