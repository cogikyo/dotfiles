package session

// picker.go drives the interactive eww session picker and applies selected workspace/session targets.

import (
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Picker manages the layout-picker overlay driven by Hyprland submap keys and rendered by eww.
type Picker struct {
	hypr  *hypr.Client
	state *state.State

	mu        sync.Mutex
	active    bool
	ws        int              // workspace cursor (2–5)
	si        int              // session index within cache[ws]
	selecting bool             // true once the user presses down to enter session selection
	confirmed bool             // true during the brief green-flash after confirm
	cache     map[int][]string // ws → sorted session names
}

type pickerPayload struct {
	WS        int             `json:"ws"`
	Occupied  string          `json:"occupied"`
	Sessions  []pickerSession `json:"sessions"`
	Count     int             `json:"count"`
	Confirmed bool            `json:"confirmed"`
}

type pickerSession struct {
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
	Active   bool   `json:"active"`
}

func NewPicker(h *hypr.Client, s *state.State) *Picker {
	return &Picker{hypr: h, state: s}
}

func (p *Picker) Execute(arg string) (string, error) {
	switch strings.TrimSpace(arg) {
	case "open":
		return p.open()
	case "close":
		return p.close()
	case "left":
		return p.move(-1, 0)
	case "right":
		return p.move(1, 0)
	case "up":
		return p.move(0, -1)
	case "down":
		return p.move(0, 1)
	case "confirm":
		return p.confirm()
	default:
		if rest, ok := strings.CutPrefix(arg, "ws "); ok {
			return p.jumpWS(rest)
		}
		return "", fmt.Errorf("usage: picker [open|close|left|right|up|down|confirm|ws <n>]")
	}
}

func (p *Picker) open() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.active {
		return "picker: already open", nil
	}

	cfg := p.state.GetConfig()
	p.cache = make(map[int][]string)
	for name, s := range cfg.Sessions {
		if s.Workspace == 1 {
			continue
		}
		p.cache[s.Workspace] = append(p.cache[s.Workspace], name)
	}
	for ws := range p.cache {
		sort.Strings(p.cache[ws])
	}

	p.ws = p.state.GetWorkspace()
	if p.ws < 2 || p.ws > 5 {
		p.ws = 2
	}
	p.si = p.activeIndex(p.ws)
	p.selecting = false
	p.confirmed = false
	p.active = true

	p.hypr.Dispatch("submap picker")
	p.pushState()
	exec.Command("eww", "open", "picker").Run()
	exec.Command("eww", "update", "picker-visible=true").Run()

	return "picker: opened", nil
}

func (p *Picker) close() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked(), nil
}

func (p *Picker) closeLocked() string {
	if !p.active {
		return "picker: not open"
	}
	p.active = false
	p.hypr.Dispatch("submap reset")
	exec.Command("eww", "update", "picker-visible=false").Run()
	go func() {
		time.Sleep(200 * time.Millisecond)
		exec.Command("eww", "close", "picker").Run()
	}()
	return "picker: closed"
}

func (p *Picker) move(dws, dsi int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return "picker: not open", nil
	}

	if dws != 0 {
		p.ws += dws
		if p.ws < 2 {
			p.ws = 5
		} else if p.ws > 5 {
			p.ws = 2
		}
		p.selecting = false
		p.si = p.activeIndex(p.ws)
	}

	if dsi != 0 {
		if !p.selecting {
			if dsi > 0 {
				p.selecting = true
				p.si = p.activeIndex(p.ws)
			}
		} else {
			sessions := p.cache[p.ws]
			if len(sessions) > 0 {
				p.si += dsi
				if p.si < 0 {
					p.selecting = false
					p.si = p.activeIndex(p.ws)
				} else if p.si >= len(sessions) {
					p.si = len(sessions) - 1
				}
			}
		}
	}

	p.pushState()
	return fmt.Sprintf("picker: ws%d si%d selecting=%v", p.ws, p.si, p.selecting), nil
}

func (p *Picker) jumpWS(arg string) (string, error) {
	ws, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || ws < 2 || ws > 5 {
		return "", fmt.Errorf("invalid workspace: %s", arg)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return "picker: not open", nil
	}

	p.ws = ws
	p.selecting = false
	p.si = p.activeIndex(ws)
	p.pushState()
	return fmt.Sprintf("picker: jumped to ws%d", ws), nil
}

func (p *Picker) confirm() (string, error) {
	p.mu.Lock()

	if !p.active {
		p.mu.Unlock()
		return "picker: not open", nil
	}

	sessions := p.cache[p.ws]
	if len(sessions) == 0 {
		p.mu.Unlock()
		return "picker: no sessions for ws" + strconv.Itoa(p.ws), nil
	}

	if !p.selecting {
		p.si = p.activeIndex(p.ws)
		p.selecting = true
	}

	name := sessions[p.si]
	ws := p.ws

	p.confirmed = true
	p.pushState()
	p.active = false
	p.mu.Unlock()

	go func() {
		time.Sleep(350 * time.Millisecond)
		p.hypr.Dispatch("submap reset")
		exec.Command("eww", "update", "picker-visible=false").Run()
		time.Sleep(200 * time.Millisecond)
		exec.Command("eww", "close", "picker").Run()

		layout := NewLayout(p.hypr, p.state)
		layout.Execute(fmt.Sprintf("set %d %s", ws, name))
		layout.Execute(strconv.Itoa(ws))
	}()

	return fmt.Sprintf("picker: confirmed %s on ws%d", name, ws), nil
}

func (p *Picker) activeIndex(ws int) int {
	active := p.activeSession(ws)
	for i, name := range p.cache[ws] {
		if name == active {
			return i
		}
	}
	return 0
}

func (p *Picker) pushState() {
	sessions := p.cache[p.ws]
	active := p.activeSession(p.ws)

	items := make([]pickerSession, len(sessions))
	for i, name := range sessions {
		items[i] = pickerSession{
			Name:     name,
			Selected: p.selecting && i == p.si,
			Active:   name == active,
		}
	}

	occupied := p.state.GetOccupied()
	occParts := make([]string, len(occupied))
	for i, id := range occupied {
		occParts[i] = strconv.Itoa(id)
	}

	payload := pickerPayload{
		WS:        p.ws,
		Occupied:  strings.Join(occParts, " "),
		Sessions:  items,
		Count:     len(items),
		Confirmed: p.confirmed,
	}

	data, _ := json.Marshal(payload)
	exec.Command("eww", "update", fmt.Sprintf("picker-data=%s", data)).Run()
}

func (p *Picker) activeSession(ws int) string {
	for _, occupied := range p.state.GetOccupied() {
		if occupied == ws {
			return p.state.GetActiveSession(ws)
		}
	}
	return ""
}
