package session

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/wm"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Share owns screen-share mode: quiet notifications, close widgets, stop GLava, and tighten gaps.
type Share struct {
	hypr    *hypr.Client
	state   *state.State
	gapsOut func() config.GapsOutConfig
	mu      sync.Mutex
}

func NewShare(h *hypr.Client, s *state.State, gapsOut func() config.GapsOutConfig) *Share {
	return &Share{hypr: h, state: s, gapsOut: gapsOut}
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
	if err := s.setGaps(s.gaps().Share); err != nil {
		return "", err
	}
	s.state.SetScreenShare(true)
	splitErr := s.retuneSplit()

	runCommand("dunstctl", "close-all")
	runCommand("dunstctl", "set-paused", "true")
	startDetached("ewwd", "close")
	runCommand("killall", "glava")

	return withSplitErr("share: on", splitErr), nil
}

func (s *Share) exit() (string, error) {
	if err := s.setGaps(s.gaps().Normal); err != nil {
		return "", err
	}
	s.state.SetScreenShare(false)
	splitErr := s.retuneSplit()

	startDetached("ewwd", "restore")
	dispatchGLava(s.hypr)
	time.AfterFunc(time.Second, func() {
		runCommand("dunstctl", "set-paused", "false")
	})

	return withSplitErr("share: off", splitErr), nil
}

// retuneSplit applies the global split preset for the new share mode.
func (s *Share) retuneSplit() error {
	split := wm.NewSplit(s.hypr, s.state)
	if err := split.Reseed(); err != nil {
		return err
	}
	return split.Refresh()
}

func withSplitErr(result string, err error) string {
	if err == nil {
		return result
	}
	return fmt.Sprintf("%s (split: %v)", result, err)
}

func (s *Share) active() bool {
	if s.state.GetScreenShare() {
		return true
	}
	normal := s.gaps().Normal
	resp, err := s.hypr.Request("getoption general:gaps_out")
	if err != nil {
		return false
	}
	return !strings.Contains(string(resp), normal.OptionValue())
}

func (s *Share) gaps() config.GapsOutConfig {
	if s.gapsOut == nil {
		return config.DefaultGapsOutConfig()
	}
	return s.gapsOut().WithDefaults()
}

func (s *Share) setGaps(gaps config.OuterGaps) error {
	var top, right, bottom, left int
	n, err := fmt.Sscanf(gaps.OptionValue(), "%d %d %d %d", &top, &right, &bottom, &left)
	if err != nil {
		return fmt.Errorf("share: parse gaps_out %q: %w", gaps.OptionValue(), err)
	}
	if n != 4 {
		return fmt.Errorf("share: parse gaps_out %q: got %d values", gaps.OptionValue(), n)
	}
	if err := s.hypr.SetOuterGaps(top, right, bottom, left); err != nil {
		return fmt.Errorf("share: set gaps_out: %w", err)
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
