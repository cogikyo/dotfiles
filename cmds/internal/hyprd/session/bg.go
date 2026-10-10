package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

type Mode string

const (
	ModeVideo  Mode = "video"
	ModeStatic Mode = "static"
)

// Execute saves the mode and applies it.
func (b *BG) Execute(arg string) (string, error) {
	mode := Mode(arg)
	if mode != ModeVideo && mode != ModeStatic {
		return "", fmt.Errorf("unknown bg mode: %s (static|video)", arg)
	}
	if err := saveMode(mode); err != nil {
		return "", err
	}
	return b.apply(mode)
}

func (b *BG) apply(mode Mode) (string, error) {
	if mode == ModeStatic {
		b.killAll()
		return "bg: static", nil
	}
	return b.ensure()
}

func modePath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("bg mode path: %w", err)
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "hyprd", "bg"), nil
}

func loadMode() (Mode, error) {
	path, err := modePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ModeVideo, nil
	}
	if err != nil {
		return "", fmt.Errorf("read bg mode: %w", err)
	}
	mode := Mode(strings.TrimSpace(string(data)))
	if mode != ModeVideo && mode != ModeStatic {
		return "", fmt.Errorf("bg mode %s: unknown mode %q (static|video)", path, mode)
	}
	return mode, nil
}

func saveMode(mode Mode) error {
	path, err := modePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save bg mode: %w", err)
	}
	if err := os.WriteFile(path, []byte(string(mode)+"\n"), 0o644); err != nil {
		return fmt.Errorf("save bg mode: %w", err)
	}
	return nil
}

func (b *BG) ensure() (string, error) {
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

// EnsureBG applies the saved mode: static stops mpvpaper, and video spawns it if it is not running.
func EnsureBG(cfg *config.BackgroundConfig) error {
	mode, err := loadMode()
	if err != nil {
		return err
	}
	_, err = NewBG(cfg).apply(mode)
	return err
}
