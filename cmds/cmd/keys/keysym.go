package main

import (
	"strings"
	"unicode/utf8"
)

type mods uint8

const (
	ctrl mods = 1 << iota
	shift
	alt
	super
)

var modNames = [...]struct {
	bit  mods
	name string
}{{ctrl, "ctrl"}, {shift, "shift"}, {alt, "alt"}, {super, "super"}}

func (m mods) names() []string {
	names := []string{}
	for _, n := range modNames {
		if m&n.bit != 0 {
			names = append(names, n.name)
		}
	}
	return names
}

func press(m mods, name string) Chord {
	key, shifted := keysym(name)
	if shifted {
		m |= shift
	}
	return Chord{Mods: m.names(), Key: key}
}

var plain = map[rune]string{
	'-': "minus", '=': "equal", '[': "bracketleft", ']': "bracketright",
	'\\': "backslash", ';': "semicolon", '\'': "apostrophe", '`': "grave",
	',': "comma", '.': "period", '/': "slash", ' ': "space",
}

var shifted = map[rune]string{
	'_': "minus", '+': "equal", '{': "bracketleft", '}': "bracketright",
	'|': "backslash", ':': "semicolon", '"': "apostrophe", '~': "grave",
	'<': "comma", '>': "period", '?': "slash",
	'!': "1", '@': "2", '#': "3", '$': "4", '%': "5",
	'^': "6", '&': "7", '*': "8", '(': "9", ')': "0",
}

var aliases = map[string]string{
	"enter": "return", "cr": "return",
	"esc": "escape",
	"bs":  "backspace",
	"del": "delete", "ins": "insert",
	"pageup": "prior", "pgup": "prior", "page_up": "prior",
	"pagedown": "next", "pgdn": "next", "page_down": "next",
	"print_screen": "print",
	"lt":           "<", "bslash": "\\", "bar": "|",
}

func keysym(name string) (string, bool) {
	if r, size := utf8.DecodeRuneInString(name); size == len(name) && size > 0 {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return name, false
		case r >= 'A' && r <= 'Z':
			return strings.ToLower(name), true
		}
		if k, ok := plain[r]; ok {
			return k, false
		}
		if k, ok := shifted[r]; ok {
			return k, true
		}
		return strings.ToLower(name), false
	}
	lower := strings.ToLower(name)
	if a, ok := aliases[lower]; ok {
		return keysym(a)
	}
	return lower, false
}
