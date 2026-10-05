package main

type Chord struct {
	Mods []string `json:"mods"`
	Key  string   `json:"key"`
}

type Bind struct {
	Prefix []Chord `json:"prefix"`
	Chord
	Label      string  `json:"label"`
	Detail     string  `json:"detail"`
	Builtin    bool    `json:"builtin"`
	Scope      string  `json:"scope,omitempty"`
	ShadowedBy []Chord `json:"shadowedBy,omitempty"`
}

type Group struct {
	Prefix []Chord `json:"prefix"`
	Label  string  `json:"label"`
}

type App struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Error  string  `json:"error,omitempty"`
	Note   string  `json:"note,omitempty"`
	Binds  []Bind  `json:"binds"`
	Groups []Group `json:"groups,omitempty"`
}
