package setup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/ui"
)

// Flow runs lock stages only after every required phase-1 report is Done and the user confirms; Run alone bypasses this gate.
type Flow struct {
	UI      *ui.UI
	Stages  []Stage
	Home    string
	Next    string
	Elevate func(ctx context.Context, stages []Stage, mode Mode, quiet bool) ([]Report, error)
}

type Request struct {
	Names  []string
	From   string
	Status bool
}

const lockQuestion = "Everything works. Lock it down now? YubiKeys, then Secure Boot"

func (f *Flow) Run(ctx context.Context, req Request) error {
	u := f.UI
	sc, err := f.scope(req)
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	if req.Status {
		reports, err := f.evaluate(ctx, sc.shown)
		if err != nil {
			return err
		}
		return f.status(sc.plan(reports, !sc.full || f.gate(reports)), reports, host)
	}
	reports, err := f.evaluate(ctx, sc.shown)
	if err != nil {
		return err
	}
	rows := sc.plan(reports, f.gate(reports))
	steps := 0
	for _, w := range rows {
		if w.step() {
			steps++
		}
	}
	if steps > 0 {
		u.Begin("plan", host)
		for _, w := range rows {
			w.show(u)
		}
		question := fmt.Sprintf("Run %d steps?", steps)
		if steps == 1 {
			question = "Run 1 step?"
		}
		ok, err := u.Proceed(question)
		if err != nil {
			return err
		}
		if !ok {
			u.End(ui.Info, "nothing changed")
			return nil
		}
		u.End(ui.Info, "")
	}
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	run := &execution{flow: f, scope: sc, results: reports, stop: stop}
	run.phase(ctx, rows)
	if !run.canceled && ctx.Err() == nil {
		run.lock(ctx)
	}
	return run.summary(ctx, req)
}

func (f *Flow) scope(req Request) (scope, error) {
	all := f.Stages
	for _, name := range append(slices.Clone(req.Names), req.From) {
		if name != "" && f.find(name) < 0 {
			known := make([]string, len(all))
			for i, s := range all {
				known[i] = s.Name
			}
			return scope{}, fmt.Errorf("unknown stage %q (known: %s)", name, strings.Join(known, ", "))
		}
	}
	sc := scope{force: map[string]bool{}, pulled: map[string]string{}, named: map[string]bool{}}
	for _, name := range req.Names {
		sc.named[name] = true
	}
	if len(req.Names) == 0 && req.From == "" {
		sc.shown, sc.full = slices.Clone(all), true
		return sc, nil
	}
	pick := map[string]bool{}
	if req.From != "" {
		for _, s := range all[f.find(req.From):] {
			if !s.Optional || sc.named[s.Name] {
				pick[s.Name], sc.force[s.Name] = true, !req.Status
			}
		}
	}
	for name := range sc.named {
		pick[name], sc.force[name] = true, !req.Status
	}
	if !req.Status {
		for _, s := range all {
			if pick[s.Name] {
				f.pull(s, s.Name, pick, sc.pulled)
			}
		}
	}
	for _, s := range all {
		if pick[s.Name] {
			sc.shown = append(sc.shown, s)
		}
	}
	return sc, nil
}

func (f *Flow) pull(s Stage, by string, pick map[string]bool, pulled map[string]string) {
	for _, need := range s.Needs {
		if pick[need] {
			continue
		}
		pick[need], pulled[need] = true, "needed by "+by
		f.pull(f.Stages[f.find(need)], by, pick, pulled)
	}
}

func (f *Flow) find(name string) int {
	return slices.IndexFunc(f.Stages, func(s Stage) bool { return s.Name == name })
}

func (f *Flow) gate(reports map[string]Report) bool {
	for _, s := range f.Stages {
		if s.Lock || s.Optional {
			continue
		}
		if r := reports[s.Name]; r.State != Done || r.Wait != "" {
			return false
		}
	}
	return true
}

