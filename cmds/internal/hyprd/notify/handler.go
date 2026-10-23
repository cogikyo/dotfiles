// Package notify handles hyprd notification intake, normalization, styling, delivery, routing, and activation.
// It owns the source-specific event dispatch pipeline and notification activation behavior.
package notify

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/detach"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	dunstDeliveryTTL   = 30 * time.Minute
	dunstDeliveryLimit = 1024
)

var opencodePaneAckGroups = []string{
	"start",
	"complete",
	"lane",
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

var globalDunstDeliveries = &dunstDeliveryRegistry{
	byID: make(map[int]dunstDelivery),
}

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

type dunstDeliveryContent struct {
	App          string
	Category     string
	DesktopEntry string
	Summary      string
	Body         string
	Urgency      string
	IconPath     string
}

type dunstDelivery struct {
	Content dunstDeliveryContent
	Seen    time.Time
}

type dunstDeliveryRegistry struct {
	mu   sync.Mutex
	byID map[int]dunstDelivery
}

func init() {
	go soundWorker()
}

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
		n.acknowledgePane(ctx)
		return nil
	}

	ctx := n.resolveContext(req, "")
	if !hasOpencodeContext(ctx) {
		return nil
	}

	var title string
	switch req.Event {
	case "start":
		title = preferredSummary(req.Message, "Working", 80)
	case "complete":
		title = preferredSummary(req.LastAssistantMessage, "Jobs done", 80)
	case "lane":
		title = preferredSummary(req.Message, "Lane done", 80)
	case "idle":
		title = preferredSummary(req.Message, "Waiting for input", 80)
	case "permission":
		title = preferredSummary(req.Message, "Permission needed", 80)
	case "question":
		title = preferredSummary(req.Message, "Question asked", 80)
	case "error":
		title = preferredSummary(req.Message, "Session error", 80)
	default:
		return fmt.Errorf("unknown opencode event: %s", req.Event)
	}
	return n.dispatch(notificationSpec{
		App:          ctx.App,
		DesktopEntry: "opencode",
		Title:        title,
		Style:        req.Event,
		FocusAction:  true,
		Birth:        req.birth,
	}, ctx)
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
		if !globalDunstDeliveries.Accept(req) {
			return nil
		}
		n.rememberDunstNotification(req)
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

// Accept reports whether this positive notification ID carries new content.
func (r *dunstDeliveryRegistry) Accept(req NotifyRequest) bool {
	if req.NotificationID <= 0 {
		return true
	}

	now := time.Now()
	content := dunstDeliveryContent{
		App:          req.App,
		Category:     req.Category,
		DesktopEntry: req.DesktopEntry,
		Summary:      req.Summary,
		Body:         req.Body,
		Urgency:      req.Urgency,
		IconPath:     req.IconPath,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := now.Add(-dunstDeliveryTTL)
	for id, delivery := range r.byID {
		if delivery.Seen.Before(cutoff) {
			delete(r.byID, id)
		}
	}

	if delivery, seen := r.byID[req.NotificationID]; seen && delivery.Content == content {
		delivery.Seen = now
		r.byID[req.NotificationID] = delivery
		return false
	}

	r.byID[req.NotificationID] = dunstDelivery{Content: content, Seen: now}
	if len(r.byID) > dunstDeliveryLimit {
		oldestID := 0
		var oldest time.Time
		for id, delivery := range r.byID {
			if id == req.NotificationID {
				continue
			}
			if oldestID == 0 || delivery.Seen.Before(oldest) {
				oldestID = id
				oldest = delivery.Seen
			}
		}
		delete(r.byID, oldestID)
	}
	return true
}

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
		return detach.Start(exec.Command("dunstify", args...))
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
			if _, ok := errors.AsType[*exec.ExitError](err); !ok {
				return fmt.Errorf("dunstify: %w", err)
			}
		}

		action := strings.TrimSpace(string(out))
		if action == "focus" {
			n.state.LockLayout()
			n.focusContext(ctx)
			n.state.UnlockLayout()
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

// Prompt blocks until the persistent critical notification closes and reports whether its action was chosen.
func (n *Notifier) Prompt(app, title, body, label string) (bool, error) {
	out, err := exec.Command("dunstify", n.PromptArgs(app, "critical", title, body, label)...).Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return false, fmt.Errorf("dunstify: %w", err)
		}
	}
	return strings.TrimSpace(string(out)) == "default", nil
}

func (n *Notifier) PromptArgs(app, urgency, title, body, label string) []string {
	spec := notificationSpec{
		App:     app,
		Title:   title,
		Body:    body,
		Action:  label,
		Tag:     app,
		Urgency: &urgency,
		Timeout: new(0),
	}
	return n.buildDunstArgs(spec, nil, n.style(""))
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

	entry := spec.DesktopEntry
	if entry == "" {
		entry = "hyprd"
	}

	args := []string{
		"-a", app,
		"-u", urgency,
		"-t", strconv.Itoa(timeout),
		"-h", "string:category:hyprd",
		"-h", "string:desktop-entry:" + entry,
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
	if icon := n.workspaceIconPath(ctx, style.Foreground); icon != "" {
		args = append(args, "-I", icon)
	} else if spec.IconPath != "" {
		args = append(args, "-I", spec.IconPath)
	}
	if spec.FocusAction && paneNotificationID(ctx) > 0 {
		args = append(args, "-A", "focus,Focus")
	}
	if spec.Action != "" {
		args = append(args, "-A", "default,"+spec.Action)
	}
	if spec.Tag != "" {
		args = append(args, "-h", "string:x-dunst-stack-tag:"+spec.Tag)
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

// NewestActive returns the pane context owned by the newest uncanceled dunstify call.
func (r *paneNotificationRegistry) NewestActive() *kittyContext {
	r.mu.Lock()
	defer r.mu.Unlock()

	var newest paneNotificationKey
	var newestToken uint64
	for key, token := range r.active {
		_, canceled := r.canceled[key][token]
		if token == 0 || token <= r.ack[key] || canceled {
			continue
		}
		if token > newestToken {
			newest = key
			newestToken = token
		}
	}
	if newestToken == 0 {
		return nil
	}
	return &kittyContext{PID: newest.PID, WindowID: newest.WindowID}
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
	return slices.Contains(opencodePaneAckGroups, group)
}

func closeDunstNotification(id int) {
	if id <= 0 {
		return
	}
	_ = detach.Start(exec.Command("dunstctl", "close", strconv.Itoa(id)))
}

// fromHyprd reports whether hyprd sent this notification itself.
//
// dunst reports every notification back through the script hook, hyprd's own included.
func fromHyprd(req NotifyRequest) bool {
	return req.Category == "hyprd" || req.DesktopEntry == "hyprd"
}

// soundForDunst picks a sound for a dunst script event.
//
// Skip hyprd's own notifications to avoid double-sounding through the dunst script callback loop.
func (n *Notifier) soundForDunst(req NotifyRequest) string {
	if fromHyprd(req) {
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
