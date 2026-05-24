package notify

// context.go resolves kitty/Hyprland context, workspace icons, and focus actions for notifications.

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/session"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ context resolution                                                           │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (n *Notifier) resolveContext(req NotifyRequest, fallbackApp string) *kittyContext {
	ctx := &kittyContext{
		PID:      req.KittyPID,
		WindowID: req.KittyWindowID,
		App:      strings.TrimSpace(req.App),
	}

	if ctx.PID == 0 && usesKittyEnv(req.Source) {
		ctx.PID = envInt("KITTY_PID")
	}
	if ctx.WindowID == 0 && usesKittyEnv(req.Source) {
		ctx.WindowID = envInt("KITTY_WINDOW_ID")
	}

	if ctx.PID == 0 && usesKittyEnv(req.Source) && allowKittyScanFallback(req.Source) {
		if found := n.findKittyContext([]string{req.Source}); found != nil {
			ctx = found
		}
	}

	if ctx.PID > 0 {
		ctx.WorkspaceID = n.workspaceForPID(ctx.PID)
	}
	if ctx.PID > 0 && ctx.WindowID > 0 {
		ctx.TabID, ctx.App = n.tabContext(ctx.PID, ctx.WindowID)
	}
	if ctx.WorkspaceID == 0 {
		ctx.WorkspaceID = n.activeWorkspaceID()
	}
	if ctx.App == "" {
		ctx.App = fallbackApp
	}
	return ctx
}

func allowKittyScanFallback(source string) bool {
	// OpenCode can have multiple concurrent sessions. Guessing by foreground
	// process can attach a notification to the wrong tab, so require explicit
	// kitty metadata from the plugin for focus actions.
	return source != "opencode"
}

func usesKittyEnv(source string) bool {
	switch source {
	case "opencode":
		return true
	default:
		return false
	}
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

func (n *Notifier) tabContext(pid, windowID int) (string, string) {
	client := session.NewKittyClient(pid)
	windows, err := client.FullState()
	if err != nil {
		return "", ""
	}
	for _, win := range windows {
		for _, tab := range win.Tabs {
			for _, pane := range tab.Windows {
				if pane.ID == windowID {
					tabID := ""
					if pane.Env != nil {
						tabID = pane.Env["KITTY_TAB_ID"]
					}
					return tabID, tabIcon(tab.Title)
				}
			}
		}
	}
	return "", ""
}

// findKittyContext scans kitty control sockets for a foreground process matching any of the given names.
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

		client := session.NewKittyClient(pid)
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
							TabID:       pane.Env["KITTY_TAB_ID"],
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

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ icons + focus actions                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (n *Notifier) workspaceIconPath(ctx *kittyContext, suffix string) string {
	if ctx == nil {
		return ""
	}
	name := workspaceIcons[ctx.WorkspaceID]
	if name == "" {
		return ""
	}

	path := filepath.Join(config.ExpandPath(workspaceIconsDir), name+suffix+".svg")
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

	if ctx.PID > 0 {
		kitty := session.NewKittyClient(ctx.PID)
		if ctx.TabID != "" {
			_ = kitty.FocusTab(ctx.TabID)
		}
		if ctx.WindowID > 0 {
			_ = kitty.FocusWindow(ctx.WindowID)
		}
	}
}

func notificationID(ctx *kittyContext) int {
	if ctx == nil || ctx.PID <= 0 || ctx.WindowID <= 0 {
		return 0
	}
	return 100000 + ctx.PID*1000 + ctx.WindowID
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ text shaping                                                                 │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func preferredSummary(primary, fallback string, max int) string {
	text := sanitizeLine(primary)
	if isSDKSummaryMarker(text) {
		text = ""
	}
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

func isSDKSummaryMarker(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "tool-calls", "stop", "length", "content-filter", "pause", "other", "unknown":
		return true
	default:
		return false
	}
}

// sanitizeLine returns the first non-blank line with markdown noise stripped.
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
