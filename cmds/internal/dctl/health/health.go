// Package health defines reusable healthcheck result types and rendering.
//
// Responsibilities:
// - Provide a stable JSON schema for checks.
// - Render equivalent plain output for humans.
// - Report aggregate failure state to command handlers.
package health

// health.go defines healthcheck statuses, result payloads, and printers.

import (
	"fmt"
	"strings"

	"dotfiles/cmds/internal/dctl/output"
)

type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
	Skip Status = "skip"
)

type Check struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Status   Status   `json:"status"`
	Expected string   `json:"expected,omitempty"`
	Observed string   `json:"observed,omitempty"`
	Fix      string   `json:"fix,omitempty"`
	Details  []string `json:"details,omitempty"`
}

func Print(out *output.Printer, checks []Check) error {
	if out.JSONMode() {
		return out.Emit(checks)
	}
	counts := map[Status]int{}
	for _, c := range checks {
		counts[c.Status]++
	}
	out.Info("health: %d ok, %d warn, %d fail, %d skip", counts[OK], counts[Warn], counts[Fail], counts[Skip])
	if counts[Warn]+counts[Fail]+counts[Skip] == 0 {
		out.OK("all checks passed")
		return nil
	}
	for _, c := range checks {
		if c.Status == OK {
			continue
		}
		name := c.Name
		if c.ID != "" && c.ID != c.Name {
			name = fmt.Sprintf("%s: %s", c.ID, c.Name)
		}
		switch c.Status {
		case OK:
			out.OK("%s", name)
		case Warn:
			out.Warn("%s", name)
		case Fail:
			out.Error("%s", name)
		case Skip:
			out.Info("skip: %s", name)
		}
		if c.Expected != "" {
			out.KV("expected", c.Expected)
		}
		if c.Observed != "" {
			out.KV("observed", c.Observed)
		}
		if c.Fix != "" {
			out.KV("fix", c.Fix)
		}
		if len(c.Details) > 0 {
			out.KV("details", strings.Join(c.Details, "; "))
		}
	}
	return nil
}

func HasFailure(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}