func (f *Flow) evaluate(ctx context.Context, stages []Stage) (map[string]Report, error) {
	out := map[string]Report{}
	var root []Stage
	for _, s := range stages {
		if s.Root {
			root = append(root, s)
		}
	}
	if len(root) > 0 {
		rs, err := f.elevate(ctx, root, Status, true)
		switch {
		case errors.Is(err, ui.ErrCanceled) || ctx.Err() != nil:
			return nil, errors.Join(err, ctx.Err())
		case errors.Is(err, ErrRefused):
			rs = Unreached(root, Unknown, "sudo authorization failed")
		case err != nil:
			rs = Unreached(root, Unknown, err.Error())
		}
		for _, r := range rs {
			out[r.Stage] = r
		}
	}
	for _, s := range stages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if s.Root {
			if _, ok := out[s.Name]; !ok {
				out[s.Name] = Unreached([]Stage{s}, Unknown, "no report from the root check")[0]
			}
			continue
		}
		out[s.Name] = s.Evaluate(ctx)
	}
	return out, nil
}

func (f *Flow) elevate(ctx context.Context, stages []Stage, mode Mode, quiet bool) ([]Report, error) {
	if f.Elevate != nil {
		return f.Elevate(ctx, stages, mode, quiet)
	}
	return Run(ctx, f.UI, stages, mode)
}

func (f *Flow) status(rows []row, reports map[string]Report, host string) error {
	u := f.UI
	u.Begin("status", host)
	var list []Report
	var parts []string
	for _, w := range rows {
		w.show(u)
		if w.mark != optMark {
			list = append(list, reports[w.stage.Name])
		}
		if w.mark != okMark && w.mark != optMark {
			parts = append(parts, w.label())
		}
	}
	if u.JSON() {
		all := make([]Report, 0, len(rows))
		for _, w := range rows {
			all = append(all, reports[w.stage.Name])
		}
		if err := u.Emit(all); err != nil {
			return err
		}
	}
	level, msg := ui.OK, "all set up"
	if len(parts) > 0 {
		level, msg = ui.Warn, "not done: "+strings.Join(parts, ", ")
	}
	if Incomplete(list, Status) {
		return errors.New(msg)
	}
	u.End(level, "%s", msg)
	return nil
}

type execution struct {
	flow     *Flow
	scope    scope
	results  map[string]Report
	waits    map[string]func(context.Context) (Report, error)
	ran      map[string]bool
	asked    bool
	declined bool
	canceled bool
	stop     context.CancelCauseFunc
}

func (e *execution) cancel() {
	e.canceled = true
	e.stop(ui.ErrCanceled)
}

func (e *execution) phase(ctx context.Context, rows []row) {
	var todo []row
	for _, w := range rows {
		if w.step() {
			todo = append(todo, w)
		}
	}
	e.apply(ctx, todo)
}

func (e *execution) apply(ctx context.Context, todo []row) {
	f, u := e.flow, e.flow.UI
	if e.ran == nil {
		e.ran, e.waits = map[string]bool{}, map[string]func(context.Context) (Report, error){}
	}
	for i := 0; i < len(todo) && !e.canceled; {
		s := todo[i].stage
		if ctx.Err() != nil {
			e.cancel()
			break
		}
		if need := e.blocked(s); need != "" {
			r := e.results[s.Name]
			r.Wait = need
			e.results[s.Name] = r
			i++
			continue
		}
		if s.Root || s.Sudo || s.Lock || slices.ContainsFunc(s.Needs, func(n string) bool { return e.waits[n] != nil }) {
			e.collect(ctx)
		}
		switch {
		case s.Root:
			j := i + 1
			for j < len(todo) && todo[j].stage.Root && todo[j].force == todo[i].force && e.inside(todo[j].stage, todo[i:j]) {
				j++
			}
			group := make([]Stage, 0, j-i)
			for _, w := range todo[i:j] {
				group = append(group, w.stage)
				e.ran[w.stage.Name] = true
			}
			mode := All
			if todo[i].force {
				mode = Force
			}
			rs, err := f.elevate(ctx, group, mode, false)
			switch {
			case errors.Is(err, ErrRefused):
				rs = Unreached(group, Failed, ErrRefused.Error())
			case errors.Is(err, ui.ErrCanceled) || ctx.Err() != nil:
				e.cancel()
			case err != nil:
				if len(rs) == 0 {
					rs = Unreached(group, Unknown, err.Error())
				}
			}
			for _, r := range rs {
				e.results[r.Stage] = r
			}
			i = j
		case s.Background:
			i++
			if !todo[i-1].force {
				if r := s.Evaluate(ctx); r.State != Pending {
					e.results[s.Name] = r
					continue
				}
			}
			e.ran[s.Name] = true
			wait, err := Start(ctx, u, s, todo[i-1].force, f.Home)
			switch {
			case errors.Is(err, context.Canceled):
				e.cancel()
			case err != nil:
				e.results[s.Name] = Unreached([]Stage{s}, Failed, err.Error())[0]
			default:
				e.waits[s.Name] = wait
			}
		default:
			i++
			force := todo[i-1].force
			if !force {
				if r := s.Evaluate(ctx); r.State != Pending {
					e.results[s.Name] = r
					continue
				}
			}
			e.ran[s.Name] = true
			r, err := s.section(ctx, u, force)
			e.results[s.Name] = r
			if err != nil {
				u.End(ui.Warn, "canceled")
				e.cancel()
			}
		}
	}
	e.collect(ctx)
}

