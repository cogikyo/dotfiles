package notify

// actions.go tracks live dunst notifications and routes their activation to Hyprland windows.
//
// Two paths converge here:
//   - dunst emits ActionInvoked when a provider default action fires (mouse click or dunstctl action).
//   - ActivateDisplayed runs from the hyprd keybind and must also serve apps whose notifications
//     carry no provider action at all, by focusing the configured window itself.
//
// Live notifications are learned from the dunst script hook, because `dunstctl history`
// only lists notifications that dunst has already closed.

import (
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/windows"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	dunstNotificationTTL = 30 * time.Minute       // drop notifications dunst never reported closing
	dunstActionGrace     = 150 * time.Millisecond // wait for dunst to report a provider action; the signal precedes dunstctl's reply
	dunstActionPoll      = 15 * time.Millisecond
)

type dunstActionTarget struct {
	Workspace int
	Class     string
	Title     string
}

func (t dunstActionTarget) configured() bool {
	return t.Class != ""
}

// dunstNotification is one notification hyprd saw arrive and has not seen closed.
type dunstNotification struct {
	ID      uint32
	Target  dunstActionTarget
	AddedAt time.Time
}

type dunstActionRouter struct {
	once  sync.Once
	mu    sync.Mutex
	hypr  *hypr.Client
	live  map[uint32]dunstNotification
	acted time.Time // when dunst last reported ActionInvoked
}

var globalDunstActionRouter = &dunstActionRouter{
	live: make(map[uint32]dunstNotification),
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ arrival                                                                      │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// actionFocusTarget resolves the configured window for a notification's app or desktop entry.
//
// Notification app names and desktop entries both appear as config keys because
// providers supply one, the other, or neither.
func (n *Notifier) actionFocusTarget(app, desktopEntry string) dunstActionTarget {
	for _, key := range []string{app, desktopEntry} {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if target, ok := n.cfg.Notify.ActionFocusApps[key]; ok {
			return dunstActionTarget{Workspace: target.Workspace, Class: target.Class, Title: target.Title}
		}
	}
	return dunstActionTarget{}
}

// rememberDunstNotification records an arriving notification so later activation can route it.
func (n *Notifier) rememberDunstNotification(req NotifyRequest) {
	if req.NotificationID <= 0 {
		return
	}
	globalDunstActionRouter.Start(n.hypr)
	globalDunstActionRouter.Remember(dunstNotification{
		ID:      uint32(req.NotificationID),
		Target:  n.actionFocusTarget(req.App, req.DesktopEntry),
		AddedAt: time.Now(),
	})
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ activation                                                                   │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// ActivateDisplayed acts on the notification dunst is showing, reporting false when there is none.
//
// A provider default action is always given first chance. When the notification belongs to a
// configured app and no provider action fires, the configured window is focused here and the
// notification is closed, so the keybind never leaves a stale notification behind.
func (n *Notifier) ActivateDisplayed() (string, bool, error) {
	if dunstDisplayedCount() == 0 {
		return "", false, nil
	}

	globalDunstActionRouter.Start(n.hypr)
	note, known := globalDunstActionRouter.Latest()
	since := time.Now()
	if err := dunstAction(); err != nil {
		return "", true, fmt.Errorf("dunst action: %w", err)
	}
	if !known || !note.Target.configured() {
		return "notification: action", true, nil
	}
	if globalDunstActionRouter.AwaitAction(since, dunstActionGrace) {
		return "notification: action", true, nil
	}

	globalDunstActionRouter.Forget(note.ID)
	closeDunstNotification(int(note.ID))
	if err := globalDunstActionRouter.focus(note.Target); err != nil {
		return "", true, fmt.Errorf("notification focus: %w", err)
	}
	return "notification: focus " + note.Target.Class, true, nil
}

func dunstDisplayedCount() int {
	out, err := exec.Command("dunstctl", "count", "displayed").Output()
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return count
}

// dunstAction invokes the default action of the notification dunst is showing.
//
// dunst silently does nothing when the notification carries no action.
func dunstAction() error {
	return exec.Command("dunstctl", "action").Run()
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ router                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (r *dunstActionRouter) Start(h *hypr.Client) {
	if h == nil {
		return
	}
	r.once.Do(func() {
		r.hypr = h
		go r.run()
	})
}

func (r *dunstActionRouter) Remember(note dunstNotification) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-dunstNotificationTTL)
	for id, live := range r.live {
		if live.AddedAt.Before(cutoff) {
			delete(r.live, id)
		}
	}

	r.live[note.ID] = note
}

// Latest returns the newest notification hyprd has not seen closed.
//
// dunst's notification_limit keeps one notification on screen, so newest is the displayed one.
func (r *dunstActionRouter) Latest() (dunstNotification, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var latest dunstNotification
	var found bool
	for _, live := range r.live {
		if !found || live.AddedAt.After(latest.AddedAt) {
			latest, found = live, true
		}
	}
	return latest, found
}

// AwaitAction reports whether dunst invoked any action after since, waiting up to grace.
//
// Any action means the keypress was consumed by a provider, even when dunst was showing a
// different notification than the newest one hyprd recorded.
func (r *dunstActionRouter) AwaitAction(since time.Time, grace time.Duration) bool {
	deadline := time.Now().Add(grace)
	for {
		r.mu.Lock()
		acted := r.acted.After(since)
		r.mu.Unlock()
		if acted {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(dunstActionPoll)
	}
}

func (r *dunstActionRouter) Forget(id uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.live, id)
}

func (r *dunstActionRouter) run() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd notify: dbus connect: %v\n", err)
		return
	}
	defer conn.Close()

	for _, member := range []string{"ActionInvoked", "NotificationClosed"} {
		if err := conn.AddMatchSignal(
			dbus.WithMatchObjectPath(dbus.ObjectPath("/org/freedesktop/Notifications")),
			dbus.WithMatchInterface("org.freedesktop.Notifications"),
			dbus.WithMatchMember(member),
		); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd notify: dbus add %s match: %v\n", member, err)
			return
		}
	}

	signals := make(chan *dbus.Signal, 32)
	conn.Signal(signals)
	for signal := range signals {
		switch signal.Name {
		case "org.freedesktop.Notifications.ActionInvoked":
			r.handleAction(signal)
		case "org.freedesktop.Notifications.NotificationClosed":
			r.handleClose(signal)
		}
	}
}

