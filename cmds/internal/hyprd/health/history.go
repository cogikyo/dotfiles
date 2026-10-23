package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	interval  = time.Minute
	Window    = time.Hour
	maxGap    = 3 * interval
	memoryMin = 128 << 20
)

type history struct {
	Taken     int64              `json:"taken"`
	Evaluated int64              `json:"evaluated"`
	Ticks     map[string]uint64  `json:"ticks"`
	Restarts  map[string]int     `json:"restarts"`
	Cursor    string             `json:"cursor"`
	Zombies   map[string]zombie  `json:"zombies"`
	Memory    map[string]*series `json:"memory"`
	Minutes   []minute           `json:"minutes"`
}

type zombie struct {
	PPID  int   `json:"ppid"`
	Since int64 `json:"since"`
}

type series struct {
	Pid    int     `json:"pid"`
	Exe    string  `json:"exe"`
	Points []point `json:"points"`
}

type point struct {
	At  int64  `json:"at"`
	RSS uint64 `json:"rss"`
}

type minute struct {
	At       int64             `json:"at"`
	Secs     int64             `json:"secs"`
	CPU      map[string]uint64 `json:"cpu,omitempty"`
	Restarts map[string]int    `json:"restarts,omitempty"`
	Lines    map[string]int    `json:"lines,omitempty"`
}

func loadHistory(path string) (*history, error) {
	h := &history{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return h, nil
	}
	if err != nil {
		return h, err
	}
	if err := json.Unmarshal(data, h); err != nil {
		return &history{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return h, nil
}

func (h *history) save(path string) error {
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (h *history) sample(ctx context.Context, now time.Time) ([]proc, []error) {
	at := now.Unix()
	rebase := h.Taken == 0 || now.Sub(time.Unix(h.Taken, 0)) > maxGap
	m := minute{At: at}
	if !rebase {
		m.Secs = at - h.Taken
	}

	var errs []error
	procs, err := readProcs()
	if err != nil {
		return nil, []error{fmt.Errorf("read processes: %w", err)}
	}
	h.sampleProcs(procs, &m, rebase, at)

	restarts := map[string]int{}
	for _, scope := range scopes {
		counts, err := restartCounts(ctx, scope)
		if err != nil {
			errs = append(errs, err)
			for key, n := range h.Restarts {
				if strings.HasPrefix(key, scope+":") {
					restarts[key] = n
				}
			}
			continue
		}
		for unit, n := range counts {
			prev, seen := h.Restarts[unit]
			delta := n
			switch {
			case rebase:
				delta = 0
			case seen && n >= prev:
				delta = n - prev
			}
			if delta > 0 {
				if m.Restarts == nil {
					m.Restarts = map[string]int{}
				}
				m.Restarts[unit] = delta
			}
			restarts[unit] = n
		}
	}
	h.Restarts = restarts

	cursor := h.Cursor
	if rebase {
		cursor = ""
	}
	lines, next, err := journalLines(ctx, cursor)
	if err != nil {
		errs = append(errs, err)
	}
	if len(lines) > 0 {
		m.Lines = lines
	}
	h.Cursor = next

	h.Minutes = append(h.Minutes, m)
	cut := at - int64(Window/time.Second)
	for len(h.Minutes) > 0 && h.Minutes[0].At <= cut {
		h.Minutes = h.Minutes[1:]
	}
	h.Taken = at
	return procs, errs
}

func (h *history) sampleProcs(procs []proc, m *minute, rebase bool, at int64) {
	ticks := make(map[string]uint64, len(procs))
	cpu := map[string]uint64{}
	zombies := map[string]zombie{}
	memory := map[string]*series{}
	cut := at - int64(Window/time.Second)

	for i := range procs {
		p := &procs[i]
		ticks[p.key] = p.ticks
		if p.state == 'Z' {
			z, ok := h.Zombies[p.key]
			if !ok || z.PPID != p.ppid {
				z = zombie{PPID: p.ppid, Since: at}
			}
			zombies[p.key] = z
			continue
		}

		if !rebase {
			prev, seen := h.Ticks[p.key]
			var delta uint64
			switch {
			case seen && p.ticks >= prev:
				delta = p.ticks - prev
			case !seen && p.start >= h.Taken:
				delta = p.ticks
			}
			if delta > 0 {
				cpu[p.resolveGroup()] += delta
			}
		}

		s, tracked := h.Memory[p.key]
		if !tracked && p.rss < memoryMin {
			continue
		}
		if !tracked {
			s = &series{Pid: p.pid, Exe: p.exe()}
		}
		s.Points = append(s.Points, point{At: at, RSS: p.rss})
		for len(s.Points) > 0 && s.Points[0].At <= cut {
			s.Points = s.Points[1:]
		}
		memory[p.key] = s
	}

	floor := uint64(m.Secs * userHZ / 100)
	maps.DeleteFunc(cpu, func(_ string, ticks uint64) bool { return ticks < max(floor, 1) })
	if len(cpu) > 0 {
		m.CPU = cpu
	}
	h.Ticks = ticks
	h.Zombies = zombies
	h.Memory = memory
}

func stateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "hyprd", "health"), nil
}
