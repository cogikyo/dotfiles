package main

import (
	"dotfiles/cmds/internal/hyprd/hypr"
	"fmt"
	"strings"
	"sync"
)

const (
	defaultActiveBorder = "rgba(f2a170ff)"
	defaultActiveShadow = "rgba(f2a17008)"
	accentShadowAlpha   = "08"
)

type Accent struct {
	hypr *hypr.Client

	mu      sync.Mutex
	color   string
	current accentTarget
}

type accentTarget struct {
	Border string
	Shadow string
}

func NewAccent(hypr *hypr.Client) *Accent {
	return &Accent{
		hypr: hypr,
	}
}

func (a *Accent) Invalidate() {
	a.mu.Lock()
	a.current = accentTarget{}
	a.mu.Unlock()
}

func (a *Accent) Execute(arg string) (string, error) {
	fields := strings.Fields(arg)
	if len(fields) != 1 {
		return "", fmt.Errorf("usage: accent <#rrggbb|rrggbb|reset>")
	}

	color, clear, err := parseAccentColor(fields[0])
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	if clear {
		a.color = ""
	} else {
		a.color = color
	}
	a.mu.Unlock()

	if err := a.Apply(); err != nil {
		return "", err
	}
	return "ok", nil
}

func (a *Accent) Reset() error {
	a.mu.Lock()
	a.color = ""
	a.mu.Unlock()
	return a.Apply()
}

func parseAccentColor(raw string) (string, bool, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "reset" || value == "clear" || value == "default" {
		return "", true, nil
	}

	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return "", false, fmt.Errorf("invalid accent color: %s", raw)
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return "", false, fmt.Errorf("invalid accent color: %s", raw)
		}
	}
	return value, false, nil
}

func (a *Accent) Apply() error {
	target := accentTarget{Border: defaultActiveBorder, Shadow: defaultActiveShadow}
	a.mu.Lock()
	color := a.color
	a.mu.Unlock()
	if color != "" {
		target = accentTarget{
			Border: "rgba(" + color + "ff)",
			Shadow: "rgba(" + color + accentShadowAlpha + ")",
		}
	}

	a.mu.Lock()
	if target == a.current {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	if err := a.hypr.Keyword("general:col.active_border", target.Border); err != nil {
		return err
	}
	if err := a.hypr.Keyword("decoration:shadow:color", target.Shadow); err != nil {
		return err
	}

	a.mu.Lock()
	a.current = target
	a.mu.Unlock()
	return nil
}
