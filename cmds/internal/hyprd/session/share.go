package session

// share.go toggles a presentation-safe desktop state for screen sharing.

import (
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	normalGapsOut = "84,85,22,125"
	shareGapsOut  = "12,16,12,16"
)

// Share owns screen-share mode: quiet notifications, close widgets, stop GLava, and tighten gaps.
type Share struct {
	hypr  *hypr.Client
	state *state.State
	mu    sync.Mutex
}

func NewShare(h *hypr.Client, s *state.State) *Share {
	return &Share{hypr: h, state: s}
}

// Execute toggles screen-share mode by default, with explicit on/off/status verbs for scripts.
func (s *Share) Execute(arg string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch strings.TrimSpace(arg) {
	case "", "toggle":
		if s.active() {
			return s.exit()
		}
		return s.enter()
	case "on", "enable":
		return s.enter()
	case "off", "disable":
		if !s.active() {
			return "share: already off", nil
		}
		return s.exit()
	case "status":
		if s.active() {
			return "share: on", nil
		}
		return "share: off", nil
	default:
		return "", fmt.Errorf("usage: share [toggle|on|off|status]")
	}
}

func (s *Share) enter() (string, error) {
	if err := s.setGaps(shareGapsOut); err != nil {
		return "", err
	}
	s.state.SetScreenShare(true)

	runCommand("dunstctl", "close-all")
	runCommand("dunstctl", "set-paused", "true")
	startDetached("ewwd", "close")
	runCommand("killall", "glava")

	return "share: on", nil
}

func (s *Share) exit() (string, error) {
	if err := s.setGaps(normalGapsOut); err != nil {
		return "", err
	}

	startDetached("ewwd", "restore")
	dispatchGLava(s.hypr)
	time.AfterFunc(time.Second, func() {
		runCommand("dunstctl", "set-paused", "false")
	})

	s.state.SetScreenShare(false)
	return "share: off", nil
}

func (s *Share) active() bool {
	if s.state.GetScreenShare() {
		return true
	}
	resp, err := s.hypr.Request("getoption general:gaps_out")
	if err != nil {
		return false
	}
	return !strings.Contains(string(resp), "84 85 22 125")
}

func (s *Share) setGaps(gaps string) error {
	resp, err := s.hypr.Request("keyword general:gaps_out " + gaps)
	if err != nil {
		return fmt.Errorf("share: set gaps_out: %w", err)
	}
	if string(resp) != "ok" {
		return fmt.Errorf("share: set gaps_out: %s", string(resp))
	}
	return nil
}

func runCommand(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil && len(out) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return err
}
