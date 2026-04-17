package notify

import (
	"dotfiles/daemons/daemon"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ entry point                                                                  │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// CmdNotify parses args, builds a NotifyRequest, and forwards it to the daemon.
//
// Exits non-zero on parse error or when the daemon is down.
// Dunst script events silently no-op when the daemon is down: dunst rules fire regardless of daemon state.
func CmdNotify(client *daemon.Client, args []string) {
	req, err := parseNotifyArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !client.IsRunning() {
		if req.Source == "dunst" {
			return
		}
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	data, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if _, err := client.Send("notify " + string(data)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ arg parsing                                                                  │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func parseNotifyArgs(args []string) (NotifyRequest, error) {
	if len(args) == 0 {
		return NotifyRequest{}, fmt.Errorf("usage: hyprd notify {hook|dunst|kitty-finish}")
	}

	switch args[0] {
	case "hook":
		return parseHookNotify(args[1:])
	case "dunst":
		return parseDunstNotify(args[1:])
	case "kitty-finish":
		command := strings.TrimSpace(strings.Join(args[1:], " "))
		return NotifyRequest{
			Source:  "kitty",
			Event:   "cmd-finish",
			Command: limitText(command, 512),
		}, nil
	case "send":
		return parseSendNotify(args[1:])
	default:
		return NotifyRequest{}, fmt.Errorf("unknown notify mode: %s", args[0])
	}
}

// parseHookNotify builds a request from Claude Code or Codex hook invocations.
//
// Payload comes from argv[1] (if JSON-shaped) or stdin.
// KITTY_PID/KITTY_WINDOW_ID are read from env so the daemon can resolve the originating terminal.
func parseHookNotify(args []string) (NotifyRequest, error) {
	if len(args) == 0 {
		return NotifyRequest{}, fmt.Errorf("usage: hyprd notify hook {claude|codex}")
	}

	switch args[0] {
	case "claude":
		if len(args) < 2 {
			return NotifyRequest{}, fmt.Errorf("usage: hyprd notify hook claude <start|subagent|complete|idle|permission>")
		}
		payload := readJSONPayload("")
		return NotifyRequest{
			Source:               "claude",
			Event:                args[1],
			Prompt:               limitText(payloadString(payload, "prompt"), 512),
			Message:              limitText(payloadString(payload, "message"), 512),
			LastAssistantMessage: limitText(payloadString(payload, "last_assistant_message", "last-assistant-message"), 512),
			AgentType:            limitText(payloadString(payload, "agent_type"), 128),
			KittyPID:             envInt("KITTY_PID"),
			KittyWindowID:        envInt("KITTY_WINDOW_ID"),
		}, nil
	case "codex":
		raw := ""
		if len(args) > 1 && looksLikeJSON(args[1]) {
			raw = args[1]
		}
		payload := readJSONPayload(raw)
		event := payloadString(payload, "type")
		if event == "" && len(args) > 1 && !looksLikeJSON(args[1]) {
			event = args[1]
		}
		return NotifyRequest{
			Source:               "codex",
			Event:                limitText(event, 128),
			Message:              limitText(payloadString(payload, "message"), 512),
			LastAssistantMessage: limitText(payloadString(payload, "last_assistant_message", "last-assistant-message"), 512),
			AgentType:            limitText(payloadString(payload, "agent_type"), 128),
			KittyPID:             envInt("KITTY_PID"),
			KittyWindowID:        envInt("KITTY_WINDOW_ID"),
		}, nil
	default:
		return NotifyRequest{}, fmt.Errorf("unknown hook source: %s", args[0])
	}
}

// parseDunstNotify builds a request from a dunst script rule.
// Positional args win; otherwise falls back to DUNST_* env vars injected by dunst.
func parseDunstNotify(args []string) (NotifyRequest, error) {
	event := "script"
	if len(args) > 0 && args[0] == "approval" {
		event = "approval-requested"
		args = args[1:]
	}

	app, summary, body, iconPath, urgency := dunstPayload(args)
	return NotifyRequest{
		Source:   "dunst",
		Event:    event,
		App:      limitText(app, 128),
		Summary:  limitText(summary, 512),
		Body:     limitText(body, 512),
		IconPath: strings.TrimSpace(iconPath),
		Urgency:  limitText(strings.ToLower(strings.TrimSpace(urgency)), 32),
	}, nil
}

func dunstPayload(args []string) (app, summary, body, iconPath, urgency string) {
	if len(args) >= 5 {
		return args[0], args[1], args[2], args[3], args[4]
	}
	return os.Getenv("DUNST_APP_NAME"),
		os.Getenv("DUNST_SUMMARY"),
		os.Getenv("DUNST_BODY"),
		os.Getenv("DUNST_ICON_PATH"),
		os.Getenv("DUNST_URGENCY")
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ payload + env helpers                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// readJSONPayload unmarshals raw (or stdin if raw is empty) into a generic map.
// Returns an empty map on parse error so callers can treat missing keys as absent fields.
func readJSONPayload(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		raw = readOptionalStdin()
	}
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

// readOptionalStdin returns piped stdin contents, or "" when stdin is a tty.
func readOptionalStdin() string {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return ""
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return ""
	}
	return string(data)
}

// payloadString returns the first string-valued key found among keys.
func payloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if s, ok := value.(string); ok {
				return s
			}
		}
	}
	return ""
}

func looksLikeJSON(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[")
}

// parseSendNotify parses a notify-send-style invocation: optional -a/-u/-t flags, then title and optional body.
// Args after title are joined into body.
func parseSendNotify(args []string) (NotifyRequest, error) {
	req := NotifyRequest{Source: "send", Urgency: "normal"}
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-a":
			i++
			if i < len(args) {
				req.App = args[i]
			}
		case "-u":
			i++
			if i < len(args) {
				req.Urgency = args[i]
			}
		case "-t":
			i++
			if i < len(args) {
				if v, err := strconv.Atoi(args[i]); err == nil {
					req.Timeout = v
				}
			}
		default:
			req.Summary = args[i]
			if i+1 < len(args) {
				req.Body = strings.Join(args[i+1:], " ")
			}
			return req, nil
		}
		i++
	}
	if req.Summary == "" {
		return req, fmt.Errorf("usage: hyprd notify send [-a app] [-u urgency] [-t timeout] title [body]")
	}
	return req, nil
}

// limitText trims whitespace and truncates to max runes (not bytes) to avoid splitting multi-byte characters.
func limitText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}
