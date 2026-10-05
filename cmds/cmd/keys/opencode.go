package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const leaderText = `"Leader key for keybind combinations"`

var (
	defaultEntry = regexp.MustCompile(`^(\w+|"[^"\\]*"):[\w$]+\(("(?:[^"\\]|\\.)*"|[\w$]+|\{[^{}]*\}),("(?:[^"\\]|\\.)*")\),?`)
	objectKey    = regexp.MustCompile(`key:("(?:[^"\\]|\\.)*")`)
)

type keybind struct {
	action, description string
	value               any
	user                bool
}

func opencode(binary, tui string) App {
	app := App{ID: "opencode", Name: "OpenCode", Binds: []Bind{}}
	data, err := os.ReadFile(binary)
	var defaults []keybind
	if err == nil {
		defaults, err = opencodeDefaults(data)
	}
	if err != nil {
		app.Note = "Defaults unavailable (" + err.Error() + "); showing only tui.json overrides."
	}
	scope := func(string) (string, bool) { return "", false }
	if len(defaults) > 1 {
		if s, err := opencodeScope(data, defaults); err != nil {
			app.Error = "opencode scopes: " + err.Error()
		} else {
			scope = s
		}
	}
	var config struct {
		Keybinds map[string]any `json:"keybinds"`
	}
	if data, err := os.ReadFile(tui); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			app.Error = fmt.Sprintf("%s: %v", tui, err)
			return app
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		app.Error = err.Error()
		return app
	}

	binds := defaults
	for _, action := range slices.Sorted(maps.Keys(config.Keybinds)) {
		i := slices.IndexFunc(binds, func(k keybind) bool { return k.action == action })
		if i < 0 {
			binds = append(binds, keybind{action: action})
			i = len(binds) - 1
		}
		binds[i].value = config.Keybinds[action]
		binds[i].user = true
	}

	var leader []Chord
	if i := slices.IndexFunc(binds, func(k keybind) bool { return k.action == "leader" }); i >= 0 {
		if seqs := opencodeSequences(binds[i].value, nil); len(seqs) > 0 {
			leader = seqs[0]
			app.Groups = []Group{{Prefix: leader, Label: "Leader"}}
		}
	}
	unsited := 0
	for _, k := range binds {
		if k.action == "leader" {
			continue
		}
		label := k.description
		if label == "" {
			label = strings.ReplaceAll(k.action, "_", " ")
		}
		layer, sited := scope(k.action)
		for _, seq := range opencodeSequences(k.value, leader) {
			last := len(seq) - 1
			app.Binds = append(app.Binds, Bind{Prefix: seq[:last], Chord: seq[last], Label: label, Detail: k.action, Builtin: !k.user, Scope: layer})
			if !sited {
				unsited++
			}
		}
	}
	if unsited > 0 && app.Error == "" {
		app.Note = strings.TrimSpace(app.Note + fmt.Sprintf(" %d binds have no keybinding site in the opencode bundle and are shown as global.", unsited))
	}
	return app
}

func opencodeScope(data []byte, defaults []keybind) (func(string) (string, bool), error) {
	commands, err := opencodeCommands(data, defaults[1].action)
	if err != nil {
		return nil, err
	}
	command := func(action string) string {
		if c, ok := commands[action]; ok {
			return c
		}
		return action
	}
	var ids []string
	for _, k := range defaults {
		ids = append(ids, command(k.action))
	}
	layers, err := opencodeScopes(data, ids)
	if err != nil {
		return nil, err
	}
	return func(action string) (string, bool) {
		sites := layers[command(action)]
		if len(sites) == 0 || slices.ContainsFunc(sites, globalLayer) {
			return "", len(sites) > 0
		}
		return sites[0], true
	}, nil
}

func globalLayer(layer string) bool {
	return layer == "app" || layer == "session" || strings.HasPrefix(layer, "app.") || strings.HasPrefix(layer, "app_") || strings.HasPrefix(layer, "session.")
}

