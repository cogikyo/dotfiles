package notify

// Two paths converge here:
//   - dunst emits ActionInvoked when a provider default action fires (mouse click or dunstctl action).
//   - ActivateDisplayed runs from the hyprd keybind and must also serve apps whose notifications carry no provider action at all, by focusing the configured window itself.
//
// A configured app's visible notification arms a route for its target.
// The keybind or a provider action consumes the route, and closing its latest notification removes it.
// Repeated notifications for one target coalesce, while an older notification's close cannot remove the newer route.
//
// Routes are learned from the dunst script hook, because `dunstctl history` only lists notifications dunst has already closed.
// Pending routes live in memory and reset with hyprd.

import (
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/windows"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	appRouteTTL      = 30 * time.Minute       // bound routes whose close signal was missed
	actionAckTTL     = 5 * time.Second        // how long an ActionInvoked record stays interesting
	dunstActionGrace = 150 * time.Millisecond // wait for dunst to report a provider action; the signal precedes dunstctl's reply
	dunstActionPoll  = 15 * time.Millisecond
	dunstObjectPath  = dbus.ObjectPath("/org/freedesktop/Notifications")
	dunstInterface   = "org.freedesktop.Notifications"
)

// focusTarget is the resolved window an activation should focus.
type focusTarget struct {
	Workspace int
	Class     string
	Title     string
}

func (t focusTarget) configured() bool {
	return t.Class != ""
}

// key identifies the route a target owns, so repeated notifications for one app coalesce.
func (t focusTarget) key() string {
	return fmt.Sprintf("%d|%s|%s", t.Workspace, t.Class, t.Title)
}

// appRoute is one configured app whose latest notification remains active.
type appRoute struct {
	Target         focusTarget
	NotificationID uint32
	Notified       time.Time
}

type appRouter struct {
	once   sync.Once
	mu     sync.Mutex
	hypr   *hypr.Client
	routes map[string]appRoute  // target key -> pending route
	live   map[uint32]string    // notification id -> target key, while dunst may still show it
	acted  map[uint32]time.Time // ActionInvoked acknowledgements, scoped per notification id
}

var globalAppRouter = &appRouter{
	routes: make(map[string]appRoute),
	live:   make(map[uint32]string),
	acted:  make(map[uint32]time.Time),
}

// actionFocusTarget resolves the configured window for a notification's app or desktop entry.
//
// Notification app names and desktop entries both appear as config keys because providers supply one, the other, or neither.
func (n *Notifier) actionFocusTarget(app, desktopEntry string) focusTarget {
	for _, key := range []string{app, desktopEntry} {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if target, ok := n.cfg.Notify.ActionFocusApps[key]; ok {
			return focusTarget{Workspace: target.Workspace, Class: target.Class, Title: target.Title}
		}
	}
	return focusTarget{}
}

// rememberDunstNotification arms a pending route when a configured app sends a notification.
//
// hyprd's own notifications come back through the script hook (always_run_script) and carry their own action, so they must never arm a route.
// Notifications suppressed by a dunstrc skip_display rule also reach the hook, but no suppressed rule matches a configured app, so the config gate keeps invisible toasts out of routing.
func (n *Notifier) rememberDunstNotification(req NotifyRequest) {
	if fromHyprd(req) {
		return
	}
	target := n.actionFocusTarget(req.App, req.DesktopEntry)
	if !target.configured() {
		return
	}

	globalAppRouter.Start(n.hypr)
	globalAppRouter.Remember(uint32(max(req.NotificationID, 0)), target)
}

