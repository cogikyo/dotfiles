package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

//go:embed kitty.py
var kittyScript string

type kittyKey struct {
	Mods []string `json:"mods"`
	Key  string   `json:"key"`
}

func (k kittyKey) chord() Chord {
	var m mods
	for _, name := range k.Mods {
		for _, n := range modNames {
			if n.name == name {
				m |= n.bit
			}
		}
	}
	return press(m, k.Key)
}

func kitty() (App, []string) {
	app := App{ID: "kitty", Name: "kitty", Binds: []Bind{}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kitty", "+runpy", `import sys; exec(sys.stdin.read(), {"__name__": "__main__"})`)
	cmd.Stdin = strings.NewReader(kittyScript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		app.Error = fmt.Sprintf("kitty: %v: %s", err, tail(stderr.String()))
		return app, nil
	}
	var dump struct {
		Binds []struct {
			Prefix []kittyKey `json:"prefix"`
			kittyKey
			Label      string     `json:"label"`
			Detail     string     `json:"detail"`
			Builtin    bool       `json:"builtin"`
			ShadowedBy []kittyKey `json:"shadowedBy"`
		} `json:"binds"`
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(out, &dump); err != nil {
		app.Error = fmt.Sprintf("kitty: unreadable dump of %d bytes: %v", len(out), err)
		return app, nil
	}
	for _, b := range dump.Binds {
		app.Binds = append(app.Binds, Bind{
			Prefix:     kittyChords(b.Prefix),
			Chord:      b.chord(),
			Label:      b.Label,
			Detail:     b.Detail,
			Builtin:    b.Builtin,
			ShadowedBy: kittyChords(b.ShadowedBy),
		})
	}
	return app, dump.Files
}

func kittyChords(keys []kittyKey) []Chord {
	chords := make([]Chord, 0, len(keys))
	for _, k := range keys {
		chords = append(chords, k.chord())
	}
	return chords
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if lines := strings.Split(s, "\n"); len(lines) > 3 {
		s = strings.Join(lines[len(lines)-3:], "\n")
	}
	return s
}
