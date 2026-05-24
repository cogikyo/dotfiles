// Package notify handles hyprd notification intake, styling, and delivery.
//
// It normalizes source-specific events into a single dispatch pipeline.
//
// Responsibilities:
// - Normalize notification requests from CLI and hook sources.
// - Resolve kitty and workspace context for icons and focus actions.
// - Dispatch dunst notifications and optional sound effects.
package notify

// handler.go routes notify events by source and executes the sound-plus-dunst dispatch pipeline.

import (
	"dotfiles/cmds/internal/config"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	idleBackoffInitial = 10 * time.Minute
	idleBackoffStep    = 10 * time.Minute
	idleBackoffMax     = 30 * time.Minute
)

var opencodePaneAckGroups = []string{
	"start",
	"complete",
	"subagent",
	"todo-complete",
	"idle",
	"permission",
	"question",
	"error",
}

var globalPaneNotifications = &paneNotificationRegistry{
	active:   make(map[paneNotificationKey]uint64),
	canceled: make(map[paneNotificationKey]map[uint64]struct{}),
	ack:      make(map[paneNotificationKey]uint64),
}

var globalIdleGate = struct {
	sync.Mutex
	byKey map[string]idleBackoff
}{byKey: make(map[string]idleBackoff)}

var soundQueue = make(chan soundRequest, 32)

type soundRequest struct {
	path   string
	volume int
}

type paneNotificationKey struct {
	PID      int
	WindowID int
	Group    string
}

type paneNotificationRegistry struct {
	mu       sync.Mutex
	next     uint64
	active   map[paneNotificationKey]uint64
	canceled map[paneNotificationKey]map[uint64]struct{}
	ack      map[paneNotificationKey]uint64
}

func init() {
	go soundWorker()
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ router                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// CanDispatch reports whether a request has enough context for visible delivery.
func (n *Notifier) CanDispatch(req NotifyRequest) bool {
	if req.Source != "opencode" {
		return true
	}
	if req.Event == "viewed" {
		return paneNotificationID(opencodePaneContext(req)) > 0
	}
	return hasOpencodeContext(n.resolveContext(req, ""))
}

// Prepare stamps request-local ordering before asynchronous handling can race a viewed ack.
func (n *Notifier) Prepare(req NotifyRequest) NotifyRequest {
	if req.Source != "opencode" || req.Event == "viewed" {
		return req
	}
	ctx := opencodePaneContext(req)
	if paneNotificationID(ctx) == 0 {
		return req
	}
	if !isPaneAckGroup(req.Event) {
		return req
	}
	req.birth = globalPaneNotifications.Next()
	return req
}

// Handle dispatches a NotifyRequest to the per-source handler.
func (n *Notifier) Handle(req NotifyRequest) error {
	switch req.Source {
	case "opencode":
		return n.handleOpencode(req)
	case "kitty":
		return n.handleKitty(req)
	case "dunst":
		return n.handleDunst(req)
	case "send":
		return n.handleSend(req)
	default:
		return fmt.Errorf("unknown notify source: %s", req.Source)
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ source handlers                                                              │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (n *Notifier) handleSend(req NotifyRequest) error {
	urgency := req.Urgency
	if urgency == "" {
		urgency = "normal"
	}

	spec := notificationSpec{
		App:     req.App,
		Title:   req.Summary,
		Body:    req.Body,
		Urgency: &urgency,
	}
	if req.Timeout > 0 {
		spec.Timeout = &req.Timeout
	}

	return n.dispatch(spec, nil)
}

func (n *Notifier) handleOpencode(req NotifyRequest) error {
	if req.Event == "viewed" {
		ctx := opencodePaneContext(req)
		if paneNotificationID(ctx) == 0 {
			return nil
		}
		n.trackAgentActivity(req, ctx)
		n.acknowledgePane(ctx)
		return nil
	}

	ctx := n.resolveContext(req, "")
	if !hasOpencodeContext(ctx) {
		return nil
	}
	n.trackAgentActivity(req, ctx)

	switch req.Event {
	case "start":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Working", 80),
			Style:       "start",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Jobs done", 80),
			Style:       "complete",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "subagent":
		title := fmt.Sprintf("%s: %s",
			preferredSummary(req.AgentType, "Agent", 32),
			preferredSummary(req.Message, "Done", 70),
		)
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       title,
			Style:       "subagent",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "todo-complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Todo complete", 80),
			Style:       "todo-complete",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "idle":
		if !n.allowIdleNotification(req, ctx) {
			return nil
		}
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Waiting for input", 80),
			Style:       "idle",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "permission":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Permission needed", 80),
			Style:       "permission",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "question":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Question asked", 80),
			Style:       "question",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	case "error":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Session error", 80),
			Style:       "error",
			FocusAction: true,
			Birth:       req.birth,
		}, ctx)
	default:
		return fmt.Errorf("unknown opencode event: %s", req.Event)
	}
}

