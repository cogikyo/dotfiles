package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"
)

const releaseWait = 30 * time.Second

type row struct {
	level ui.Level
	msg   string
}

type release struct {
	Pkgbase   string `json:"pkgbase"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Local     string `json:"local_version"`
	Upstream  string `json:"upstream_version"`
	OutOfDate bool   `json:"out_of_date"`
}

func releases(ctx context.Context, run execx.Runner, dirs []string) []row {
	if len(dirs) == 0 {
		return nil
	}
	for _, tool := range []string{"pkgctl", "nvchecker"} {
		if _, err := exec.LookPath(tool); err != nil {
			return []row{{ui.Info, "release check skipped: " + tool + " is not installed"}}
		}
	}
	if r, ok := run.(execx.OSRunner); ok {
		r.Group = true
		run = r
	}
	ctx, cancel := context.WithTimeout(ctx, releaseWait)
	defer cancel()
	out, err := run.Output(ctx, "", "env", slices.Concat([]string{"GIT_TERMINAL_PROMPT=0", "pkgctl", "version", "check", "--json", "-v"}, dirs)...)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return []row{{ui.Info, fmt.Sprintf("release check timed out after %s", releaseWait)}}
	}
	if err != nil {
		return []row{{ui.Info, "release check failed: " + err.Error()}}
	}
	var checks []release
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		return []row{{ui.Info, "release check output unreadable: " + err.Error()}}
	}
	var rows []row
	var current []string
	for _, c := range checks {
		switch {
		case c.OutOfDate:
			rows = append(rows, row{ui.Warn, fmt.Sprintf("%s %s released upstream; recipe pins %s", c.Pkgbase, c.Upstream, c.Local)})
		case c.Status == "failure":
			msg, _, _ := strings.Cut(c.Message, "\n")
			rows = append(rows, row{ui.Info, fmt.Sprintf("%s release check failed: %s", c.Pkgbase, msg)})
		default:
			current = append(current, c.Pkgbase)
		}
	}
	if len(current) > 0 {
		rows = append(rows, row{ui.OK, "no newer upstream release: " + strings.Join(current, ", ")})
	}
	return rows
}
