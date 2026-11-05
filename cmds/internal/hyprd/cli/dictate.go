package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/hyprd/hypr"
)

const (
	pillWidth  = 112
	pillHeight = 26
	caretGap   = 8
)

type rect struct {
	X, Y, W, H float64
}

func (r rect) contains(x, y float64) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func Dictate() {
	hex := hexBinary()
	if at, err := hudAt(); err == nil {
		args := []string{"dictation", "toggle", "--hud-at", fmt.Sprintf("%d,%d", at[0], at[1])}
		if code, stderr := runHex(hex, args); !refusedPin(code, stderr) {
			os.Stderr.Write(stderr)
			os.Exit(code)
		}
	} else {
		fmt.Fprintf(os.Stderr, "hyprd dictate: %v; using default placement\n", err)
	}
	code, stderr := runHex(hex, []string{"dictation", "toggle"})
	os.Stderr.Write(stderr)
	os.Exit(code)
}

func hudAt() ([2]int, error) {
	c, err := hypr.NewClient()
	if err != nil {
		return [2]int{}, err
	}
	monitors, err := c.Monitors()
	if err != nil {
		return [2]int{}, err
	}
	i := slices.IndexFunc(monitors, func(m hypr.Monitor) bool { return m.Focused })
	if i < 0 {
		return [2]int{}, errors.New("no focused monitor")
	}
	m := monitors[i]
	area := usable(m)
	w, h := m.LogicalSize()
	if caret, ok := focusedCaret(c); ok && (rect{float64(m.X), float64(m.Y), w, h}).contains(caret.X+caret.W/2, caret.Y+caret.H/2) {
		return place(caret, area), nil
	}
	data, err := c.Request("j/cursorpos")
	if err != nil {
		return [2]int{}, err
	}
	var pos struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal(data, &pos); err != nil {
		return [2]int{}, fmt.Errorf("parse cursorpos: %w", err)
	}
	return pin(pos.X, pos.Y, area), nil
}

func focusedCaret(c *hypr.Client) (rect, bool) {
	data, err := c.Request("caret")
	if err != nil {
		return rect{}, false
	}
	var reply struct {
		Found bool    `json:"found"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		W     float64 `json:"w"`
		H     float64 `json:"h"`
	}
	if json.Unmarshal(data, &reply) != nil || !reply.Found {
		return rect{}, false
	}
	return rect{reply.X, reply.Y, reply.W, reply.H}, true
}

func usable(m hypr.Monitor) rect {
	w, h := m.Area()
	return rect{float64(m.X + m.Reserved[0]), float64(m.Y + m.Reserved[1]), w, h}
}

func place(caret, area rect) [2]int {
	y := caret.Y - caretGap - pillHeight/2
	if y-pillHeight/2 < area.Y {
		y = caret.Y + caret.H + caretGap + pillHeight/2
	}
	return pin(caret.X+caret.W/2, y, area)
}

func pin(x, y float64, area rect) [2]int {
	x = max(area.X+pillWidth/2, min(x, area.X+area.W-pillWidth/2))
	y = max(area.Y+pillHeight/2, min(y, area.Y+area.H-pillHeight/2))
	return [2]int{int(math.Round(x - area.X)), int(math.Round(y - area.Y))}
}

func hexBinary() string {
	out, err := exec.Command("systemctl", "--user", "show", "--property=MainPID", "--value", "hex.service").Output()
	if err != nil {
		return "hex"
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return "hex"
	}
	exe := fmt.Sprintf("/proc/%d/exe", pid)
	if _, err := os.Stat(exe); err != nil {
		return "hex"
	}
	// Run the live Hex inode, even if an update replaced its on-disk binary.
	return exe
}

func runHex(hex string, args []string) (int, []byte) {
	var stderr bytes.Buffer
	cmd := exec.Command(hex, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, stderr.Bytes()
	case errors.As(err, &exit):
		return exit.ExitCode(), stderr.Bytes()
	default:
		return 1, fmt.Appendf(stderr.Bytes(), "hyprd dictate: %v\n", err)
	}
}

func refusedPin(code int, stderr []byte) bool {
	return code == 2 || code == 1 && bytes.Contains(stderr, []byte("control version"))
}
