package main

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	layerSite  = regexp.MustCompile(`\.(createLayer|layer)\(\(\)=>`)
	commandID  = regexp.MustCompile(`\{id:"[^"\\]+"`)
	actionSite = regexp.MustCompile(`command:"([\w.-]+)"`)
	memo       = regexp.MustCompile(`^[\w$]+\(\(\)=>\[`)
	mapField   = regexp.MustCompile(`^(.+)\.map\(\(?([\w$]+)\)?=>([\w$]+)\.(\w+)\)$`)
	identifier = regexp.MustCompile(`^[\w$]+$`)
	member     = regexp.MustCompile(`^([\w$]+)\.(\w+)$`)
)

func opencodeSites(data []byte, names []string) (map[string]string, error) {
	known := map[string]bool{}
	for _, name := range names {
		known[name] = true
	}
	sites := map[string]string{}
	add := func(names []string, global bool, scope string) {
		for _, name := range names {
			if !known[name] {
				continue
			}
			if current, ok := sites[name]; ok && (current == "" || !global) {
				continue
			}
			switch {
			case global:
				sites[name] = ""
			case scope != "":
				sites[name] = scope
			default:
				sites[name], _, _ = strings.Cut(name, ".")
			}
		}
	}

	var layers [][2]int
	for _, m := range layerSite.FindAllSubmatchIndex(data, -1) {
		open := m[1] - len("()=>") - 1
		end, err := jsEnd(data, open)
		if err != nil {
			continue
		}
		layers = append(layers, [2]int{open, end})
		object, ok := jsLayer(data[m[1] : end-1])
		if !ok {
			continue
		}
		global := string(data[m[2]:m[3]]) == "createLayer"
		var commands, bindings string
		for _, prop := range jsSplit(object[1 : len(object)-1]) {
			name, value, _ := strings.Cut(prop, ":")
			switch name {
			case "mode":
				global = value == `"global"` || value == `"base"`
			case "commands":
				commands = value
			case "bindings":
				bindings = value
			}
		}
		if commands != "" {
			add(jsCommands(data, commands, open), global, "")
		}
		if bindings != "" {
			list, _ := jsList(data, bindings, open, 0)
			add(list, global, "")
		}
	}
	if len(layers) == 0 {
		return nil, errors.New("no keymap layer sites")
	}

	needle := []byte("bindings:[")
	for at := 0; ; {
		i := bytes.Index(data[at:], needle)
		if i < 0 {
			break
		}
		at += i + len(needle)
		open := at - 1
		if slices.ContainsFunc(layers, func(l [2]int) bool { return l[0] < open && open < l[1] }) {
			continue
		}
		end, err := jsEnd(data, open)
		if err != nil {
			continue
		}
		list, _ := jsList(data, string(data[open:end]), open, 0)
		add(list, true, "")
	}

	for _, loc := range actionSite.FindAllSubmatchIndex(data, -1) {
		from := max(0, loc[0]-512)
		open := bytes.LastIndexAny(data[from:loc[0]], "{}")
		if open < 0 || data[from+open] != '{' {
			continue
		}
		end, err := jsEnd(data, from+open)
		if err != nil {
			continue
		}
		props := jsSplit(string(data[from+open+1 : end-1]))
		if slices.ContainsFunc(props, func(p string) bool {
			return strings.HasPrefix(p, "onTrigger:") || strings.HasPrefix(p, "onTrigger(")
		}) {
			add([]string{string(data[loc[2]:loc[3]])}, false, "dialog")
		}
	}
	if len(sites) == 0 {
		return nil, errors.New("no keybinding sites")
	}
	return sites, nil
}

func jsLayer(body []byte) (string, bool) {
	at := 0
	switch {
	case bytes.HasPrefix(body, []byte("({")):
		at = 1
	case bytes.HasPrefix(body, []byte("{")):
		i := bytes.Index(body, []byte("return{"))
		if i < 0 {
			return "", false
		}
		at = i + len("return")
	default:
		return "", false
	}
	end, err := jsEnd(body, at)
	if err != nil {
		return "", false
	}
	return string(body[at:end]), true
}

func jsCommands(data []byte, expr string, near int) []string {
	scan := func(literal string) ([]string, error) {
		var ids []string
		for _, loc := range commandID.FindAllStringIndex(literal, -1) {
			end, err := jsEnd([]byte(literal), loc[0])
			if err != nil {
				continue
			}
			object := literal[loc[0]:end]
			if slices.Contains(jsSplit(object[1:len(object)-1]), "bind:!1") {
				continue
			}
			id, _ := jsProperty(object, "id")
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return nil, errors.New("no command IDs")
		}
		return ids, nil
	}
	if identifier.MatchString(expr) {
		ids, _ := jsDefinition(data, expr, '[', near, scan)
		return ids
	}
	ids, _ := scan(expr)
	return ids
}

