package notify

// types.go defines request/spec/context structs plus the Notifier type shared by notify handlers.

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"time"
)

var agentStartSounds = []string{
	"zug-zug",
	"work-work",
	"okie-dokie",
	"something-need-doing",
}

// NotifyRequest is the daemon-facing notification event, populated per-source by the CLI layer.
type NotifyRequest struct {
	Source               string `json:"source"`
	Event                string `json:"event"`
	NotificationID       int    `json:"notification_id,omitempty"`
	App                  string `json:"app,omitempty"`
	DesktopEntry         string `json:"desktop_entry,omitempty"`
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
	NoReplace   bool
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

// Notifier routes notification requests through dunstify and paplay.
type Notifier struct {
	hypr  *hypr.Client
	state *state.State
	cfg   *config.HyprConfig
}

func NewNotifier(h *hypr.Client, s *state.State, cfg *config.HyprConfig) *Notifier {
	return &Notifier{hypr: h, state: s, cfg: cfg}
}
