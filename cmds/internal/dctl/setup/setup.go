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
	Name       string
	Root       bool
	Online     bool
	Background bool
	Sudo       bool
	Items      []Item
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
	for _, s := range stages {
		if err := ctx.Err(); err != nil {
			return reports, err
		}
		if mode == Force {
			r, err := s.Apply(ctx, true)
			Show(u, r)
			reports = append(reports, r)
			if err != nil {
				return reports, err
			}
			continue
		}
		r := s.Evaluate(ctx)
		if mode == Status || r.State != Pending {
			if mode == Status {
				Show(u, r)
			}
			reports = append(reports, r)
			continue
		}
		r, err := s.Apply(ctx, false)
		Show(u, r)
		reports = append(reports, r)
		if err != nil {
			return reports, err
		}
	}
	return reports, ctx.Err()
}

var ErrRefused = errors.New("sudo refused; nothing applied as root")

func Unreached(stages []Stage, state State, detail string) []Report {
	reports := make([]Report, 0, len(stages))
	for _, s := range stages {
		r := Report{Stage: s.Name, State: state}
		for _, it := range s.Items {
			r.Items = append(r.Items, Result{Item: it.Name, State: state, Detail: detail})
		}
		reports = append(reports, r)
	}
	return reports
}

func (s Stage) Evaluate(ctx context.Context) Report {
	r := Report{Stage: s.Name}
	for _, it := range s.Items {
		r.Items = append(r.Items, it.evaluate(ctx))
	}
	r.State = worst(r.Items)
	return r
}

// Apply stops at a canceled prompt or context and returns that error so the whole run ends.
func (s Stage) Apply(ctx context.Context, force bool) (Report, error) {
	r := Report{Stage: s.Name}
	err := ctx.Err()
	for _, it := range s.Items {
		if err != nil {
			break
		}
		var res Result
		res, err = it.apply(ctx, force)
		r.Items = append(r.Items, res)
		if err == nil {
			err = ctx.Err()
		}
	}
	r.State = worst(r.Items)
	return r, err
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

func (it Item) apply(ctx context.Context, force bool) (Result, error) {
	if !force {
		if r := it.evaluate(ctx); r.State != Pending {
			return r, nil
		}
	}
	if it.Fix != nil {
		if err := it.Fix(ctx); err != nil {
			r := Result{Item: it.Name, State: Failed, Detail: "fix: " + err.Error()}
			if isManual(err) {
				r.State = ManualState
			}
			if errors.Is(err, ui.ErrCanceled) {
				return r, err
			}
			return r, nil
		}
	}
	r := it.evaluate(ctx)
	if r.State == Pending {
		r.State = Failed
	}
	return r, nil
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

func summary(r Report) string {
	var open []string
	for _, it := range r.Items {
		if it.State != Done {
			open = append(open, short(r, it.Item))
		}
	}
	if len(open) == 0 {
		return string(r.State)
	}
	return fmt.Sprintf("%s: %s", r.State, strings.Join(open, ", "))
}

func Show(u *ui.UI, r Report) {
	u.Node(level(r.State), fmt.Sprintf("%-10s", r.Stage), summary(r))
	for _, it := range r.Items {
		if it.State != Done {
			u.Detail("%s (%s): %s", short(r, it.Item), it.State, it.Detail)
		}
	}
}

func Outcome(reports []Report, mode Mode) (ui.Level, string) {
	var failed, open, deferred []string
	for _, r := range reports {
		switch {
		case r.State == Done:
		case slices.ContainsFunc(r.Items, func(it Result) bool { return it.State == Failed }):
			failed = append(failed, r.Stage)
		case r.State == ManualState && mode != Status:
			deferred = append(deferred, fmt.Sprintf("%s (%s)", r.Stage, r.State))
		default:
			open = append(open, fmt.Sprintf("%s (%s)", r.Stage, r.State))
		}
	}
	switch {
	case len(failed) > 0:
		return ui.Err, "setup failed: " + strings.Join(failed, ", ")
	case len(open) > 0:
		return ui.Warn, "setup incomplete: " + strings.Join(open, ", ")
	case len(deferred) > 0:
		return ui.Warn, "setup complete with deferred: " + strings.Join(deferred, ", ")
	}
	return ui.OK, "setup complete"
}

// Incomplete fails on every item that is neither done nor manual, except unknown items in Status mode.
func Incomplete(reports []Report, mode Mode) bool {
	return slices.ContainsFunc(reports, func(r Report) bool {
		return slices.ContainsFunc(r.Items, func(it Result) bool {
			switch it.State {
			case Done, ManualState:
				return false
			case Unknown:
				return mode != Status
			}
			return true
		})
	})
}
