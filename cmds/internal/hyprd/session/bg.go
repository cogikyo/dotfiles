package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
)

const (
	bgStartTimeout = 2 * time.Second
	bgFrameSettle  = 250 * time.Millisecond
)

var errNoActiveMonitors = errors.New("no active hyprland monitors")

// BG manages a single mpvpaper wallpaper process.
type BG struct {
	cfg *config.BackgroundConfig
}

const (
	backgroundCPUs        = "0-6,8-14,16-1023"
	backgroundOOMScoreAdj = "1000"
)

func NewBG(cfg *config.BackgroundConfig) *BG {
	return &BG{cfg: cfg}
}

// Execute runs "ensure" (spawn if dead) or "kill" (pkill all).
func (b *BG) Execute(mode string) (string, error) {
	switch mode {
	case "ensure":
		return b.ensure()
	case "kill":
		b.killAll()
		return "bg: killed", nil
	default:
		return "", fmt.Errorf("unknown bg mode: %s (ensure|kill)", mode)
	}
}

func (b *BG) ensure() (string, error) {
	if !b.cfg.Enabled {
		b.killAll()
		return "bg: disabled", nil
	}
	if b.isAlive() {
		return "bg: running", nil
	}
	b.killAll()
	display, err := b.spawn()
	if err != nil {
		if errors.Is(err, errNoActiveMonitors) {
			return "bg: no active monitors", nil
		}
		return "", err
	}
	if !b.waitAlive() {
		return "", fmt.Errorf("bg: spawned on %s but IPC did not become ready", display)
	}
	time.Sleep(bgFrameSettle)
	return fmt.Sprintf("bg: spawned on %s", display), nil
}

func (b *BG) isAlive() bool {
	return b.request(`["get_property","path"]`) == nil
}

func (b *BG) SetPaused(paused bool) error {
	if !b.cfg.Enabled {
		return nil
	}
	return b.request(fmt.Sprintf(`["set_property","pause",%t]`, paused))
}

func (b *BG) request(command string) error {
	conn, err := net.DialTimeout("unix", b.cfg.Socket, 200*time.Millisecond)
	if err != nil {
		return fmt.Errorf("connect mpvpaper: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return fmt.Errorf("set mpvpaper deadline: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "{\"command\":%s}\n", command); err != nil {
		return fmt.Errorf("send %s: %w", command, err)
	}
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return fmt.Errorf("read %s reply: %w", command, err)
		}
		var reply struct {
			Event string `json:"event"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(line, &reply); err != nil {
			return fmt.Errorf("parse %s reply %q: %w", command, line, err)
		}
		if reply.Event != "" {
			continue
		}
		if reply.Error != "success" {
			return fmt.Errorf("%s: %s", command, reply.Error)
		}
		return nil
	}
}

func (b *BG) waitAlive() bool {
	deadline := time.Now().Add(bgStartTimeout)
	for time.Now().Before(deadline) {
		if b.isAlive() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return b.isAlive()
}

func (b *BG) spawn() (string, error) {
	display, err := b.resolveDisplay()
	if err != nil {
		return "", err
	}
	opts := "--loop --hwdec=auto-safe --input-ipc-server=" + b.cfg.Socket
	cmd := exec.Command("taskset", "-c", backgroundCPUs, "mpvpaper", "-p", "-o", opts, display, config.ExpandPath(b.cfg.Video))
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start mpvpaper: %w", err)
	}
	// Keep mpvpaper ahead of preferred development processes in OOM victim selection.
	scorePath := fmt.Sprintf("/proc/%d/oom_score_adj", cmd.Process.Pid)
	if err := os.WriteFile(scorePath, []byte(backgroundOOMScoreAdj), 0); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", fmt.Errorf("set mpvpaper OOM priority: %w", err)
	}
	go cmd.Wait()
	return display, nil
}

func (b *BG) resolveDisplay() (string, error) {
	display := strings.TrimSpace(b.cfg.Display)
	if display != "" && display != "auto" {
		return display, nil
	}

	data, err := exec.Command("hyprctl", "-j", "monitors").Output()
	if err != nil {
		return "", fmt.Errorf("query hyprland monitors: %w", err)
	}

	var monitors []struct {
		Name     string `json:"name"`
		Focused  bool   `json:"focused"`
		Disabled bool   `json:"disabled"`
	}
	if err := json.Unmarshal(data, &monitors); err != nil {
		return "", fmt.Errorf("parse hyprland monitors: %w", err)
	}

	for _, m := range monitors {
		if m.Focused && !m.Disabled && m.Name != "" {
			return m.Name, nil
		}
	}
	for _, m := range monitors {
		if !m.Disabled && m.Name != "" {
			return m.Name, nil
		}
	}
	return "", errNoActiveMonitors
}

// killAll escalates to SIGKILL because a wedged mpvpaper ignores SIGTERM.
func (b *BG) killAll() {
	exec.Command("pkill", "-x", "mpvpaper").Run()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("pgrep", "-x", "-r", "R,S,D,T", "mpvpaper").Run() != nil {
			// settle so a following spawn does not race socket teardown
			time.Sleep(100 * time.Millisecond)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	exec.Command("pkill", "-KILL", "-x", "mpvpaper").Run()
	time.Sleep(100 * time.Millisecond)
}

// EnsureBG spawns the wallpaper if not already running.
func EnsureBG(cfg *config.BackgroundConfig) error {
	bg := NewBG(cfg)
	_, err := bg.Execute("ensure")
	return err
}
