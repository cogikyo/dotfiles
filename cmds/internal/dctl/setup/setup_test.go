package setup

import (
	"context"
	"errors"
	"testing"

	"dotfiles/cmds/internal/dctl/ui"
)

type probe struct {
	healthy bool
	cures   bool
	fixErr  error
	fixes   int
}

func (p *probe) item(name string) Item {
	return Item{
		Name: name,
		Check: func(context.Context) error {
			if p.healthy {
				return nil
			}
			return errors.New("broken")
		},
		Fix: func(context.Context) error {
			p.fixes++
			if p.fixErr != nil {
				return p.fixErr
			}
			p.healthy = p.cures
			return nil
		},
	}
}

func states(t *testing.T, u *ui.UI, mode Mode, stages ...Stage) map[string]State {
	t.Helper()
	reports, err := Run(context.Background(), u, stages, mode)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]State{}
	for _, r := range reports {
		out[r.Stage] = r.State
		for _, it := range r.Items {
			out[it.Item] = it.State
		}
	}
	return out
}

func quiet(yes bool) *ui.UI { return ui.New(ui.Options{JSON: true, Yes: yes}) }

func TestStatus(t *testing.T) {
	pending := &probe{cures: true}
	unfixable := Item{Name: "unfixable", Check: func(context.Context) error { return errors.New("by hand") }}
	manual := Item{Name: "manual", Check: func(context.Context) error { return errors.Join(errors.New("context"), Manual("press %s", "it")) }}
	got := states(t, quiet(false), Status,
		Stage{Name: "a", Items: []Item{(&probe{healthy: true}).item("ok")}},
		Stage{Name: "b", Items: []Item{pending.item("pending"), unfixable}},
		Stage{Name: "c", Items: []Item{unfixable, manual}},
		Stage{Name: "d", Items: []Item{manual}},
	)
	want := map[string]State{
		"a": Done, "ok": Done,
		"b": Pending, "pending": Pending, "unfixable": Failed,
		"c": Failed, "manual": ManualState,
		"d": ManualState,
	}
	for name, s := range want {
		if got[name] != s {
			t.Errorf("%s: %s, want %s", name, got[name], s)
		}
	}
	if pending.fixes != 0 {
		t.Errorf("status ran %d fixes", pending.fixes)
	}
}

func TestApply(t *testing.T) {
	cured := &probe{cures: true}
	stubborn := &probe{}
	refused := &probe{fixErr: Manual("insert the key")}
	broken := &probe{fixErr: errors.New("nope")}
	healthy := &probe{healthy: true}
	got := states(t, quiet(true), Ask, Stage{Name: "a", Items: []Item{
		cured.item("cured"), stubborn.item("stubborn"), refused.item("refused"), broken.item("broken"), healthy.item("healthy"),
	}})
	want := map[string]State{"cured": Done, "stubborn": Failed, "refused": ManualState, "broken": Failed, "healthy": Done, "a": Failed}
	for name, s := range want {
		if got[name] != s {
			t.Errorf("%s: %s, want %s", name, got[name], s)
		}
	}
	if healthy.fixes != 0 {
		t.Errorf("pending-only apply fixed a done item %d times", healthy.fixes)
	}
	states(t, quiet(false), Force, Stage{Name: "a", Items: []Item{healthy.item("healthy")}})
	if healthy.fixes != 1 {
		t.Errorf("forced apply fixed a done item %d times, want 1", healthy.fixes)
	}
}

func TestLaterStageSeesEarlierApply(t *testing.T) {
	repo := &probe{cures: true}
	link := &probe{cures: true}
	gated := link.item("link")
	check := gated.Check
	gated.Check = func(ctx context.Context) error {
		if !repo.healthy {
			return Manual("clone the repo first")
		}
		return check(ctx)
	}
	got := states(t, quiet(false), All, Stage{Name: "repos", Items: []Item{repo.item("clone")}}, Stage{Name: "firefox", Items: []Item{gated}})
	if got["repos"] != Done || got["firefox"] != Done || link.fixes != 1 {
		t.Fatalf("states %v, link fixes %d", got, link.fixes)
	}
}

func TestAskNeedsTerminal(t *testing.T) {
	p := &probe{cures: true}
	_, err := Run(context.Background(), quiet(false), []Stage{{Name: "a", Items: []Item{p.item("x")}}}, Ask)
	if !errors.Is(err, ui.ErrNoTTY) || p.fixes != 0 {
		t.Fatalf("err %v, fixes %d", err, p.fixes)
	}
}

func TestSelect(t *testing.T) {
	p := &probe{}
	all := []Stage{{Name: "a", Items: []Item{p.item("x")}}, {Name: "b", Items: []Item{p.item("y")}}}
	got, err := Select(all, []string{"b", "a"})
	if err != nil || len(got) != 2 || got[0].Name != "a" {
		t.Fatalf("select keeps declared order: %v %v", got, err)
	}
	if _, err := Select(all, []string{"zz"}); err == nil {
		t.Error("unknown stage accepted")
	}
}

func TestIncomplete(t *testing.T) {
	reports := func(s State) []Report {
		return []Report{{State: Done, Items: []Result{{State: Done}, {State: ManualState}, {State: Unknown}}}, {State: s, Items: []Result{{State: s}}}}
	}
	for _, tc := range []struct {
		state State
		mode  Mode
		want  bool
	}{
		{Done, Status, false},
		{Pending, Status, true},
		{Failed, Status, true},
		{Pending, Ask, false},
		{Pending, All, false},
		{Failed, Ask, true},
		{Failed, Force, true},
	} {
		if got := Incomplete(reports(tc.state), tc.mode); got != tc.want {
			t.Errorf("%s under %s: incomplete %v, want %v", tc.state, tc.mode, got, tc.want)
		}
	}
}