func jsList(data []byte, expr string, near, depth int) ([]string, error) {
	if depth > 4 {
		return nil, fmt.Errorf("%s: references nest too deep", expr)
	}
	if jsArray(expr) {
		var list []string
		for _, item := range jsSplit(expr[1 : len(expr)-1]) {
			values, err := jsItem(data, item, near, depth)
			if err != nil {
				return nil, err
			}
			list = append(list, values...)
		}
		return list, nil
	}
	if identifier.MatchString(expr) {
		return jsDefinition(data, expr, '[', near, func(literal string) ([]string, error) {
			return jsList(data, literal, near, depth+1)
		})
	}
	if m := mapField.FindStringSubmatch(expr); m != nil && m[2] == m[3] {
		objects, err := jsObjects(data, m[1], near, depth+1)
		if err != nil {
			return nil, err
		}
		var list []string
		for _, object := range objects {
			value, ok := jsProperty(object, m[4])
			if !ok {
				return nil, fmt.Errorf("%s: element without string %s", m[1], m[4])
			}
			list = append(list, value)
		}
		return list, nil
	}
	if i := strings.Index(expr, "?["); i > 0 && strings.HasSuffix(expr, ":[]") {
		return jsList(data, expr[i+1:len(expr)-len(":[]")], near, depth+1)
	}
	return nil, fmt.Errorf("unresolvable command list %.60s", expr)
}

func jsObjects(data []byte, expr string, near, depth int) ([]string, error) {
	if depth > 4 {
		return nil, fmt.Errorf("%s: references nest too deep", expr)
	}
	if jsArray(expr) {
		var objects []string
		for _, item := range jsSplit(expr[1 : len(expr)-1]) {
			if spread, ok := strings.CutPrefix(item, "..."); ok {
				nested, err := jsObjects(data, spread, near, depth+1)
				if err != nil {
					return nil, err
				}
				objects = append(objects, nested...)
				continue
			}
			if !strings.HasPrefix(item, "{") {
				return nil, fmt.Errorf("element is not an object: %.60s", item)
			}
			objects = append(objects, item)
		}
		return objects, nil
	}
	name := strings.TrimSuffix(expr, "()")
	if !identifier.MatchString(name) {
		return nil, fmt.Errorf("unresolvable object list %.60s", expr)
	}
	return jsDefinition(data, name, '[', near, func(literal string) ([]string, error) {
		return jsObjects(data, literal, near, depth+1)
	})
}

func jsArray(expr string) bool {
	if !strings.HasPrefix(expr, "[") {
		return false
	}
	end, err := jsEnd([]byte(expr), 0)
	return err == nil && end == len(expr)
}

func jsItem(data []byte, item string, near, depth int) ([]string, error) {
	if spread, ok := strings.CutPrefix(item, "..."); ok {
		return jsList(data, spread, near, depth+1)
	}
	if strings.HasPrefix(item, `"`) {
		s, err := strconv.Unquote(item)
		return []string{s}, err
	}
	if m := member.FindStringSubmatch(item); m != nil {
		return jsDefinition(data, m[1], '{', near, func(literal string) ([]string, error) {
			value, ok := jsProperty(literal, m[2])
			if !ok {
				return nil, fmt.Errorf("%s has no string %s", m[1], m[2])
			}
			return []string{value}, nil
		})
	}
	return nil, fmt.Errorf("unresolvable command %.60s", item)
}

func jsDefinition(data []byte, name string, open byte, near int, parse func(string) ([]string, error)) ([]string, error) {
	needle := []byte(name + "=")
	var starts []int
	for at := 0; ; {
		i := bytes.Index(data[at:], needle)
		if i < 0 {
			break
		}
		at += i + len(needle)
		if before := data[at-len(needle)-1]; before == '_' || before == '$' || before == '.' || isAlnum(before) {
			continue
		}
		switch {
		case at < len(data) && data[at] == open:
			starts = append(starts, at)
		case open == '[':
			if m := memo.Find(data[at:min(len(data), at+64)]); m != nil {
				starts = append(starts, at+len(m)-1)
			}
		}
	}
	slices.SortFunc(starts, func(a, b int) int { return abs(a-near) - abs(b-near) })
	for _, start := range starts {
		end, err := jsEnd(data, start)
		if err != nil {
			continue
		}
		if list, err := parse(string(data[start:end])); err == nil {
			return list, nil
		}
	}
	return nil, fmt.Errorf("no resolvable definition of %s", name)
}

func jsProperty(object, name string) (string, bool) {
	for _, prop := range jsSplit(object[1 : len(object)-1]) {
		if value, ok := strings.CutPrefix(prop, name+":"); ok {
			s, err := strconv.Unquote(value)
			return s, err == nil
		}
	}
	return "", false
}

func jsEnd(data []byte, at int) (int, error) {
	var stack []byte
	closers := map[byte]byte{'(': ')', '[': ']', '{': '}'}
	for i := at; i < len(data) && i < at+1<<16; i++ {
		switch c := data[i]; c {
		case '"', '\'', '`':
			i = jsStringEnd(data, i) - 1
		case '(', '[', '{':
			stack = append(stack, closers[c])
		case ')', ']', '}':
			if len(stack) == 0 || stack[len(stack)-1] != c {
				return 0, fmt.Errorf("unbalanced %q at %d", c, i)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, errors.New("unterminated expression")
}

func jsSplit(s string) []string {
	var parts []string
	data := []byte(s)
	start := 0
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '"', '\'', '`':
			i = jsStringEnd(data, i) - 1
		case '(', '[', '{':
			end, err := jsEnd(data, i)
			if err != nil {
				return nil
			}
			i = end - 1
		case ',':
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" {
		parts = append(parts, rest)
	}
	return parts
}

func jsStringEnd(data []byte, at int) int {
	j := at + 1
	for j < len(data) && data[j] != data[at] {
		if data[j] == '\\' {
			j++
		}
		j++
	}
	return j + 1
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
