package hardware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
)

const nothingToDo = 9

type fwupdError struct {
	Domain  string
	Code    int
	Message string
}

func ready() error {
	if _, err := exec.LookPath("fwupdmgr"); err != nil {
		return errors.New("fwupdmgr not found; install fwupd from base.lst")
	}
	return nil
}

func Updates(ctx context.Context, run execx.Runner) ([]string, error) {
	if err := ready(); err != nil {
		return nil, err
	}
	out, err := run.Output(ctx, "", "fwupdmgr", "get-updates", "--json")
	return pending([]byte(out), err)
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
