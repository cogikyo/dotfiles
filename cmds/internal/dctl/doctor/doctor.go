package doctor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/ui"
)

type Check struct {
	Name  string
	Check func(context.Context) error
	Fix   func(context.Context) error
}

type Group struct {
	Name   string
	Root   bool
	Checks []Check
}

type Status string

const (
	Passed  Status = "ok"
	Fixed   Status = "fixed"
	Failed  Status = "failed"
	Blocked Status = "blocked"
)

type Result struct {
	Group  string `json:"group"`
	Check  string `json:"check"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func (r Result) Healthy() bool { return r.Status == Passed || r.Status == Fixed }

type Options struct {
	Fix      bool
	Elevated bool
	Rerun    string
}

func Select(all []Group, names []string) ([]Group, error) {
	seen := map[string]bool{}
	for _, g := range all {
		if seen[g.Name] {
			return nil, fmt.Errorf("duplicate doctor group %q", g.Name)
		}
		seen[g.Name] = true
		for _, c := range g.Checks {
			if seen[c.Name] {
				return nil, fmt.Errorf("duplicate doctor check %q", c.Name)
			}
			seen[c.Name] = true
		}
	}
	if len(names) == 0 {
		return all, nil
	}
	for _, name := range names {
		if !slices.ContainsFunc(all, func(g Group) bool { return g.Name == name }) {
			known := make([]string, 0, len(all))
			for _, g := range all {
				known = append(known, g.Name)
			}
			return nil, fmt.Errorf("unknown doctor group %q (known: %s)", name, strings.Join(known, ", "))
		}
	}
	return slices.DeleteFunc(slices.Clone(all), func(g Group) bool { return !slices.Contains(names, g.Name) }), nil
}

type blocked struct{ reason string }

func (b blocked) Error() string { return b.reason }

func Block(format string, args ...any) error {
	return blocked{fmt.Sprintf(format, args...)}
}

func Run(ctx context.Context, u *ui.UI, groups []Group, opts Options) ([]Result, error) {
	var results []Result
	for _, g := range groups {
		u.Step("%s", g.Name)
		for _, c := range g.Checks {
			if err := ctx.Err(); err != nil {
				return results, err
			}
			r := evaluate(ctx, g, c, opts)
			if err := ctx.Err(); err != nil {
				return results, err
			}
			show(u, r)
			results = append(results, r)
		}
	}
	return results, nil
}

func evaluate(ctx context.Context, g Group, c Check, opts Options) Result {
	r := Result{Group: g.Name, Check: c.Name, Status: Passed}
	err := c.Check(ctx)
	if err == nil {
		return r
	}
	r.Status, r.Detail = judge(err), err.Error()
	if r.Status == Blocked || !opts.Fix || c.Fix == nil {
		return r
	}
	switch {
	case g.Root && !opts.Elevated:
		r.Status, r.Detail = Blocked, fmt.Sprintf("%s; needs root: sudo %s %s", r.Detail, opts.Rerun, g.Name)
		return r
	case !g.Root && opts.Elevated:
		r.Status, r.Detail = Blocked, fmt.Sprintf("%s; refusing to fix as root: run %s %s as the owning user", r.Detail, opts.Rerun, g.Name)
		return r
	}
	if err := c.Fix(ctx); err != nil {
		r.Status, r.Detail = judge(err), "fix: "+err.Error()
		return r
	}
	if err := c.Check(ctx); err != nil {
		r.Status, r.Detail = judge(err), "recheck: "+err.Error()
		return r
	}
	r.Status, r.Detail = Fixed, ""
	return r
}

func judge(err error) Status {
	if _, ok := errors.AsType[blocked](err); ok {
		return Blocked
	}
	return Failed
}

func show(u *ui.UI, r Result) {
	switch r.Status {
	case Passed:
		u.Row(ui.OK, r.Check)
	case Fixed:
		u.Row(ui.OK, r.Check+" (fixed)")
	case Blocked:
		u.Row(ui.Warn, r.Check+" (blocked)")
		u.Dim("%s", r.Detail)
	default:
		u.Row(ui.Err, r.Check)
		u.Dim("%s", r.Detail)
	}
}
