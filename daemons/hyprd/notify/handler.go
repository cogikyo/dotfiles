package notify

import (
	"dotfiles/daemons/config"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ router                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Handle dispatches a NotifyRequest to the per-source handler.
//
// Event vocabularies:
//   - claude: start, subagent, complete, idle, permission.
//   - codex: agent-turn-start, agent-turn-complete, idle, approval-requested.
//   - dunst: script, approval-requested.
//   - kitty: cmd-finish.
//   - send: free-form.
func (n *Notifier) Handle(req NotifyRequest) error {
	switch req.Source {
	case "claude":
		return n.handleClaude(req)
	case "codex":
		return n.handleCodex(req)
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

// handleSend forwards a notify-send-style request with no style lookup, sound, or focus action.
func (n *Notifier) handleSend(req NotifyRequest) error {
	urgency := req.Urgency
	if urgency == "" {
		urgency = "normal"
	}

	spec := notificationSpec{
		App:   req.App,
		Title: req.Summary,
		Body:  req.Body,
	}
	if urgency != "" {
		spec.Urgency = &urgency
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
			Sound:       "civ5-worker-select",
			Volume:      n.cfg.Notify.LoudVolume,
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
			Sound:       "sdv-frog",
			Volume:      n.cfg.Notify.QuietVolume,
			FocusAction: true,
		}, ctx)
	case "complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Jobs done", 80),
			Style:       "complete",
			Sound:       "jobs-done",
			Volume:      n.cfg.Notify.LoudVolume,
			FocusAction: true,
		}, ctx)
	case "idle":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Waiting for input", 80),
			Style:       "idle",
			Sound:       "hey-listen",
			Volume:      n.cfg.Notify.QuietVolume,
			FocusAction: true,
		}, ctx)
	case "permission":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.Message, "Permission needed", 80),
			Style:       "permission",
			Sound:       "ssb-ready",
			FocusAction: true,
		}, ctx)
	default:
		return fmt.Errorf("unknown claude event: %s", req.Event)
	}
}

func (n *Notifier) handleCodex(req NotifyRequest) error {
	ctx := n.resolveContext(req, "codex")

	switch req.Event {
	case "agent-turn-start":
		pick := pickSound(codexStartSounds)
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, pick, 80),
			Style:       "start",
			Sound:       pick,
			Volume:      n.cfg.Notify.QuietVolume,
			Timeout:     new(4000),
			FocusAction: true,
		}, ctx)
	case "agent-turn-complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Jobs done", 80),
			Style:       "complete",
			Sound:       "jobs-done",
			Volume:      n.cfg.Notify.LoudVolume,
			Urgency:     new("low"),
			Persistent:  new(false),
			FocusAction: true,
		}, ctx)
	case "idle":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Waiting for input", 80),
			Style:       "idle",
			Sound:       "hey-listen",
			Volume:      n.cfg.Notify.QuietVolume,
			Urgency:     new("low"),
			Persistent:  new(false),
			IconSuffix:  new(""),
			FocusAction: true,
		}, ctx)
	case "approval-requested":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       "Permission needed",
			Style:       "permission",
			Sound:       "hey-listen",
			Volume:      n.cfg.Notify.LoudVolume,
			Delay:       300 * time.Millisecond,
			FocusAction: true,
		}, ctx)
	default:
		return fmt.Errorf("unknown codex event: %s", req.Event)
	}
}

