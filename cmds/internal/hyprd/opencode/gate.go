package opencode

import (
	"fmt"
	"time"

	"dotfiles/cmds/internal/hyprd/kitty"
)

const (
	autoAge   = 4 * time.Hour
	reportTTL = 3 * time.Minute
	hiddenFor = time.Minute
)

type Report struct {
	Start   string  `json:"start"`
	Session *string `json:"session"`
	At      int64   `json:"at"`
	Draft   *bool   `json:"draft"`
	Tabs    *int    `json:"tabs"`
	Pending *bool   `json:"pending"`
}

func (p Pane) key() kitty.PaneKey {
	return kitty.PaneKey{KittyPID: p.KittyPID, PaneID: p.KittyPaneID}
}

func (p Pane) gate(now time.Time, minAge time.Duration) string {
	report := p.Report
	// Report shell/age blockers before view, and view before TUI state.
	// Unknown facts block because recycling can discard unsaved input.
	switch {
	case !p.Shell:
		return "no login shell to relaunch into"
	case minAge > 0 && p.Started.IsZero():
		return "start time unknown"
	case now.Sub(p.Started) < minAge:
		return "younger than " + hours(minAge) + " (" + hours(now.Sub(p.Started)) + ")"
	case p.Shown == nil:
		return "view unknown"
	case *p.Shown:
		return "in view"
	case p.Hidden.IsZero():
		return "out of view since unknown"
	case now.Sub(p.Hidden) < hiddenFor:
		return fmt.Sprintf("out of view only %s", now.Sub(p.Hidden).Truncate(time.Second))
	case report == nil:
		return "TUI report missing"
	case now.Sub(time.UnixMilli(report.At)) > reportTTL:
		return "TUI report stale"
	case report.Draft == nil:
		return "draft unknown"
	case *report.Draft:
		return "draft present"
	case report.Tabs == nil:
		return "tab count unknown"
	case *report.Tabs > 1:
		return fmt.Sprintf("%d OpenCode tabs open", *report.Tabs)
	case report.Pending == nil:
		return "displayed questions unknown"
	case *report.Pending:
		return "question or permission displayed"
	}
	return ""
}

func hours(d time.Duration) string {
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
