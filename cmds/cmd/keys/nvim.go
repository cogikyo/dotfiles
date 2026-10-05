package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

//go:embed nvim.lua
var nvimScript []byte

const nvimNote = "Global normal-mode maps only; buffer-local maps such as LSP attach and nvim-tree buffers are not included."

func nvim() (App, []string) {
	app := App{ID: "nvim", Name: "Neovim", Note: nvimNote, Binds: []Bind{}}
	dir, err := os.MkdirTemp("", "keys-nvim-")
	if err != nil {
		app.Error = err.Error()
		return app, nil
	}
	defer os.RemoveAll(dir)
	script, out := filepath.Join(dir, "keys.lua"), filepath.Join(dir, "out.json")
	if err := os.WriteFile(script, nvimScript, 0o600); err != nil {
		app.Error = err.Error()
		return app, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nvim", "--headless", "-n", "-i", "NONE", "-V1", "--cmd", "luafile "+script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"KEYS_OUT="+out,
		"XDG_STATE_HOME="+filepath.Join(dir, "state"),
		"XDG_CACHE_HOME="+filepath.Join(dir, "cache"),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var dump struct {
		Error string `json:"error"`
		Maps  []struct {
			LHS     string `json:"lhs"`
			RHS     string `json:"rhs"`
			Desc    string `json:"desc"`
			Builtin bool   `json:"builtin"`
		} `json:"maps"`
		Groups []struct {
			LHS   string `json:"lhs"`
			Label string `json:"label"`
		} `json:"groups"`
		GroupsLoaded bool   `json:"groupsLoaded"`
		Leader       string `json:"leader"`
		LocalLeader  string `json:"localleader"`
	}
	data, readErr := os.ReadFile(out)
	switch {
	case readErr != nil && runErr != nil:
		app.Error = fmt.Sprintf("nvim: %v: %s", runErr, tail(stderr.String()))
		return app, nil
	case readErr != nil:
		app.Error = "nvim: no keymap dump: " + readErr.Error()
		return app, nil
	}
	if err := json.Unmarshal(data, &dump); err != nil {
		app.Error = fmt.Sprintf("nvim: unreadable dump of %d bytes: %v", len(data), err)
		return app, nil
	}
	if dump.Error != "" {
		app.Error = "nvim: " + dump.Error
		return app, nil
	}

	for _, m := range dump.Maps {
		if strings.Contains(m.LHS, "<Plug>") || strings.Contains(m.LHS, "<SNR>") {
			continue
		}
		chords := vimKeys(m.LHS)
		if len(chords) == 0 {
			continue
		}
		label := m.Desc
		if label == "" {
			label = rhsLabel(m.RHS)
		}
		last := len(chords) - 1
		app.Binds = append(app.Binds, Bind{Prefix: chords[:last], Chord: chords[last], Label: label, Detail: m.RHS, Builtin: m.Builtin})
	}
	for _, g := range dump.Groups {
		if chords := vimKeys(g.LHS); len(chords) > 0 {
			app.Groups = append(app.Groups, Group{Prefix: chords, Label: g.Label})
		}
	}
	leader := vimKeys(dump.Leader)
	app.Groups = leaderGroup(app.Groups, leader, "Leader")
	if local := vimKeys(dump.LocalLeader); !slices.EqualFunc(local, leader, chordEqual) && slices.ContainsFunc(app.Binds, func(b Bind) bool {
		return len(b.Prefix) >= len(local) && slices.EqualFunc(b.Prefix[:len(local)], local, chordEqual)
	}) {
		app.Groups = leaderGroup(app.Groups, local, "Local leader")
	}
	if !dump.GroupsLoaded {
		app.Note += " Group labels were unavailable: config.keymaps did not return groups."
	}
	return app, nil
}

func leaderGroup(groups []Group, prefix []Chord, label string) []Group {
	if len(prefix) == 0 || slices.ContainsFunc(groups, func(g Group) bool { return slices.EqualFunc(g.Prefix, prefix, chordEqual) }) {
		return groups
	}
	return append(groups, Group{Prefix: prefix, Label: label})
}

func chordEqual(a, b Chord) bool {
	return a.Key == b.Key && slices.Equal(a.Mods, b.Mods)
}

func rhsLabel(rhs string) string {
	name, ok := strings.CutPrefix(rhs, "<Plug>")
	if !ok || name == "" {
		return shortRHS(rhs)
	}
	name = strings.TrimSuffix(strings.TrimPrefix(name, "("), ")")
	var words []string
	for _, field := range strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' }) {
		words = append(words, camelWords(field)...)
	}
	if len(words) == 0 {
		return shortRHS(rhs)
	}
	for i, w := range words {
		if len(w) > 1 {
			words[i] = strings.ToLower(w)
		}
	}
	if r := []rune(words[0]); len(r) > 1 {
		words[0] = strings.ToUpper(string(r[0])) + string(r[1:])
	}
	return strings.Join(words, " ")
}

func camelWords(s string) []string {
	r := []rune(s)
	var words []string
	start := 0
	for i := 1; i < len(r); i++ {
		upper := unicode.IsUpper(r[i])
		lowerBefore := unicode.IsLower(r[i-1])
		acronymEnd := unicode.IsUpper(r[i-1]) && i+1 < len(r) && unicode.IsLower(r[i+1])
		if upper && (lowerBefore || acronymEnd) {
			words = append(words, string(r[start:i]))
			start = i
		}
	}
	return append(words, string(r[start:]))
}

func shortRHS(rhs string) string {
	s := strings.TrimSpace(rhs)
	s = strings.TrimPrefix(s, "<Cmd>")
	s = strings.TrimPrefix(s, ":")
	s = strings.TrimSuffix(s, "<CR>")
	if r := []rune(s); len(r) > 32 {
		s = string(r[:31]) + "…"
	}
	return s
}

func vimKeys(lhs string) []Chord {
	chords := []Chord{}
	for len(lhs) > 0 {
		if lhs[0] == '<' {
			if end := strings.IndexByte(lhs[1:], '>'); end > 0 {
				if c, ok := vimSpecial(lhs[1 : end+1]); ok {
					chords = append(chords, c)
					lhs = lhs[end+2:]
					continue
				}
			}
		}
		r := []rune(lhs)[0]
		chords = append(chords, press(0, string(r)))
		lhs = lhs[len(string(r)):]
	}
	return chords
}

func vimSpecial(token string) (Chord, bool) {
	var m mods
	for len(token) > 2 && token[1] == '-' {
		switch token[0] {
		case 'C', 'c':
			m |= ctrl
		case 'S', 's':
			m |= shift
		case 'M', 'm', 'A', 'a':
			m |= alt
		case 'D', 'd':
			m |= super
		default:
			return Chord{}, false
		}
		token = token[2:]
	}
	if strings.ContainsAny(token, "<> ") || token == "" {
		return Chord{}, false
	}
	if len(token) == 1 && m&ctrl != 0 && m&shift == 0 {
		token = strings.ToLower(token)
	}
	return press(m, token), true
}
