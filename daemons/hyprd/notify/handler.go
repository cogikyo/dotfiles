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
	"dotfiles/daemons/config"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ router                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Handle dispatches a NotifyRequest to the per-source handler.
func (n *Notifier) Handle(req NotifyRequest) error {
	switch req.Source {
	case "claude":
		return n.handleClaude(req)
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

func (n *Notifier) handleClaude(req NotifyRequest) error {
	ctx := n.resolveContext(req, "claude")

	switch req.Event {
	case "start":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Prompt, req.Message, 80),
			Style:       "start",
			FocusAction: true,
		}, ctx)
	case "subagent":
		title := fmt.Sprintf("%s: %s",
			preferredSummary(req.AgentType, "Agent", 32),
			preferredSummary(req.LastAssistantMessage, "Done", 70),
		)
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       title,
			Style:       "subagent",
			FocusAction: true,
		}, ctx)
	case "complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Jobs done", 80),
			Style:       "complete",
			FocusAction: true,
		}, ctx)
	case "idle":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Waiting for input", 80),
			Style:       "idle",
			FocusAction: true,
		}, ctx)
	case "permission":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Permission needed", 80),
			Style:       "permission",
			FocusAction: true,
		}, ctx)
	default:
		return fmt.Errorf("unknown claude event: %s", req.Event)
	}
}

func (n *Notifier) handleOpencode(req NotifyRequest) error {
	ctx := n.resolveContext(req, "opencode")

	switch req.Event {
	case "start":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Working", 80),
			Style:       "start",
			FocusAction: true,
		}, ctx)
	case "complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Jobs done", 80),
			Style:       "complete",
			FocusAction: true,
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
		}, ctx)
	case "todo-complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Todo complete", 80),
			Style:       "todo-complete",
			FocusAction: true,
		}, ctx)
	case "idle":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Waiting for input", 80),
			Style:       "idle",
			FocusAction: true,
		}, ctx)
	case "permission":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Permission needed", 80),
			Style:       "permission",
			FocusAction: true,
		}, ctx)
	case "question":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Question asked", 80),
			Style:       "question",
			FocusAction: true,
		}, ctx)
	case "error":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Session error", 80),
			Style:       "error",
			FocusAction: true,
		}, ctx)
	default:
		return fmt.Errorf("unknown opencode event: %s", req.Event)
	}
}

// handleKitty skips claude and opencode commands (handled via the richer hook path).
func (n *Notifier) handleKitty(req NotifyRequest) error {
	command := strings.TrimSpace(req.Command)
	if command == "" || strings.HasPrefix(command, "claude") || strings.HasPrefix(command, "opencode") {
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

// dispatch resolves sound/volume from the agent style, plays the sound, and sends the dunst notification.
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

	if !spec.FocusAction || notificationID(ctx) == 0 {
		return runDetached("dunstify", args...)
	}

	const maxPersistentRetries = 600 // ~10 min at 1s cadence
	for i := range maxPersistentRetries + 1 {
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

	args := []string{"-a", app, "-u", urgency, "-t", strconv.Itoa(timeout), "-h", "string:category:hyprd"}
	if !spec.NoReplace {
		if id := notificationID(ctx); id > 0 {
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
	if spec.FocusAction && notificationID(ctx) > 0 {
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
	paVolume := volume * 65536 / 100
	return runDetached("paplay", "--volume="+strconv.Itoa(paVolume), path)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ config lookups                                                               │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// soundForDunst picks a sound for a dunst script event.
//
// Notifications dispatched by hyprd itself carry category "hyprd" — skip those
// to avoid double-sounding through the dunst script callback loop.
func (n *Notifier) soundForDunst(req NotifyRequest) string {
	if req.Category == "hyprd" {
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
