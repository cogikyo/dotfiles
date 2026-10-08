package setup

import (
	"fmt"
	"slices"
	"strings"

	"dotfiles/cmds/internal/ui"
)

type mark int

const (
	okMark mark = iota
	todoMark
	fixMark
	laterMark
	optMark
	waitMark
	errMark
)

type row struct {
	stage Stage
	mark  mark
	text  string
	more  string
	force bool
}

var levels = map[mark]ui.Level{okMark: ui.OK, todoMark: ui.Todo, fixMark: ui.Fix, laterMark: ui.Later, optMark: ui.Optional, waitMark: ui.Wait, errMark: ui.Err}

var labels = map[mark]string{todoMark: "TODO", fixMark: "FIX", laterMark: "LATER", waitMark: "WAIT", errMark: "ERR"}

func (w row) show(u *ui.UI) {
	u.Row(levels[w.mark], fmt.Sprintf("%-10s  %s", w.stage.Name, w.text))
	if w.more != "" {
		u.Detail("%s", w.more)
	}
}

func (w row) label() string { return labels[w.mark] + " " + w.stage.Name }

func (w row) step() bool { return w.mark == todoMark && !w.stage.Lock }

type scope struct {
	shown  []Stage
	force  map[string]bool
	pulled map[string]string
	named  map[string]bool
	full   bool
}

func (sc scope) plan(reports map[string]Report, gate bool) []row {
	marks := map[string]mark{}
	rows := make([]row, 0, len(sc.shown))
	for _, s := range sc.shown {
		w := sc.mark(s, reports[s.Name], marks, gate)
		marks[s.Name] = w.mark
		rows = append(rows, w)
	}
	return rows
}

func (sc scope) mark(s Stage, r Report, marks map[string]mark, gate bool) row {
	w := row{stage: s, force: sc.force[s.Name]}
	switch {
	case s.Optional && !sc.named[s.Name] && r.State != Done:
		w.mark, w.text = optMark, fmt.Sprintf("`dctl setup %s`", s.Name)
		return w
	case r.State == Done && !w.force:
		w.mark, w.text = okMark, s.About
		return w
	case r.State == LaterState && (!w.force || deferred(r)):
		w.mark = laterMark
		w.text, w.more = headline(r, LaterState)
		return w
	case s.Lock && !gate:
		w.mark, w.text = waitMark, "after every required stage is OK"
		return w
	}
	for _, need := range s.Needs {
		m, ok := marks[need]
		if !ok || m == okMark {
			continue
		}
		if m == todoMark {
			w.mark, w.text = todoMark, about(s, "after "+need)
			return w
		}
		w.mark, w.text = waitMark, "after "+need
		return w
	}
	switch {
	case r.State == Pending || w.force:
		w.mark, w.text = todoMark, about(s, sc.pulled[s.Name])
	case r.State == ManualState:
		w.mark = fixMark
		w.text, w.more = headline(r, ManualState)
	default:
		w.mark = errMark
		w.text, w.more = headline(r, r.State)
	}
	return w
}

func about(s Stage, note string) string {
	switch {
	case note == "":
		return s.About
	case s.About == "":
		return note
	}
	return s.About + " · " + note
}

func deferred(r Report) bool {
	return !slices.ContainsFunc(r.Items, func(it Result) bool { return it.State != LaterState })
}

func headline(r Report, state State) (head, rest string) {
	i := slices.IndexFunc(r.Items, func(it Result) bool { return it.State == state })
	if i < 0 {
		return string(state), ""
	}
	head, rest, _ = strings.Cut(r.Items[i].Detail, "\n")
	return head, rest
}
