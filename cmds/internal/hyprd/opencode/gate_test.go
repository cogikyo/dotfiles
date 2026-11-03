package opencode

import (
	"testing"
	"time"
)

func TestGate(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	yes, no := true, false
	one, two := 1, 2
	fresh := func(edit func(*Report)) *Report {
		report := &Report{At: now.Add(-time.Second).UnixMilli(), Draft: &no, Tabs: &one, Pending: &no}
		if edit != nil {
			edit(report)
		}
		return report
	}
	pane := func(edit func(*Pane)) Pane {
		p := Pane{Started: now.Add(-5 * time.Hour), Shell: true, Shown: &no, Hidden: now.Add(-5 * time.Minute), Report: fresh(nil)}
		if edit != nil {
			edit(&p)
		}
		return p
	}

	tests := []struct {
		name   string
		pane   Pane
		minAge time.Duration
		want   string
	}{
		{"eligible auto", pane(nil), autoAge, ""},
		{"eligible manual", pane(nil), 0, ""},
		{"no login shell", pane(func(p *Pane) { p.Shell = false }), 0, "no login shell to relaunch into"},
		{"young auto", pane(func(p *Pane) { p.Started = now.Add(-90 * time.Minute) }), autoAge, "younger than 4h00m (1h30m)"},
		{"young manual", pane(func(p *Pane) { p.Started = now.Add(-time.Minute) }), 0, ""},
		{"start unknown auto", pane(func(p *Pane) { p.Started = time.Time{} }), autoAge, "start time unknown"},
		{"start unknown manual", pane(func(p *Pane) { p.Started = time.Time{} }), 0, ""},
		{"view unknown", pane(func(p *Pane) { p.Shown = nil }), 0, "view unknown"},
		{"in view", pane(func(p *Pane) { p.Shown = &yes }), 0, "in view"},
		{"hidden since unknown", pane(func(p *Pane) { p.Hidden = time.Time{} }), 0, "out of view since unknown"},
		{"hidden briefly", pane(func(p *Pane) { p.Hidden = now.Add(-30 * time.Second) }), 0, "out of view only 30s"},
		{"hidden a minute", pane(func(p *Pane) { p.Hidden = now.Add(-hiddenFor) }), 0, ""},
		{"report missing", pane(func(p *Pane) { p.Report = nil }), 0, "TUI report missing"},
		{"report stale", pane(func(p *Pane) {
			p.Report = fresh(func(r *Report) { r.At = now.Add(-reportTTL - time.Second).UnixMilli() })
		}), 0, "TUI report stale"},
		{"report without heartbeat", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.At = 0 }) }), 0, "TUI report stale"},
		{"draft unknown", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.Draft = nil }) }), 0, "draft unknown"},
		{"draft present", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.Draft = &yes }) }), 0, "draft present"},
		{"tabs unknown", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.Tabs = nil }) }), 0, "tab count unknown"},
		{"several tabs", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.Tabs = &two }) }), 0, "2 OpenCode tabs open"},
		{"pending field missing", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.Pending = nil }) }), 0, "displayed questions unknown"},
		{"form displayed", pane(func(p *Pane) { p.Report = fresh(func(r *Report) { r.Pending = &yes }) }), 0, "question or permission displayed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.pane.gate(now, test.minAge); got != test.want {
				t.Errorf("gate() = %q, want %q", got, test.want)
			}
		})
	}
}