func (e *execution) blocked(s Stage) string {
	for _, need := range s.Needs {
		if e.waits[need] != nil {
			continue
		}
		if r, ok := e.results[need]; ok && (r.State != Done || r.Wait != "") {
			return need
		}
	}
	return ""
}

func (e *execution) inside(s Stage, group []row) bool {
	for _, need := range s.Needs {
		if slices.ContainsFunc(group, func(w row) bool { return w.stage.Name == need }) {
			continue
		}
		if r, ok := e.results[need]; ok && (r.State != Done || r.Wait != "") {
			return false
		}
	}
	return true
}

func (e *execution) collect(ctx context.Context) {
	for _, s := range e.flow.Stages {
		wait := e.waits[s.Name]
		if wait == nil {
			continue
		}
		delete(e.waits, s.Name)
		r, err := wait(ctx)
		e.results[s.Name] = r
		if errors.Is(err, ui.ErrCanceled) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			e.cancel()
		}
	}
}

func (e *execution) lock(ctx context.Context) {
	f, u := e.flow, e.flow.UI
	if !f.gate(e.results) {
		return
	}
	var stages []Stage
	for _, s := range e.scope.shown {
		if s.Lock {
			stages = append(stages, s)
		}
	}
	rows := scope{shown: stages, force: e.scope.force, pulled: e.scope.pulled, named: e.scope.named}.plan(e.results, true)
	if !slices.ContainsFunc(rows, func(w row) bool { return w.mark == todoMark }) {
		return
	}
	e.asked = true
	if u.Yes() {
		e.declined = true
		return
	}
	u.Begin("lock", "")
	for _, w := range rows {
		w.show(u)
	}
	ok, err := u.Confirm(lockQuestion)
	switch {
	case err != nil:
		u.End(ui.Warn, "")
		e.declined = true
		if errors.Is(err, ui.ErrCanceled) {
			e.cancel()
		}
		return
	case !ok:
		u.End(ui.Info, "")
		e.declined = true
		return
	}
	u.End(ui.Info, "")
	var todo []row
	for _, w := range rows {
		if w.mark == todoMark {
			todo = append(todo, w)
		}
	}
	e.apply(ctx, todo)
}

