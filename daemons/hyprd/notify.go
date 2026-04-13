package main

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/commands"
	"dotfiles/daemons/hyprd/hypr"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var codexStartSounds = []string{
	"zug-zug",
	"work-work",
	"okie-dokie",
	"something-need-doing",
}

// NotifyRequest is the compact, daemon-facing notification event model.
type NotifyRequest struct {
	Source               string `json:"source"`
	Event                string `json:"event"`
	App                  string `json:"app,omitempty"`
	Summary              string `json:"summary,omitempty"`
	Body                 string `json:"body,omitempty"`
	Urgency              string `json:"urgency,omitempty"`
	IconPath             string `json:"icon_path,omitempty"`
	Timeout              int    `json:"timeout,omitempty"`
	Command              string `json:"command,omitempty"`
	Prompt               string `json:"prompt,omitempty"`
	Message              string `json:"message,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
	AgentType            string `json:"agent_type,omitempty"`
	KittyPID             int    `json:"kitty_pid,omitempty"`
	KittyWindowID        int    `json:"kitty_window_id,omitempty"`
}

type notificationSpec struct {
	App         string
	Title       string
	Body        string
	IconPath    string
	Style       string
	Sound       string
	Volume      int
	Delay       time.Duration
	FocusAction bool
	Urgency     *string
	Timeout     *int
	Persistent  *bool
	IconSuffix  *string
}

type kittyContext struct {
	PID         int
	WindowID    int
	WorkspaceID int
	App         string
}

// Notifier routes compact notification requests through Dunst and paplay.
type Notifier struct {
	hypr  *hypr.Client
	state *State
	cfg   *config.HyprConfig
}

func NewNotifier(h *hypr.Client, s *State, cfg *config.HyprConfig) *Notifier {
	return &Notifier{hypr: h, state: s, cfg: cfg}
}

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
			Timeout:     ptr(4000),
			FocusAction: true,
		}, ctx)
	case "agent-turn-complete":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Jobs done", 80),
			Style:       "complete",
			Sound:       "jobs-done",
			Volume:      n.cfg.Notify.LoudVolume,
			Urgency:     ptr("low"),
			Persistent:  ptr(false),
			FocusAction: true,
		}, ctx)
	case "idle":
		return n.dispatch(notificationSpec{
			App:         ctx.App,
			Title:       preferredSummary(req.LastAssistantMessage, "Waiting for input", 80),
			Style:       "idle",
			Sound:       "hey-listen",
			Volume:      n.cfg.Notify.QuietVolume,
			Urgency:     ptr("low"),
			Persistent:  ptr(false),
			IconSuffix:  ptr(""),
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

func (n *Notifier) handleKitty(req NotifyRequest) error {
	command := strings.TrimSpace(req.Command)
	if command == "" || strings.HasPrefix(command, "claude") {
		return nil
	}

	return n.dispatch(notificationSpec{
		App:     "kitty",
		Title:   " Finished",
		Body:    command,
		Timeout: ptr(5000),
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

	const maxPersistentRetries = 600 // ~10 minutes at 1s intervals
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

func (n *Notifier) resolveContext(req NotifyRequest, fallbackApp string) *kittyContext {
	ctx := &kittyContext{
		PID:      req.KittyPID,
		WindowID: req.KittyWindowID,
		App:      strings.TrimSpace(req.App),
	}

	if ctx.PID == 0 && (req.Source == "codex" || req.Source == "claude") {
		ctx.PID = envInt("KITTY_PID")
	}
	if ctx.WindowID == 0 && (req.Source == "codex" || req.Source == "claude") {
		ctx.WindowID = envInt("KITTY_WINDOW_ID")
	}

	if ctx.PID > 0 {
		ctx.WorkspaceID = n.workspaceForPID(ctx.PID)
	}
	if ctx.WorkspaceID == 0 {
		ctx.WorkspaceID = n.activeWorkspaceID()
	}
	if ctx.App == "" && ctx.PID > 0 && ctx.WindowID > 0 {
		ctx.App = n.tabIcon(ctx.PID, ctx.WindowID)
	}
	if ctx.App == "" {
		ctx.App = fallbackApp
	}
	return ctx
}

func (n *Notifier) workspaceForPID(pid int) int {
	clients, err := n.hypr.Clients()
	if err != nil {
		return 0
	}
	for _, c := range clients {
		if c.Pid == pid {
			return c.Workspace.ID
		}
	}
	return 0
}

func (n *Notifier) activeWorkspaceID() int {
	data, err := n.hypr.Request("j/activeworkspace")
	if err != nil {
		return 0
	}
	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &ws); err != nil {
		return 0
	}
	return ws.ID
}

func (n *Notifier) tabIcon(pid, windowID int) string {
	client := commands.NewKittyClient(pid)
	windows, err := client.FullState()
	if err != nil {
		return ""
	}
	for _, win := range windows {
		for _, tab := range win.Tabs {
			for _, pane := range tab.Windows {
				if pane.ID == windowID {
					return tabIcon(tab.Title)
				}
			}
		}
	}
	return ""
}

func (n *Notifier) findKittyContext(processes []string) *kittyContext {
	matches := func(cmdline []string) bool {
		for _, part := range cmdline {
			lower := strings.ToLower(part)
			for _, process := range processes {
				if process != "" && strings.Contains(lower, strings.ToLower(process)) {
					return true
				}
			}
		}
		return false
	}

	paths, err := filepath.Glob("/tmp/kitty-*")
	if err != nil {
		return nil
	}

	var best *kittyContext
	bestScore := -1

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(path), "kitty-"))
		if err != nil {
			continue
		}

		client := commands.NewKittyClient(pid)
		windows, err := client.FullState()
		if err != nil {
			continue
		}

		for _, win := range windows {
			for _, tab := range win.Tabs {
				for _, pane := range tab.Windows {
					for _, proc := range pane.ForegroundProcesses {
						if matches(proc.Cmdline) {
							goto matched
						}
					}
					continue
				matched:
					score := 0
					if tab.IsFocused {
						score++
					}
					if pane.IsFocused {
						score += 2
					}
					if score > bestScore {
						bestScore = score
						best = &kittyContext{
							PID:         pid,
							WindowID:    pane.ID,
							WorkspaceID: n.workspaceForPID(pid),
							App:         tabIcon(tab.Title),
						}
					}
				}
			}
		}
	}

	return best
}

func (n *Notifier) workspaceIconPath(ctx *kittyContext, suffix string) string {
	if ctx == nil {
		return ""
	}
	name := n.cfg.Notify.WorkspaceIcons[ctx.WorkspaceID]
	if name == "" {
		return ""
	}

	path := filepath.Join(config.ExpandPath(n.cfg.Notify.WorkspaceIconsDir), name+suffix+".svg")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func (n *Notifier) focusContext(ctx *kittyContext) {
	if ctx == nil {
		return
	}

	if ctx.PID > 0 {
		if clients, err := n.hypr.Clients(); err == nil {
			for _, c := range clients {
				if c.Pid == ctx.PID {
					_ = n.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
					break
				}
			}
		}
	}

	if ctx.PID > 0 && ctx.WindowID > 0 {
		_ = commands.NewKittyClient(ctx.PID).FocusWindow(ctx.WindowID)
	}
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

func preferredSummary(primary, fallback string, max int) string {
	text := sanitizeLine(primary)
	if text == "" {
		text = sanitizeLine(fallback)
	}
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > max {
		return string(runes[:max])
	}
	return text
}

func sanitizeLine(input string) string {
	for line := range strings.SplitSeq(input, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#")
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "`", "")
		if line != "" {
			return line
		}
	}
	return ""
}

func tabIcon(title string) string {
	fields := strings.Fields(title)
	if len(fields) == 3 {
		return fields[1]
	}
	return strings.TrimSpace(title)
}

func pickSound(options []string) string {
	if len(options) == 0 {
		return ""
	}
	return options[time.Now().UnixNano()%int64(len(options))]
}

func runDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func envInt(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	n, _ := strconv.Atoi(value)
	return n
}

func ptr[T any](v T) *T {
	return &v
}
