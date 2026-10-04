package setup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"dotfiles/cmds/internal/ui"
)

// Item checks without changing state; Fix must be safe to repeat even when Check succeeds.
type Item struct {
	Name  string
	Check func(context.Context) error
	Fix   func(context.Context) error
}

type Stage struct {
	Name  string
	Root  bool
	Items []Item
}

type State string

const (
	Done        State = "done"
	Pending     State = "pending"
	ManualState State = "manual"
	Unknown     State = "unknown"
	Failed      State = "failed"
)

type Mode string

const (
	Ask    Mode = "ask"
	All    Mode = "all"
	Force  Mode = "force"
	Status Mode = "status"
)

var Modes = []Mode{Ask, All, Force, Status}

type Result struct {
	Item   string `json:"item"`
	State  State  `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type Report struct {
	Stage string   `json:"stage"`
	State State    `json:"state"`
	Items []Result `json:"items"`
}

type manual struct{ action string }

func (m manual) Error() string { return m.action }

// Manual reports required outside action without making the item a command failure.
func Manual(format string, args ...any) error {
	return manual{fmt.Sprintf(format, args...)}
}

func List(head string, items []string) string {
	return head + ":\n  " + strings.Join(items, "\n  ")
}

func Select(all []Stage, names []string) ([]Stage, error) {
	for _, name := range names {
		if !slices.ContainsFunc(all, func(s Stage) bool { return s.Name == name }) {
			known := make([]string, len(all))
			for i, s := range all {
				known[i] = s.Name
			}
			return nil, fmt.Errorf("unknown stage %q (known: %s)", name, strings.Join(known, ", "))
		}
	}
	if len(names) == 0 {
		return slices.Clone(all), nil
	}
	return slices.DeleteFunc(slices.Clone(all), func(s Stage) bool { return !slices.Contains(names, s.Name) }), nil
}

func Run(ctx context.Context, u *ui.UI, stages []Stage, mode Mode) ([]Report, error) {
	reports := make([]Report, 0, len(stages))
	if mode == Force {
		for _, s := range stages {
			if err := ctx.Err(); err != nil {
				return reports, err
			}
			u.Header("%s", s.Name)
			r := s.apply(ctx, true)
			show(u, r)
			reports = append(reports, r)
		}
		return reports, ctx.Err()
	}
	for _, s := range stages {
		if err := ctx.Err(); err != nil {
			return reports, err
		}
		r := s.evaluate(ctx)
		if mode == Status || r.State != Pending {
			list(u, r)
		} else {
			u.Node(level(r.State), headline(r))
		}
		reports = append(reports, r)
	}
	if mode == Status {
		return reports, ctx.Err()
	}
	applied := false
	for i, s := range stages {
		if err := ctx.Err(); err != nil {
			return reports, err
		}
		if applied {
			// An earlier stage can remove a manual prerequisite, such as repos before Firefox.
			if reports[i] = s.evaluate(ctx); reports[i].State == Pending {
				u.Node(level(Pending), headline(reports[i]))
			}
		}
		if reports[i].State != Pending {
			continue
		}
		u.Header("%s", s.Name)
		if mode == Ask {
			ok, err := u.Proceed(fmt.Sprintf("Apply %s?", s.Name))
			if err != nil {
				return reports, err
			}
			if !ok {
				continue
			}
		}
		reports[i] = s.apply(ctx, false)
		applied = true
		show(u, reports[i])
	}
	return reports, ctx.Err()
}

func Unreached(u *ui.UI, stages []Stage, detail string) []Report {
	reports := make([]Report, 0, len(stages))
	for _, s := range stages {
		r := Report{Stage: s.Name, State: Unknown}
		for _, it := range s.Items {
			r.Items = append(r.Items, Result{Item: it.Name, State: Unknown, Detail: detail})
		}
		u.Node(level(Unknown), headline(r))
		reports = append(reports, r)
	}
	if len(reports) > 0 {
		u.Detail(detail)
	}
	return reports
}

func (s Stage) evaluate(ctx context.Context) Report {
	r := Report{Stage: s.Name}
	for _, it := range s.Items {
		r.Items = append(r.Items, it.evaluate(ctx))
	}
	r.State = worst(r.Items)
	return r
}

func (s Stage) apply(ctx context.Context, force bool) Report {
	r := Report{Stage: s.Name}
	for _, it := range s.Items {
		if ctx.Err() != nil {
			break
		}
		r.Items = append(r.Items, it.apply(ctx, force))
	}
	r.State = worst(r.Items)
	return r
}

func (it Item) evaluate(ctx context.Context) Result {
	err := it.Check(ctx)
	r := Result{Item: it.Name, State: Done}
	switch {
	case err == nil:
		return r
	case isManual(err):
		r.State = ManualState
	case it.Fix != nil:
		r.State = Pending
	default:
		r.State = Failed
	}
	r.Detail = err.Error()
	return r
}

func (it Item) apply(ctx context.Context, force bool) Result {
	if !force {
		if r := it.evaluate(ctx); r.State != Pending {
			return r
		}
	}
	if it.Fix != nil {
		if err := it.Fix(ctx); err != nil {
			r := Result{Item: it.Name, State: Failed, Detail: "fix: " + err.Error()}
			if isManual(err) {
				r.State = ManualState
			}
			return r
		}
	}
	r := it.evaluate(ctx)
	if r.State == Pending {
		r.State = Failed
	}
	return r
}

func isManual(err error) bool {
	_, ok := errors.AsType[manual](err)
	return ok
}

var severity = []State{Pending, Failed, Unknown, ManualState}

func worst(items []Result) State {
	for _, s := range severity {
		if slices.ContainsFunc(items, func(r Result) bool { return r.State == s }) {
			return s
		}
	}
	return Done
}

func level(s State) ui.Level {
	switch s {
	case Done:
		return ui.OK
	case ManualState:
		return ui.Info
	case Failed:
		return ui.Err
	}
	return ui.Warn
}

func short(r Report, item string) string {
	return strings.TrimPrefix(item, r.Stage+"-")
}

func headline(r Report) string {
	line := fmt.Sprintf("%-10s %s", r.Stage, r.State)
	var open []string
	for _, it := range r.Items {
		if it.State != Done {
			open = append(open, short(r, it.Item))
		}
	}
	if len(open) > 0 {
		line += ": " + strings.Join(open, ", ")
	}
	return line
}

func list(u *ui.UI, r Report) {
	u.Node(level(r.State), headline(r))
	details(u, r)
}

func show(u *ui.UI, r Report) {
	u.Row(level(r.State), headline(r))
	details(u, r)
}

func details(u *ui.UI, r Report) {
	for _, it := range r.Items {
		if it.State != Done {
			u.Detail(fmt.Sprintf("%s (%s): %s", short(r, it.Item), it.State, it.Detail))
		}
	}
}

// Incomplete treats pending items as failures only in Status mode; manual and unknown items never fail it.
func Incomplete(reports []Report, mode Mode) bool {
	return slices.ContainsFunc(reports, func(r Report) bool {
		return slices.ContainsFunc(r.Items, func(it Result) bool {
			return it.State == Failed || mode == Status && it.State == Pending
		})
	})
}
