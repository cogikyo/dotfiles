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
	gatherSite = regexp.MustCompile(`keybinds\.gather\(`)
	getSite    = regexp.MustCompile(`keybinds\.get\("([^"\\]+)"\)`)
	actionSite = regexp.MustCompile(`command:"([\w.-]+)"`)
	mapField   = regexp.MustCompile(`^([\w$]+)\.map\(\(?([\w$]+)\)?=>([\w$]+)\.(\w+)\)$`)
	identifier = regexp.MustCompile(`^[\w$]+$`)
	member     = regexp.MustCompile(`^([\w$]+)\.(\w+)$`)
)

func opencodeCommands(data []byte, first string) (map[string]string, error) {
	needle := []byte("{" + first + `:"`)
	for at := 0; ; {
		i := bytes.Index(data[at:], needle)
		if i < 0 {
			return nil, fmt.Errorf("command map starting with %s not found", first)
		}
		at += i + 1
		end, err := jsEnd(data, at-1)
		if err != nil {
			continue
		}
		commands := map[string]string{}
		for _, prop := range jsSplit(string(data[at : end-1])) {
			name, value, _ := strings.Cut(prop, ":")
			command, err := strconv.Unquote(value)
			if err != nil {
				commands = nil
				break
			}
			commands[name] = command
		}
		if len(commands) > 0 {
			return commands, nil
		}
	}
}

func opencodeScopes(data []byte, commands []string) (map[string][]string, error) {
	known := map[string]bool{}
	for _, c := range commands {
		known[c] = true
	}
	scopes := map[string][]string{}
	add := func(command, scope string) {
		if known[command] && !slices.Contains(scopes[command], scope) {
			scopes[command] = append(scopes[command], scope)
		}
	}

	gathers := gatherSite.FindAllIndex(data, -1)
	if len(gathers) == 0 {
		return nil, errors.New("no keybinds.gather sites")
	}
	for _, loc := range gathers {
		open := loc[1] - 1
		end, err := jsEnd(data, open)
		if err != nil {
			return nil, fmt.Errorf("keybinds.gather at %d: %w", open, err)
		}
		args := jsSplit(string(data[open+1 : end-1]))
		if len(args) != 2 {
			return nil, fmt.Errorf("keybinds.gather at %d: %d arguments", open, len(args))
		}
		layer, err := strconv.Unquote(args[0])
		if err != nil {
			return nil, fmt.Errorf("keybinds.gather at %d: layer %s: %w", open, args[0], err)
		}
		list, err := jsList(data, args[1], open, 0)
		if err != nil {
			return nil, fmt.Errorf("keybinds.gather(%q): %w", layer, err)
		}
		for _, command := range list {
			add(command, layer)
		}
	}

	gets := getSite.FindAllSubmatch(data, -1)
	if len(gets) == 0 {
		return nil, errors.New("no keybinds.get sites")
	}
	for _, m := range gets {
		command := string(m[1])
		namespace := command[:max(0, strings.LastIndex(command, "."))]
		add(command, namespace)
	}

	dialogs := 0
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
		if !slices.ContainsFunc(props, func(p string) bool {
			return strings.HasPrefix(p, "onTrigger:") || strings.HasPrefix(p, "onTrigger(")
		}) {
			continue
		}
		add(string(data[loc[2]:loc[3]]), "dialog")
		dialogs++
	}
	if dialogs == 0 {
		return nil, errors.New("no dialog action sites")
	}
	return scopes, nil
}

func jsList(data []byte, expr string, near, depth int) ([]string, error) {
	if depth > 4 {
		return nil, fmt.Errorf("%s: references nest too deep", expr)
	}
	if strings.HasPrefix(expr, "[") {
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
		return jsDefinition(data, m[1], '[', near, func(literal string) ([]string, error) {
			var list []string
			for _, item := range jsSplit(literal[1 : len(literal)-1]) {
				if !strings.HasPrefix(item, "{") {
					return nil, fmt.Errorf("%s: element is not an object", m[1])
				}
				value, ok := jsProperty(item, m[4])
				if !ok {
					return nil, fmt.Errorf("%s: element without string %s", m[1], m[4])
				}
				list = append(list, value)
			}
			return list, nil
		})
	}
	return nil, fmt.Errorf("unresolvable command list %.60s", expr)
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
	needle := []byte(name + "=" + string(open))
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
		starts = append(starts, at-1)
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
