package notify

import (
	"dotfiles/daemons/hyprd/hypr"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

type dunstActionTarget struct {
	Class string
}

type dunstActionEntry struct {
	Target  dunstActionTarget
	AddedAt time.Time
}

type dunstActionRouter struct {
	once    sync.Once
	mu      sync.Mutex
	hypr    *hypr.Client
	pending map[uint32]dunstActionEntry
}

var globalDunstActionRouter = &dunstActionRouter{
	pending: make(map[uint32]dunstActionEntry),
}

// actionFocusTargetForDunst derives a focus target from the notification's desktop entry,
// which matches the Hyprland window class for standard apps.
func (n *Notifier) actionFocusTargetForDunst(req NotifyRequest) (dunstActionTarget, bool) {
	class := strings.ToLower(strings.TrimSpace(req.DesktopEntry))
	if class == "" {
		return dunstActionTarget{}, false
	}
	return dunstActionTarget{Class: class}, true
}

func (n *Notifier) rememberDunstAction(req NotifyRequest, target dunstActionTarget) {
	if req.NotificationID <= 0 {
		return
	}
	globalDunstActionRouter.Start(n.hypr)
	globalDunstActionRouter.Remember(uint32(req.NotificationID), target)
}

func (r *dunstActionRouter) Start(h *hypr.Client) {
	if h == nil {
		return
	}
	r.once.Do(func() {
		r.hypr = h
		go r.run()
	})
}

func (r *dunstActionRouter) Remember(id uint32, target dunstActionTarget) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-30 * time.Minute)
	for pendingID, entry := range r.pending {
		if entry.AddedAt.Before(cutoff) {
			delete(r.pending, pendingID)
		}
	}

	r.pending[id] = dunstActionEntry{Target: target, AddedAt: time.Now()}
}

func (r *dunstActionRouter) run() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd notify: dbus connect: %v\n", err)
		return
	}
	defer conn.Close()

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath("/org/freedesktop/Notifications")),
		dbus.WithMatchInterface("org.freedesktop.Notifications"),
		dbus.WithMatchMember("ActionInvoked"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd notify: dbus add ActionInvoked match: %v\n", err)
		return
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(dbus.ObjectPath("/org/freedesktop/Notifications")),
		dbus.WithMatchInterface("org.freedesktop.Notifications"),
		dbus.WithMatchMember("NotificationClosed"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd notify: dbus add NotificationClosed match: %v\n", err)
		return
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
	entry, ok := r.pending[id]
	if ok {
		delete(r.pending, id)
	}
	r.mu.Unlock()
	if !ok {
		return
	}

	if err := r.focus(entry.Target); err != nil {
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
	delete(r.pending, id)
	r.mu.Unlock()
}

const notifyFocusWorkspace = 2

func (r *dunstActionRouter) focus(target dunstActionTarget) error {
	if r.hypr == nil || target.Class == "" {
		return nil
	}

	clients, err := r.hypr.Clients()
	if err != nil {
		return err
	}

	// Prefer the instance on the communication workspace; fall back to any match.
	var fallback *hypr.Window
	for i := range clients {
		if !strings.EqualFold(clients[i].Class, target.Class) {
			continue
		}
		if clients[i].Workspace.ID == notifyFocusWorkspace {
			return r.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", clients[i].Address))
		}
		if fallback == nil {
			fallback = &clients[i]
		}
	}
	if fallback != nil {
		return r.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", fallback.Address))
	}
	return nil
}