func opencodePaneContext(req NotifyRequest) *kittyContext {
	return &kittyContext{PID: req.KittyPID, WindowID: req.KittyWindowID}
}

func hasOpencodeContext(ctx *kittyContext) bool {
	if ctx == nil {
		return false
	}
	app := strings.TrimSpace(ctx.App)
	return ctx.PID > 0 && ctx.WindowID > 0 && app != "" && !strings.EqualFold(app, "opencode")
}

// handleKitty skips opencode commands, which are handled via the richer hook path.
func (n *Notifier) handleKitty(req NotifyRequest) error {
	command := strings.TrimSpace(req.Command)
	if command == "" || strings.HasPrefix(command, "opencode") {
		return nil
	}

	return n.dispatch(notificationSpec{
		App:     "kitty",
		Title:   " Finished",
		Body:    command,
		Timeout: new(5000),
	}, nil)
}

func (n *Notifier) handleDunst(req NotifyRequest) error {
	switch req.Event {
	case "script":
		if target, ok := n.actionFocusTargetForDunst(req); ok {
			n.rememberDunstAction(req, target)
		}
		sound := n.soundForDunst(req)
		if sound != "" {
			if err := n.playSound(sound, n.cfg.Notify.DefaultVolume); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown dunst event: %s", req.Event)
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ dispatch pipeline                                                            │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// dispatch resolves sound/volume from the agent style, queues the sound, and sends the dunst notification.
func (n *Notifier) dispatch(spec notificationSpec, ctx *kittyContext) error {
	if spec.Delay > 0 {
		time.Sleep(spec.Delay)
	}

	sound := spec.Sound
	volume := spec.Volume
	if style := n.style(spec.Style); sound == "" {
		sound = style.Sound
		if volume == 0 {
			volume = style.Volume
		}
	}

	if sound != "" {
		if err := n.playSound(sound, volume); err != nil {
			return err
		}
	}
	return n.sendDunst(spec, ctx)
}

// sendDunst invokes dunstify, re-arming focus-action notifications on timeout until dismissed.
func (n *Notifier) sendDunst(spec notificationSpec, ctx *kittyContext) error {
	style := n.style(spec.Style)

	timeout := style.Timeout
	if spec.Timeout != nil {
		timeout = *spec.Timeout
	}
	persistent := timeout == 0

	args := n.buildDunstArgs(spec, ctx, style)

	if !spec.FocusAction || paneNotificationID(ctx) == 0 {
		return runDetached("dunstify", args...)
	}

	key, token, tracked := globalPaneNotifications.Begin(ctx, spec.Style, spec.Birth)
	if tracked {
		defer globalPaneNotifications.End(key, token)
	}

	const maxPersistentRetries = 600 // ~10 min at 1s cadence
	for i := range maxPersistentRetries + 1 {
		if tracked && globalPaneNotifications.Canceled(key, token) {
			return nil
		}

		cmd := exec.Command("dunstify", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				return fmt.Errorf("dunstify: %w", err)
			}
		}

		action := strings.TrimSpace(string(out))
		if action == "focus" {
			n.focusContext(ctx)
			return nil
		}
		if !persistent || i >= maxPersistentRetries {
			return nil
		}
		if tracked && globalPaneNotifications.Canceled(key, token) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return nil
}

// buildDunstArgs assembles the dunstify argv; non-nil spec fields override style defaults.
func (n *Notifier) buildDunstArgs(spec notificationSpec, ctx *kittyContext, style config.ResolvedStyle) []string {
	app := strings.TrimSpace(spec.App)
	if app == "" {
		app = "hyprd"
	}

	urgency := style.Urgency
	if spec.Urgency != nil {
		urgency = *spec.Urgency
	}
	if urgency == "" {
		urgency = "normal"
	}

	timeout := style.Timeout
	if spec.Timeout != nil {
		timeout = *spec.Timeout
	}

	iconSuffix := style.IconSuffix

	args := []string{
		"-a", app,
		"-u", urgency,
		"-t", strconv.Itoa(timeout),
		"-h", "string:category:hyprd",
		"-h", "string:desktop-entry:hyprd",
	}
	if !spec.NoReplace {
		if id := replacementNotificationID(ctx, spec.Style); id > 0 {
			args = append(args, "-r", strconv.Itoa(id))
		}
	}
	if style.Background != "" {
		args = append(args, "-h", "string:bgcolor:"+style.Background)
	}
	if style.Foreground != "" {
		args = append(args, "-h", "string:fgcolor:"+style.Foreground)
	}
	if style.Frame != "" {
		args = append(args, "-h", "string:frcolor:"+style.Frame)
	}
	if icon := n.workspaceIconPath(ctx, iconSuffix); icon != "" {
		args = append(args, "-I", icon)
	} else if spec.IconPath != "" {
		args = append(args, "-I", spec.IconPath)
	}
	if spec.FocusAction && paneNotificationID(ctx) > 0 {
		args = append(args, "-A", "focus,Focus")
	}
	args = append(args, spec.Title, spec.Body)
	return args
}

func (n *Notifier) playSound(name string, volume int) error {
	if name == "" || name == "none" {
		return nil
	}

	path := filepath.Join(config.ExpandPath(soundsDir), name+".ogg")
	if volume == 0 {
		volume = n.cfg.Notify.DefaultVolume
	}
	if volume == 0 {
		volume = 100
	}
	select {
	case soundQueue <- soundRequest{path: path, volume: volume}:
	default:
		go func() { soundQueue <- soundRequest{path: path, volume: volume} }()
	}
	return nil
}

func soundWorker() {
	for req := range soundQueue {
		paVolume := req.volume * 65536 / 100
		_ = exec.Command("paplay", "--volume="+strconv.Itoa(paVolume), req.path).Run()
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ pane acknowledgement                                                         │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (n *Notifier) acknowledgePane(ctx *kittyContext) {
	globalPaneNotifications.Cancel(ctx, opencodePaneAckGroups)
	for _, group := range opencodePaneAckGroups {
		closeDunstNotification(replacementNotificationID(ctx, group))
	}
}

func (r *paneNotificationRegistry) Begin(ctx *kittyContext, group string, birth uint64) (paneNotificationKey, uint64, bool) {
	key, ok := notificationKey(ctx, group)
	if !ok {
		return paneNotificationKey{}, 0, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if birth == 0 {
		birth = r.nextLocked()
	}
	if token, active := r.active[key]; active {
		r.cancelLocked(key, token)
	}
	r.active[key] = birth
	if birth <= r.ack[key] {
		r.cancelLocked(key, birth)
	}
	return key, birth, true
}

func (r *paneNotificationRegistry) Cancel(ctx *kittyContext, groups []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	epoch := r.nextLocked()
	for _, group := range groups {
		key, ok := notificationKey(ctx, group)
		if !ok {
			continue
		}
		r.ack[key] = epoch
		if token, active := r.active[key]; active {
			r.cancelLocked(key, token)
		}
	}
}

func (r *paneNotificationRegistry) Next() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextLocked()
}

func (r *paneNotificationRegistry) Canceled(key paneNotificationKey, token uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, canceled := r.canceled[key][token]
	return token != 0 && canceled
}

func (r *paneNotificationRegistry) End(key paneNotificationKey, token uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active[key] == token {
		delete(r.active, key)
	}
	if tokens := r.canceled[key]; tokens != nil {
		delete(tokens, token)
		if len(tokens) == 0 {
			delete(r.canceled, key)
		}
	}
}

func (r *paneNotificationRegistry) nextLocked() uint64 {
	r.next++
	return r.next
}

func (r *paneNotificationRegistry) cancelLocked(key paneNotificationKey, token uint64) {
	if token == 0 {
		return
	}
	if r.canceled[key] == nil {
		r.canceled[key] = make(map[uint64]struct{})
	}
	r.canceled[key][token] = struct{}{}
}

func notificationKey(ctx *kittyContext, group string) (paneNotificationKey, bool) {
	if paneNotificationID(ctx) == 0 {
		return paneNotificationKey{}, false
	}
	group = strings.TrimSpace(group)
	if group == "" {
		group = "default"
	}
	return paneNotificationKey{PID: ctx.PID, WindowID: ctx.WindowID, Group: group}, true
}

func isPaneAckGroup(group string) bool {
	for _, ackGroup := range opencodePaneAckGroups {
		if group == ackGroup {
			return true
		}
	}
	return false
}

func closeDunstNotification(id int) {
	if id <= 0 {
		return
	}
	_ = runDetached("dunstctl", "close", strconv.Itoa(id))
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ config lookups                                                               │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// soundForDunst picks a sound for a dunst script event.
//
// Notifications dispatched by hyprd itself carry category "hyprd" — skip those
// to avoid double-sounding through the dunst script callback loop.
func (n *Notifier) soundForDunst(req NotifyRequest) string {
	if req.Category == "hyprd" || req.DesktopEntry == "hyprd" {
		return ""
	}

	app := strings.ToLower(req.App)
	if n.isSilentApp(app) {
		return ""
	}
	if strings.EqualFold(app, "kitty") {
		content := strings.ToLower(req.Summary + " " + req.Body)
		for _, needle := range n.cfg.Notify.KittySilentPatterns {
			if needle != "" && strings.Contains(content, needle) {
				return ""
			}
		}
	}

	sound := n.lookupUrgencySound(req.Urgency)
	if appSound, ok := n.lookupAppSound(app); ok {
		sound = appSound
	}
	if sound == "" || sound == "none" {
		return ""
	}
	return sound
}

func (n *Notifier) style(name string) config.ResolvedStyle {
	if name == "" {
		return config.ResolvedStyle{}
	}
	return n.cfg.Notify.ResolveEvent(name)
}

func (n *Notifier) trackAgentActivity(req NotifyRequest, ctx *kittyContext) {
	if req.Event == "idle" {
		return
	}

	globalIdleGate.Lock()
	delete(globalIdleGate.byKey, idleNotificationKey(req, ctx))
	globalIdleGate.Unlock()
}

func (n *Notifier) allowIdleNotification(req NotifyRequest, ctx *kittyContext) bool {
	now := time.Now()
	key := idleNotificationKey(req, ctx)

	globalIdleGate.Lock()
	defer globalIdleGate.Unlock()

	backoff := globalIdleGate.byKey[key]
	if !backoff.NextAllowed.IsZero() && now.Before(backoff.NextAllowed) {
		return false
	}

	interval := backoff.Interval
	if interval == 0 {
		interval = idleBackoffInitial
	} else if interval < idleBackoffMax {
		interval += idleBackoffStep
		if interval > idleBackoffMax {
			interval = idleBackoffMax
		}
	}
	globalIdleGate.byKey[key] = idleBackoff{NextAllowed: now.Add(interval), Interval: interval}
	return true
}

func idleNotificationKey(req NotifyRequest, ctx *kittyContext) string {
	if id := paneNotificationID(ctx); id > 0 {
		return req.Source + ":" + strconv.Itoa(id)
	}
	if ctx != nil && ctx.App != "" {
		return req.Source + ":" + ctx.App
	}
	if req.App != "" {
		return req.Source + ":" + req.App
	}
	return req.Source
}

func (n *Notifier) isSilentApp(app string) bool {
	for _, silent := range n.cfg.Notify.SilentApps {
		if silent != "" && silent == app {
			return true
		}
	}
	return false
}

func (n *Notifier) lookupUrgencySound(urgency string) string {
	return n.cfg.Notify.UrgencySounds[strings.ToLower(urgency)]
}

func (n *Notifier) lookupAppSound(app string) (string, bool) {
	sound, ok := n.cfg.Notify.AppSounds[strings.ToLower(app)]
	return sound, ok
}