// handleKitty fires after a shell command finishes in a kitty window.
// Claude invocations are skipped — those go through the richer claude hook path.
func (n *Notifier) handleKitty(req NotifyRequest) error {
	command := strings.TrimSpace(req.Command)
	if command == "" || strings.HasPrefix(command, "claude") {
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
	case "approval-requested":
		ctx := n.resolveContext(req, "")
		if ctx.PID == 0 {
			ctx = n.findKittyContext([]string{"codex", "claude"})
		}
		app := req.App
		if app == "" {
			app = "codex"
		}
		if ctx != nil && ctx.App != "" {
			app = ctx.App
		}
		return n.dispatch(notificationSpec{
			App:         app,
			Title:       "Permission needed",
			Style:       "permission",
			Sound:       "hey-listen",
			Volume:      n.cfg.Notify.LoudVolume,
			Delay:       300 * time.Millisecond,
			FocusAction: true,
		}, ctx)
	case "script":
		sound := n.soundForDunst(req.App, req.Summary, req.Body, req.Urgency)
		if sound == "" {
			return nil
		}
		return n.playSound(sound, 0)
	default:
		return fmt.Errorf("unknown dunst event: %s", req.Event)
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ dispatch pipeline                                                            │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// dispatch honors an optional pre-send delay, plays the sound, then sends the dunstify notification.
// Delay debounces rapid paired events (e.g. approval-requested arriving alongside a start event).
func (n *Notifier) dispatch(spec notificationSpec, ctx *kittyContext) error {
	if spec.Delay > 0 {
		time.Sleep(spec.Delay)
	}
	if spec.Sound != "" {
		if err := n.playSound(spec.Sound, spec.Volume); err != nil {
			return err
		}
	}
	return n.sendDunst(spec, ctx)
}

// sendDunst invokes dunstify.
// For focus-action notifications it re-arms on timeout so it stays visible until the user focuses or dismisses it.
// Non-focus specs run detached and return.
func (n *Notifier) sendDunst(spec notificationSpec, ctx *kittyContext) error {
	style := n.style(spec.Style)
	persistent := style.Persistent
	if spec.Persistent != nil {
		persistent = *spec.Persistent
	}
	args := n.buildDunstArgs(spec, ctx, style)

	if !spec.FocusAction || ctx == nil || ctx.WindowID == 0 {
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

// buildDunstArgs assembles the dunstify argv from spec + style + context.
// Non-nil spec fields override style defaults.
// Replace-ID = ctx.WindowID+100000 so repeat notifications for the same pane coalesce.
func (n *Notifier) buildDunstArgs(spec notificationSpec, ctx *kittyContext, style config.NotifyStyle) []string {
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
	if spec.IconSuffix != nil {
		iconSuffix = *spec.IconSuffix
	}

	args := []string{"-a", app, "-u", urgency, "-t", strconv.Itoa(timeout)}
	if ctx != nil && ctx.WindowID > 0 {
		args = append(args, "-r", strconv.Itoa(ctx.WindowID+100000))
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
	if spec.FocusAction && ctx != nil && ctx.WindowID > 0 {
		args = append(args, "-A", "focus,Focus")
	}
	args = append(args, spec.Title, spec.Body)
	return args
}

// playSound runs paplay detached on <SoundsDir>/<name>.ogg.
// "" and "none" mean no sound; volume=0 uses the paplay default.
func (n *Notifier) playSound(name string, volume int) error {
	if name == "" || name == "none" {
		return nil
	}

	path := filepath.Join(config.ExpandPath(n.cfg.Notify.SoundsDir), name+".ogg")
	args := []string{}
	if volume > 0 {
		args = append(args, "--volume="+strconv.Itoa(volume))
	}
	args = append(args, path)
	return runDetached("paplay", args...)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ config lookups                                                               │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// soundForDunst picks a sound for a dunst script event.
// Silent apps and kitty messages matching KittySilentPatterns return "".
// App-specific mappings win over urgency-level mappings.
func (n *Notifier) soundForDunst(app, summary, body, urgency string) string {
	if n.isSilentApp(app) {
		return ""
	}
	if strings.EqualFold(app, "kitty") {
		content := strings.ToLower(summary + " " + body)
		for _, needle := range n.cfg.Notify.KittySilentPatterns {
			if needle != "" && strings.Contains(content, strings.ToLower(needle)) {
				return ""
			}
		}
	}

	sound := n.lookupUrgencySound(urgency)
	if appSound, ok := n.lookupAppSound(app); ok {
		sound = appSound
	}
	if sound == "" || sound == "none" {
		return ""
	}
	return sound
}

func (n *Notifier) style(name string) config.NotifyStyle {
	if name == "" {
		return config.NotifyStyle{}
	}
	if style, ok := n.cfg.Notify.Styles[name]; ok {
		return style
	}
	return config.NotifyStyle{}
}

func (n *Notifier) isSilentApp(app string) bool {
	return slices.Contains(n.cfg.Notify.SilentApps, strings.ToLower(app))
}

func (n *Notifier) lookupUrgencySound(urgency string) string {
	return n.cfg.Notify.UrgencySounds[strings.ToLower(urgency)]
}

func (n *Notifier) lookupAppSound(app string) (string, bool) {
	sound, ok := n.cfg.Notify.AppSounds[strings.ToLower(app)]
	return sound, ok
}