func (e *execution) summary(ctx context.Context, req Request) error {
	u := e.flow.UI
	var failed, skipped, fixes, laters, open []string
	var rows []row
	var all []Report
	gate := e.flow.gate(e.results)
	for _, s := range e.scope.shown {
		r, ok := e.results[s.Name]
		if !ok {
			continue
		}
		all = append(all, r)
		if s.Optional && !e.scope.named[s.Name] {
			continue
		}
		w := row{stage: s}
		switch {
		case r.Wait != "":
			w.mark, w.text = waitMark, "after "+r.Wait
			if e.stuck(r.Wait) {
				skipped = append(skipped, s.Name)
			}
		case r.State == Done:
			continue
		case r.State == ManualState:
			w.mark = fixMark
			w.text, w.more = headline(r, ManualState)
			fixes = append(fixes, s.Name)
		case r.State == LaterState:
			w.mark = laterMark
			w.text, w.more = headline(r, LaterState)
			laters = append(laters, s.Name)
		case s.Lock && !gate && r.State == Pending:
			w.mark, w.text = waitMark, "after every required stage is OK"
		case r.State == Pending:
			open = append(open, s.Name)
			continue
		default:
			w.mark = errMark
			w.text, _ = headline(r, r.State)
			failed = append(failed, s.Name)
		}
		rows = append(rows, w)
	}
	level, outcome, next := ui.OK, "complete", ""
	if e.scope.full {
		next = "Everything is set up and locked down."
	}
	switch {
	case e.canceled:
		level, outcome, next = ui.Warn, "canceled", "rerun `dctl setup` to continue"
	case len(failed)+len(skipped) > 0:
		level, outcome, next = ui.Err, strings.Join(slices.DeleteFunc([]string{labeled("failed", failed), labeled("skipped", skipped)}, func(s string) bool { return s == "" }), "; "), "fix the failed stages, then run `dctl setup`"
	case len(fixes) > 0:
		next = fixes[0] + ": " + rows[slices.IndexFunc(rows, func(w row) bool { return w.mark == fixMark })].text
	case len(laters) > 0:
		next = e.later(rows)
	case e.ran["totp"] && e.results["totp"].State == Done:
		next = "Reboot and compare the boot TOTP with your authenticator before unlocking."
	case len(open) > 0 && e.asked && u.Yes():
		next = "run `dctl setup` without --yes to lock it down"
	case len(open) > 0:
		next = "run `dctl setup` again to lock it down"
	}
	if !e.canceled && len(failed)+len(skipped) == 0 {
		var parts []string
		for _, w := range rows {
			parts = append(parts, w.label())
		}
		switch {
		case len(parts) > 0:
			level, outcome = ui.Warn, "complete except "+strings.Join(parts, ", ")
		case len(open) > 0:
			level, outcome = ui.Warn, "set up, not locked down yet"
		}
	}
	u.Begin("summary", "")
	for _, w := range rows {
		w.show(u)
	}
	if next != "" {
		u.Row(ui.Info, "next: "+next)
	}
	if e.scope.full {
		e.hint(next, len(failed)+len(skipped)+len(fixes)+len(laters)+len(open) == 0 && !e.canceled)
	}
	if u.JSON() {
		if err := u.Emit(all); err != nil {
			return err
		}
	}
	switch {
	case e.canceled:
		return ui.ErrCanceled
	case len(failed)+len(skipped) > 0:
		return errors.New(outcome)
	}
	u.End(level, "%s", outcome)
	return ctx.Err()
}

func (e *execution) stuck(name string) bool {
	r := e.results[name]
	if r.Wait != "" {
		return e.stuck(r.Wait)
	}
	return r.State != Done && r.State != ManualState && r.State != LaterState
}

func labeled(head string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	return head + ": " + strings.Join(names, ", ")
}

func (e *execution) later(rows []row) string {
	if r := e.results["secureboot"]; r.State == LaterState && slices.ContainsFunc(r.Items, func(it Result) bool { return it.Item == "secureboot-keys" && it.State == Done }) {
		return "Reboot and unlock with a YubiKey's FIDO2 PIN; if Secure Boot is still off, turn it on with F2 and save with F10. Then run `dctl setup` to seal the boot TOTP."
	}
	w := rows[slices.IndexFunc(rows, func(w row) bool { return w.mark == laterMark })]
	return w.stage.Name + ": " + w.text
}

func (e *execution) hint(next string, done bool) {
	path := e.flow.Next
	if path == "" {
		return
	}
	if done {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			e.flow.UI.Warn("could not remove setup hint %s: %v", path, err)
		}
		return
	}
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = os.WriteFile(path, []byte(strings.ReplaceAll(next, "`", "")+"\n"), 0o644)
	}
	if err != nil {
		e.flow.UI.Warn("could not save setup hint %s: %v", path, err)
	}
}
