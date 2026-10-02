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
	Sudo   bool
	Online bool
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

func Select(all []Group, names []string, offline bool) ([]Group, error) {
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
		return slices.DeleteFunc(slices.Clone(all), func(g Group) bool { return offline && g.Online }), nil
	}
	for _, name := range names {
		i := slices.IndexFunc(all, func(g Group) bool { return g.Name == name })
		if i < 0 {
			known := make([]string, 0, len(all))
			for _, g := range all {
				known = append(known, g.Name)
			}
			return nil, fmt.Errorf("unknown doctor group %q (known: %s)", name, strings.Join(known, ", "))
		}
		if offline && all[i].Online {
			return nil, fmt.Errorf("doctor group %q needs the network; drop --offline", name)
		}
	}
	return slices.DeleteFunc(slices.Clone(all), func(g Group) bool { return !slices.Contains(names, g.Name) }), nil
}

type blocked struct{ reason string }

func (b blocked) Error() string { return b.reason }

func Block(format string, args ...any) error {
	return blocked{fmt.Sprintf(format, args...)}
}

func List(head string, items []string) string {
	return head + ":\n  " + strings.Join(items, "\n  ")
}

func Run(ctx context.Context, u *ui.UI, groups []Group, opts Options) ([]Result, error) {
	var results []Result
	for _, g := range groups {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		shown := false
		head := func() {
			if !shown {
				u.Step("%s", g.Name)
				shown = true
			}
		}
		start := len(results)
		for _, c := range g.Checks {
			r := evaluate(ctx, g, c, opts, head)
			if ctx.Err() != nil {
				break
			}
			results = append(results, r)
		}
		show(u, g, results[start:], opts, shown)
		if err := ctx.Err(); err != nil {
			return results, err
		}
	}
	return results, nil
}

func evaluate(ctx context.Context, g Group, c Check, opts Options, head func()) Result {
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
	case g.Sudo && !opts.Elevated:
		r.Status, r.Detail = Blocked, fmt.Sprintf("%s\nneeds root: sudo %s %s", r.Detail, opts.Rerun, g.Name)
		return r
	case !g.Sudo && opts.Elevated:
		r.Status, r.Detail = Blocked, fmt.Sprintf("%s\nrefusing to fix as root: run %s %s as the owning user", r.Detail, opts.Rerun, g.Name)
		return r
	}
	head()
	if err := c.Fix(ctx); err != nil {
		r.Status, r.Detail = judge(err), "fix: "+err.Error()
		return r
	}
	if err := c.Check(ctx); err != nil {
		r.Status, r.Detail = judge(err), "recheck: "+err.Error()
		return r
	}
	r.Status = Fixed
	return r
}

func judge(err error) Status {
	if _, ok := errors.AsType[blocked](err); ok {
		return Blocked
	}
	return Failed
}

func show(u *ui.UI, g Group, results []Result, opts Options, shown bool) {
	passed, fixable := 0, false
	for i, r := range results {
		switch {
		case r.Status == Passed:
			passed++
		case r.Status == Failed && g.Checks[i].Fix != nil:
			fixable = true
		}
	}
	if !shown {
		label := g.Name
		if passed > 0 {
			label = fmt.Sprintf("%s  %d passed", g.Name, passed)
		}
		u.Step("%s", label)
	}
	for _, r := range results {
		name := strings.TrimPrefix(r.Check, g.Name+"-")
		switch r.Status {
		case Fixed:
			u.Row(ui.OK, name+" (fixed)")
			u.Detail("was: " + r.Detail)
		case Blocked:
			u.Row(ui.Warn, name+" (blocked)")
			u.Detail(r.Detail)
		case Failed:
			u.Row(ui.Err, name)
			u.Detail(r.Detail)
		}
	}
	if shown && passed > 0 {
		u.Dim("%d passed", passed)
	}
	if !fixable || opts.Fix {
		return
	}
	sudo := ""
	if g.Sudo {
		sudo = "sudo "
	}
	u.Hint("Fix: %s%s %s", sudo, opts.Rerun, g.Name)
}

func Summary(results []Result) string {
	counts := map[Status]int{}
	for _, r := range results {
		counts[r.Status]++
	}
	var parts []string
	for _, s := range []struct {
		status Status
		label  string
	}{{Failed, "failed"}, {Blocked, "blocked"}, {Fixed, "fixed"}, {Passed, "passed"}} {
		if n := counts[s.status]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, s.label))
		}
	}
	return strings.Join(parts, ", ")
}
