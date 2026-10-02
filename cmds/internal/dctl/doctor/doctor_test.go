package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/ui"
)

type probe struct {
	healthy bool
	fixErr  error
	cures   bool
	fixes   int
}

func (p *probe) check(name string) Check {
	return Check{
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

func runAll(t *testing.T, opts Options, groups ...Group) map[string]Result {
	t.Helper()
	results, err := Run(context.Background(), ui.New(ui.Options{JSON: true}), groups, opts)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Result{}
	for _, r := range results {
		out[r.Check] = r
	}
	return out
}

func TestFixRecheck(t *testing.T) {
	cured := &probe{cures: true}
	stubborn := &probe{}
	failing := &probe{fixErr: errors.New("nope")}
	results := runAll(t, Options{Fix: true},
		Group{Name: "a", Checks: []Check{cured.check("cured"), stubborn.check("stubborn"), failing.check("failing")}},
	)
	for name, want := range map[string]Status{"cured": Fixed, "stubborn": Failed, "failing": Failed} {
		if got := results[name].Status; got != want {
			t.Errorf("%s: status %s, want %s (%s)", name, got, want, results[name].Detail)
		}
	}
	if results["stubborn"].Detail != "recheck: broken" || results["failing"].Detail != "fix: nope" {
		t.Errorf("details: %q, %q", results["stubborn"].Detail, results["failing"].Detail)
	}
}

func TestReadOnlyNeverFixes(t *testing.T) {
	p := &probe{cures: true}
	results := runAll(t, Options{}, Group{Name: "a", Checks: []Check{p.check("x")}})
	if p.fixes != 0 || results["x"].Status != Failed {
		t.Fatalf("fixes %d, status %s", p.fixes, results["x"].Status)
	}
}

func TestBlockedCheckIsNotFixed(t *testing.T) {
	p := &probe{cures: true}
	c := p.check("x")
	c.Check = func(context.Context) error { return errors.Join(errors.New("context"), Block("needs %s", "y")) }
	results := runAll(t, Options{Fix: true, Elevated: true}, Group{Name: "a", Root: true, Checks: []Check{c}})
	if results["x"].Status != Blocked || p.fixes != 0 {
		t.Fatalf("status %s, fixes %d", results["x"].Status, p.fixes)
	}
}

func TestPrivilege(t *testing.T) {
	for _, tc := range []struct {
		name     string
		root     bool
		elevated bool
		want     Status
		hint     string
	}{
		{"root group as user", true, false, Blocked, "sudo dctl doctor --fix a"},
		{"root group as root", true, true, Fixed, ""},
		{"user group as root", false, true, Blocked, "run dctl doctor --fix a as the owning user"},
		{"user group as user", false, false, Fixed, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &probe{cures: true}
			opts := Options{Fix: true, Elevated: tc.elevated, Rerun: "dctl doctor --fix"}
			r := runAll(t, opts, Group{Name: "a", Root: tc.root, Checks: []Check{p.check("x")}})["x"]
			if r.Status != tc.want || !strings.Contains(r.Detail, tc.hint) {
				t.Fatalf("status %s detail %q", r.Status, r.Detail)
			}
			if fixed := p.fixes > 0; fixed != (tc.want == Fixed) {
				t.Fatalf("fixes %d", p.fixes)
			}
		})
	}
}

func TestCancelStopsRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	later := &probe{cures: true}
	interrupted := Check{Name: "interrupted", Check: func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	}}
	results, err := Run(ctx, ui.New(ui.Options{JSON: true}), []Group{
		{Name: "a", Checks: []Check{(&probe{healthy: true}).check("first"), interrupted}},
		{Name: "b", Checks: []Check{later.check("later")}},
	}, Options{Fix: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
	if len(results) != 1 || results[0].Check != "first" || later.fixes != 0 {
		t.Fatalf("results %v, later fixes %d", results, later.fixes)
	}
}

func TestSelect(t *testing.T) {
	p := &probe{}
	all := []Group{
		{Name: "a", Checks: []Check{p.check("x")}},
		{Name: "b", Checks: []Check{p.check("y")}},
	}
	got, err := Select(all, []string{"b", "a"}, false)
	if err != nil || len(got) != 2 || got[0].Name != "a" {
		t.Fatalf("select keeps declared order: %v %v", got, err)
	}
	if _, err := Select(all, []string{"zz"}, false); err == nil {
		t.Error("unknown group accepted")
	}
	dup := append(all, Group{Name: "c", Checks: []Check{p.check("x")}})
	if _, err := Select(dup, nil, false); err == nil {
		t.Error("duplicate check name accepted")
	}
	if _, err := Select(append(all, Group{Name: "a"}), nil, false); err == nil {
		t.Error("duplicate group name accepted")
	}
}