// ActivateDisplayed acts on a visible notification, reporting whether it handled the keypress.
//
// A provider default action always gets first chance, since it belongs to whatever dunst is showing.
// A live pane notification can bridge a momentary zero from dunst; a true zero falls through to the normal keybind.
// When a visible notification has no provider action, the newest app route is consumed and focused.
// Reporting false leaves the keybind free to fall through to its normal behavior.
func (n *Notifier) ActivateDisplayed() (string, bool, error) {
	globalAppRouter.Start(n.hypr)

	displayed := dunstDisplayedCount()
	if displayed > 0 {
		since := time.Now()
		if err := dunstAction(); err != nil {
			return "", true, fmt.Errorf("dunst action: %w", err)
		}
		if id, acted := globalAppRouter.AwaitAction(since, dunstActionGrace); acted {
			logf("activate: provider action id=%d", id)
			return "notification: action", true, nil
		}
	}

	if displayed == 0 {
		if ctx := globalPaneNotifications.NewestActive(); ctx != nil {
			n.acknowledgePane(ctx)
			n.focusContext(ctx)
			logf("activate: focused active pane pid=%d window=%d (displayed=0)", ctx.PID, ctx.WindowID)
			return "notification: focus pane", true, nil
		}
		logf("activate: no displayed notification")
		return "", false, nil
	}

	target, stale, ok := globalAppRouter.Consume()
	if !ok {
		logf("activate: no pending route (displayed=%d)", displayed)
		return "", false, nil
	}
	for _, id := range stale {
		closeDunstNotification(int(id))
	}

	focused, err := globalAppRouter.focus(target)
	if err != nil {
		return "", true, fmt.Errorf("notification focus %s: %w", target.Class, err)
	}
	if focused == "" {
		logf("activate: target %s absent (displayed=%d)", target.Class, displayed)
		return "", false, nil
	}
	logf("activate: focused %s (displayed=%d)", focused, displayed)
	return "notification: focus " + focused, true, nil
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

func (r *appRouter) Start(h *hypr.Client) {
	if h == nil {
		return
	}
	r.once.Do(func() {
		r.hypr = h
		go r.run()
	})
}

// Remember arms or refreshes the route for a target; id is 0 when dunst gave no notification id.
func (r *appRouter) Remember(id uint32, target focusTarget) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pruneLocked()
	key := target.key()
	_, pending := r.routes[key]
	r.routes[key] = appRoute{Target: target, NotificationID: id, Notified: time.Now()}
	if id > 0 {
		r.live[id] = key
	}
	logf("route armed: target=%s id=%d coalesced=%t", target.Class, id, pending)
}

// Consume removes the newest pending route and returns it with the notification ids it owns.
func (r *appRouter) Consume() (focusTarget, []uint32, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pruneLocked()
	var newest appRoute
	var key string
	for candidate, route := range r.routes {
		if key == "" || route.Notified.After(newest.Notified) {
			newest, key = route, candidate
		}
	}
	if key == "" {
		return focusTarget{}, nil, false
	}
	return newest.Target, r.dropLocked(key), true
}

// AwaitAction reports the notification id dunst acted on after since, waiting up to grace.
//
// hyprd cannot know which notification dunst is showing, so any action within the window means a provider consumed the keypress.
// The id is kept so route bookkeeping and logs stay per-notification.
func (r *appRouter) AwaitAction(since time.Time, grace time.Duration) (uint32, bool) {
	deadline := time.Now().Add(grace)
	for {
		r.mu.Lock()
		var acted uint32
		var found bool
		for id, at := range r.acted {
			if at.After(since) {
				acted, found = id, true
				break
			}
		}
		r.mu.Unlock()
		if found {
			return acted, true
		}
		if time.Now().After(deadline) {
			return 0, false
		}
		time.Sleep(dunstActionPoll)
	}
}

// dropLocked removes a route and returns the notification ids that belonged to it.
func (r *appRouter) dropLocked(key string) []uint32 {
	delete(r.routes, key)
	var ids []uint32
	for id, owner := range r.live {
		if owner == key {
			ids = append(ids, id)
			delete(r.live, id)
		}
	}
	slices.Sort(ids)
	return ids
}

func (r *appRouter) pruneLocked() {
	cutoff := time.Now().Add(-appRouteTTL)
	for key, route := range r.routes {
		if route.Notified.Before(cutoff) {
			logf("route expired: target=%s", route.Target.Class)
			r.dropLocked(key)
		}
	}
	for id, owner := range r.live {
		if _, pending := r.routes[owner]; !pending {
			delete(r.live, id)
		}
	}
	ackCutoff := time.Now().Add(-actionAckTTL)
	for id, at := range r.acted {
		if at.Before(ackCutoff) {
			delete(r.acted, id)
		}
	}
}

