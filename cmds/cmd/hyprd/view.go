package main

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"sync"

	"dotfiles/cmds/internal/daemon"
	"dotfiles/cmds/internal/hyprd/kitty"
)

type View struct {
	views *kitty.Tracker
	subs  *daemon.SubscriptionManager
	dirty chan struct{}
	mu    sync.Mutex
	panes []viewPane
	fault string
}

type viewPane struct {
	kitty.PaneKey
	Shown bool `json:"shown"`
}

type viewPayload struct {
	Panes []viewPane `json:"panes"`
}

var viewEvents = map[string]bool{
	"workspace":        true,
	"workspacev2":      true,
	"focusedmon":       true,
	"activespecial":    true,
	"moveworkspacev2":  true,
	"monitoraddedv2":   true,
	"monitorremovedv2": true,
	"openwindow":       true,
	"movewindowv2":     true,
	"closewindow":      true,
}

func NewView(views *kitty.Tracker, subs *daemon.SubscriptionManager) *View {
	return &View{views: views, subs: subs, dirty: make(chan struct{}, 1), panes: []viewPane{}}
}

func (v *View) Mark() {
	select {
	case v.dirty <- struct{}{}:
	default:
	}
}

func (v *View) Run(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-v.dirty:
			v.refresh()
		}
	}
}

func (v *View) Payload() viewPayload {
	v.mu.Lock()
	defer v.mu.Unlock()
	return viewPayload{Panes: v.panes}
}

func (v *View) refresh() {
	_, seen, err := v.views.View()
	fault := ""
	if err != nil {
		fault = err.Error()
	}
	if fault != v.fault && fault != "" {
		fmt.Fprintf(os.Stderr, "hyprd view: %s\n", fault)
	}
	v.fault = fault
	panes := make([]viewPane, 0, len(seen))
	for key, pane := range seen {
		panes = append(panes, viewPane{PaneKey: key, Shown: pane.Shown})
	}
	slices.SortFunc(panes, func(a, b viewPane) int {
		return cmp.Or(cmp.Compare(a.KittyPID, b.KittyPID), cmp.Compare(a.PaneID, b.PaneID))
	})

	v.mu.Lock()
	changed := !slices.Equal(v.panes, panes)
	v.panes = panes
	v.mu.Unlock()
	if changed {
		v.subs.Notify("view", viewPayload{Panes: panes})
	}
}