// handleAction focuses the configured window once the notification's provider action fires.
func (r *dunstActionRouter) handleAction(signal *dbus.Signal) {
	if len(signal.Body) < 2 {
		return
	}
	id, ok := signal.Body[0].(uint32)
	if !ok {
		return
	}
	if _, ok := signal.Body[1].(string); !ok {
		return
	}

	r.mu.Lock()
	note, known := r.live[id]
	delete(r.live, id)
	r.acted = time.Now()
	r.mu.Unlock()

	if !known || !note.Target.configured() {
		return
	}
	if err := r.focus(note.Target); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd notify: focus action %d: %v\n", id, err)
	}
}

func (r *dunstActionRouter) handleClose(signal *dbus.Signal) {
	if len(signal.Body) == 0 {
		return
	}
	id, ok := signal.Body[0].(uint32)
	if !ok {
		return
	}

	r.mu.Lock()
	delete(r.live, id)
	r.mu.Unlock()
}

// focus prefers a matching window on the target workspace, then any match, then the workspace itself.
func (r *dunstActionRouter) focus(target dunstActionTarget) error {
	if r.hypr == nil || !target.configured() {
		return nil
	}

	clients, err := r.hypr.Clients()
	if err != nil {
		return err
	}

	var fallback *hypr.Window
	for i := range clients {
		client := &clients[i]
		if !windows.MatchesTarget(client, target.Class, target.Title) {
			continue
		}
		if target.Workspace > 0 && client.Workspace.ID == target.Workspace {
			return r.hypr.FocusWindow(client.Address)
		}
		if fallback == nil {
			fallback = client
		}
	}
	if fallback != nil {
		return r.hypr.FocusWindow(fallback.Address)
	}
	if target.Workspace > 0 {
		return r.hypr.FocusWorkspace(target.Workspace)
	}
	return nil
}
