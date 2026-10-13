package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

const leaderText = `"Leader key for keybind combinations"`

var (
	defaultEntry = regexp.MustCompile(`^(\w+|"[^"\\]*"):[\w$]+\(("(?:[^"\\]|\\.)*"|!1|[\w$]+|\{[^{}]*\}),("(?:[^"\\]|\\.)*")\),?`)
	objectKey    = regexp.MustCompile(`key:("(?:[^"\\]|\\.)*")`)
	pluginField  = regexp.MustCompile(`^([A-Za-z_$][\w$]*)\s*:\s*("(?:[^"\\\n]|\\.)*")\s*[,}]`)
)

type keybind struct {
	action, description string
	value               any
	user, plugin        bool
}

func opencode(binary, config, plugins string) App {
	app := App{ID: "opencode", Name: "OpenCode", Binds: []Bind{}}
	data, err := os.ReadFile(binary)
	var defaults []keybind
	if err == nil {
		defaults, err = opencodeDefaults(data)
	}
	if err != nil {
		app.Note = "Defaults unavailable (" + err.Error() + "); showing only cli.json overrides."
	}
	var sites map[string]string
	if len(defaults) > 0 {
		names := make([]string, len(defaults))
		for i, k := range defaults {
			names[i] = k.action
		}
		if sites, err = opencodeSites(data, names); err != nil {
			app.Error = "opencode scopes: " + err.Error()
		}
	}
	var cli struct {
		Keybinds map[string]any `json:"keybinds"`
	}
	if data, err := os.ReadFile(config); err == nil {
		data, err := hujson.Standardize(data)
		if err == nil {
			err = json.Unmarshal(data, &cli)
		}
		if err != nil {
			app.Error = fmt.Sprintf("%s: %v", config, err)
			return app
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		app.Error = err.Error()
		return app
	}

	binds := defaults
	commands, err := opencodePlugins(plugins)
	if err != nil {
		app.Error = "opencode plugins: " + err.Error()
	}
	for _, c := range commands {
		i := slices.IndexFunc(binds, func(k keybind) bool { return k.action == c.action })
		if i < 0 {
			binds = append(binds, c)
			continue
		}
		binds[i] = c
	}
	for _, action := range slices.Sorted(maps.Keys(cli.Keybinds)) {
		i := slices.IndexFunc(binds, func(k keybind) bool { return k.action == action })
		if i < 0 {
			binds = append(binds, keybind{action: action})
			i = len(binds) - 1
		}
		binds[i].value = cli.Keybinds[action]
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
			label = strings.NewReplacer(".", " ", "_", " ").Replace(k.action)
		}
		layer, sited := sites[k.action]
		for _, seq := range opencodeSequences(k.value, leader) {
			last := len(seq) - 1
			app.Binds = append(app.Binds, Bind{Prefix: seq[:last], Chord: seq[last], Label: label, Detail: k.action, Builtin: !k.user, Scope: layer})
			if !sited && !k.plugin {
				unsited++
			}
		}
	}
	if unsited > 0 && app.Error == "" {
		app.Note = strings.TrimSpace(app.Note + fmt.Sprintf(" %d binds have no keybinding site in the opencode bundle and are shown as global.", unsited))
	}
	return app
}

func opencodePlugins(dir string) ([]keybind, error) {
	var binds []keybind
	for _, path := range tree(dir) {
		if ext := filepath.Ext(path); (ext != ".ts" && ext != ".tsx") || strings.Contains(path, "/node_modules/") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return binds, err
		}
		binds = append(binds, pluginCommands(string(src))...)
	}
	return binds, nil
}

func pluginCommands(src string) []keybind {
	var binds []keybind
	ids := map[string]string{}
	objects := []map[string]string{{}}
	prev := byte('{')
scan:
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case strings.HasPrefix(src[i:], "//"):
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				break scan
			}
			i += end
			continue
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				break scan
			}
			i += end + 3
			continue
		case c == '"' || c == '\'' || c == '`':
			i = stringEnd(src, i)
		case c == '{':
			objects = append(objects, map[string]string{})
		case c == '}' && len(objects) > 1:
			top := objects[len(objects)-1]
			objects = objects[:len(objects)-1]
			if top["id"] != "" && top["title"] != "" {
				ids[top["title"]] = top["id"]
			}
			if top["bind"] != "" && (top["id"] != "" || top["title"] != "") {
				binds = append(binds, keybind{action: top["id"], description: top["title"], value: top["bind"], user: true, plugin: true})
			}
		case prev == '{' || prev == ',':
			m := pluginField.FindStringSubmatchIndex(src[i:])
			if m == nil {
				break
			}
			if value, err := strconv.Unquote(src[i+m[4] : i+m[5]]); err == nil {
				objects[len(objects)-1][src[i+m[2]:i+m[3]]] = value
			}
			i += m[5] - 1
			c = '"'
		}
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			prev = c
		}
	}
	for i, k := range binds {
		if k.action == "" {
			binds[i].action = cmp.Or(ids[k.description], k.description)
		}
	}
	return binds
}

func stringEnd(src string, i int) int {
	quote := src[i]
	for i++; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case quote:
			return i
		case '\n':
			if quote != '`' {
				return i
			}
		}
	}
	return len(src)
}

func opencodeDefaults(data []byte) ([]keybind, error) {
	err := errors.New("keybind table not found")
	for at := 0; ; {
		i := bytes.Index(data[at:], []byte(leaderText))
		if i < 0 {
			return nil, err
		}
		at += i + len(leaderText)
		var binds []keybind
		binds, err = opencodeTable(data, at-len(leaderText))
		if err == nil && slices.ContainsFunc(binds, func(k keybind) bool { return strings.Contains(k.action, ".") }) {
			return binds, nil
		}
		if err == nil {
			err = errors.New("only legacy keybind tables found")
		}
	}
}

func opencodeTable(data []byte, at int) ([]keybind, error) {
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
	case expr == "!1":
		return false, nil
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
	if len(s) > 1 && strings.Count(s, s[:1]) == len(s) {
		for range len(s) - 1 {
			seq = append(seq, press(m, s[:1]))
		}
		s = s[:1]
	}
	return append(seq, press(m, s)), true
}