func opencodeDefaults(data []byte) ([]keybind, error) {
	at := bytes.Index(data, []byte(leaderText))
	if at < 0 {
		return nil, errors.New("keybind table not found")
	}
	start := bytes.LastIndex(data[max(0, at-256):at], []byte("{leader:"))
	if start < 0 {
		return nil, errors.New("keybind table start not found")
	}
	start += max(0, at-256) + 1
	before := data[max(0, start-4096):start]
	rest := data[start:min(len(data), start+1<<16)]

	var binds []keybind
	for {
		m := defaultEntry.FindSubmatch(rest)
		if m == nil {
			break
		}
		rest = rest[len(m[0]):]
		description, err := strconv.Unquote(string(m[3]))
		if err != nil {
			return nil, fmt.Errorf("%s description: %w", m[1], err)
		}
		value, err := jsValue(string(m[2]), before)
		if err != nil {
			return nil, fmt.Errorf("%s default: %w", m[1], err)
		}
		binds = append(binds, keybind{action: strings.Trim(string(m[1]), `"`), description: description, value: value})
	}
	if len(binds) < 10 || !bytes.HasPrefix(rest, []byte("}")) {
		return nil, fmt.Errorf("keybind table ended early after %d entries", len(binds))
	}
	return binds, nil
}

func jsValue(expr string, before []byte) (any, error) {
	switch {
	case strings.HasPrefix(expr, `"`):
		return strconv.Unquote(expr)
	case strings.HasPrefix(expr, "{"):
		m := objectKey.FindStringSubmatch(expr)
		if m == nil {
			return nil, fmt.Errorf("object without key: %s", expr)
		}
		return strconv.Unquote(m[1])
	}
	m := regexp.MustCompile(`[,\s]`+regexp.QuoteMeta(expr)+`=("(?:[^"\\]|\\.)*")`).FindAllSubmatch(before, -1)
	if m == nil {
		return nil, fmt.Errorf("unresolved identifier %s", expr)
	}
	return strconv.Unquote(string(m[len(m)-1][1]))
}

func opencodeSequences(value any, leader []Chord) [][]Chord {
	var seqs [][]Chord
	switch v := value.(type) {
	case string:
		if v == "none" {
			return nil
		}
		for _, alt := range splitAlternatives(v) {
			if seq, ok := opencodeSequence(alt, leader); ok {
				seqs = append(seqs, seq)
			}
		}
	case []any:
		for _, item := range v {
			seqs = append(seqs, opencodeSequences(item, leader)...)
		}
	case map[string]any:
		if key, ok := v["key"]; ok {
			return opencodeSequences(key, leader)
		}
		name, _ := v["name"].(string)
		var m mods
		for field, bit := range map[string]mods{"ctrl": ctrl, "shift": shift, "meta": alt, "super": super} {
			if on, _ := v[field].(bool); on {
				m |= bit
			}
		}
		if name != "" {
			seqs = append(seqs, []Chord{press(m, name)})
		}
	}
	return seqs
}

func splitAlternatives(s string) []string {
	var alts []string
	var cur strings.Builder
	for _, r := range s {
		if r == ',' && cur.Len() > 0 && !strings.HasSuffix(cur.String(), "+") {
			alts = append(alts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		alts = append(alts, cur.String())
	}
	return alts
}

func opencodeSequence(s string, leader []Chord) ([]Chord, bool) {
	var seq []Chord
	if rest, ok := strings.CutPrefix(s, "<leader>"); ok {
		if leader == nil {
			return nil, false
		}
		seq, s = slices.Clone(leader), rest
	}
	var m mods
	for {
		mod, rest, ok := strings.Cut(s, "+")
		if !ok || rest == "" {
			break
		}
		switch mod {
		case "ctrl":
			m |= ctrl
		case "shift":
			m |= shift
		case "alt", "meta", "option":
			m |= alt
		case "super", "cmd":
			m |= super
		default:
			return nil, false
		}
		s = rest
	}
	if s == "" {
		return nil, false
	}
	return append(seq, press(m, s)), true
}