func (r *appRouter) run() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		logf("dbus monitor setup: connect: %v", err)
		return
	}
	defer conn.Close()

	rules := []string{
		"type='signal',path='/org/freedesktop/Notifications',interface='org.freedesktop.Notifications',member='ActionInvoked'",
		"type='signal',path='/org/freedesktop/Notifications',interface='org.freedesktop.Notifications',member='NotificationClosed'",
	}
	if call := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, uint32(0)); call.Err != nil {
		logf("dbus monitor setup: BecomeMonitor: %v", call.Err)
		return
	}

	messages := make(chan *dbus.Message, 32)
	conn.Eavesdrop(messages)
	for message := range messages {
		if message.Type != dbus.TypeSignal {
			continue
		}
		path, _ := message.Headers[dbus.FieldPath].Value().(dbus.ObjectPath)
		if path != dunstObjectPath {
			continue
		}
		iface, _ := message.Headers[dbus.FieldInterface].Value().(string)
		if iface != dunstInterface {
			continue
		}
		member, _ := message.Headers[dbus.FieldMember].Value().(string)
		switch member {
		case "ActionInvoked":
			r.handleAction(message.Body)
		case "NotificationClosed":
			r.handleClose(message.Body)
		}
	}
}

// handleAction records the acknowledgement and focuses the window when the action belongs to a route.
func (r *appRouter) handleAction(body []any) {
	if len(body) < 2 {
		return
	}
	id, ok := body[0].(uint32)
	if !ok {
		return
	}
	if _, ok := body[1].(string); !ok {
		return
	}

	r.mu.Lock()
	r.acted[id] = time.Now()
	key, routed := r.live[id]
	route, pending := r.routes[key]
	var stale []uint32
	if routed && pending {
		stale = r.dropLocked(key)
	}
	r.mu.Unlock()

	if !routed || !pending {
		return
	}
	for _, staleID := range stale {
		closeDunstNotification(int(staleID))
	}
	focused, err := r.focus(route.Target)
	if err != nil {
		logf("action focus %s (id=%d): %v", route.Target.Class, id, err)
		return
	}
	logf("action focus: id=%d target=%s focused=%q", id, route.Target.Class, focused)
}

// handleClose removes a route when its latest coalesced notification closes for any reason.
func (r *appRouter) handleClose(body []any) {
	if len(body) < 2 {
		return
	}
	id, ok := body[0].(uint32)
	if !ok {
		return
	}
	reason, ok := body[1].(uint32)
	if !ok {
		return
	}

	r.mu.Lock()
	key, routed := r.live[id]
	delete(r.live, id)
	route, pending := r.routes[key]
	closedLatest := routed && pending && route.NotificationID == id
	if closedLatest {
		r.dropLocked(key)
	}
	r.mu.Unlock()

	if closedLatest {
		logf("route closed: id=%d reason=%d", id, reason)
	} else if routed && pending {
		logf("route kept: closed id=%d newer=%d reason=%d", id, route.NotificationID, reason)
	}
}

// focus reports which window or workspace it focused, empty when the target is not on screen anywhere.
func (r *appRouter) focus(target focusTarget) (string, error) {
	if r.hypr == nil || !target.configured() {
		return "", nil
	}

	clients, err := r.hypr.Clients()
	if err != nil {
		return "", err
	}

	var fallback *hypr.Window
	for i := range clients {
		client := &clients[i]
		if !windows.MatchesTarget(client, target.Class, target.Title) {
			continue
		}
		if target.Workspace > 0 && client.Workspace.ID == target.Workspace {
			if err := r.hypr.FocusWindow(client.Address); err != nil {
				return "", err
			}
			return target.Class, nil
		}
		if fallback == nil {
			fallback = client
		}
	}
	if fallback != nil {
		if err := r.hypr.FocusWindow(fallback.Address); err != nil {
			return "", err
		}
		return target.Class, nil
	}
	if target.Workspace > 0 {
		if err := r.hypr.FocusWorkspace(target.Workspace); err != nil {
			return "", err
		}
		return "workspace " + strconv.Itoa(target.Workspace), nil
	}
	return "", nil
}

// logf writes one decision line to the journal, where `journalctl --user -u hyprd` can replay a routing incident.
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "hyprd notify: "+format+"\n", args...)
}
