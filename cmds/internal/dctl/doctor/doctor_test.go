package doctor

import (
	"context"
	"errors"
	"os"
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
	results := runAll(t, Options{Fix: true, Elevated: true}, Group{Name: "a", Sudo: true, Checks: []Check{c}})
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
			r := runAll(t, opts, Group{Name: "a", Sudo: tc.root, Checks: []Check{p.check("x")}})["x"]
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
	broken := Check{Name: "a-broken", Check: func(context.Context) error { return errors.New("by hand") }}
	interrupted := Check{Name: "interrupted", Check: func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	}}
	u, out := human(t, true)
	results, err := Run(ctx, u, []Group{
		{Name: "a", Checks: []Check{(&probe{healthy: true}).check("first"), broken, interrupted}},
		{Name: "b", Checks: []Check{later.check("later")}},
	}, Options{Fix: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
	if len(results) != 2 || results[0].Check != "first" || later.fixes != 0 {
		t.Fatalf("results %v, later fixes %d", results, later.fixes)
	}
	if got := out(); got != "  ==> a  1 passed\n   ERR    broken\n        by hand\n" {
		t.Errorf("output %q", got)
	}
}

func human(t *testing.T, plain bool) (*ui.UI, func() string) {
	t.Helper()
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = f
	u := ui.New(ui.Options{Plain: plain})
	os.Stdout = stdout
	return u, func() string {
		data, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func TestShowCollapsesPassingChecks(t *testing.T) {
	healthy := &probe{healthy: true}
	manual := Check{Name: "b-manual", Check: func(context.Context) error { return errors.New("by hand") }}
	run := func(plain bool) (string, []Result) {
		u, out := human(t, plain)
		results, err := Run(context.Background(), u, []Group{
			{Name: "a", Checks: []Check{healthy.check("a-one"), healthy.check("a-two")}},
			{Name: "b", Sudo: true, Checks: []Check{healthy.check("b-ok"), (&probe{}).check("b-broken"), manual}},
			{Name: "c", Checks: []Check{manual}},
		}, Options{Rerun: "dctl doctor --fix"})
		if err != nil {
			t.Fatal(err)
		}
		return out(), results
	}
	if colored, _ := run(false); !strings.Contains(colored, "\x1b[") {
		t.Fatalf("color not forced: %q", colored)
	}
	got, results := run(true)
	want := `  ==> a  2 passed
  ==> b  1 passed
   ERR    broken
        broken
   ERR    manual
        by hand
  Fix: sudo dctl doctor --fix b
  ==> c
   ERR    b-manual
        by hand
`
	if got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
	if got := Summary(results); got != "3 failed, 3 passed" {
		t.Errorf("summary %q", got)
	}
}

func TestSummary(t *testing.T) {
	got := Summary([]Result{{Status: Blocked}, {Status: Fixed}, {Status: Passed}, {Status: Failed}, {Status: Failed}})
	if got != "2 failed, 1 blocked, 1 fixed, 1 passed" {
		t.Errorf("summary %q", got)
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
