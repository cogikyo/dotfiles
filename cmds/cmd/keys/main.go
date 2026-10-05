package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"dotfiles/cmds/internal/dctl/paths"
)

type server struct {
	static     string
	definition string
	vil        string
	apps       []*cached
}

type state struct {
	Version         string          `json:"version"`
	Definition      json.RawMessage `json:"definition"`
	DefinitionError string          `json:"definitionError,omitempty"`
	Vil             json.RawMessage `json:"vil"`
	VilError        string          `json:"vilError,omitempty"`
	Apps            []App           `json:"apps"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:42070", "listen address")
	flag.Parse()

	root, err := paths.DiscoverRoot()
	if err != nil {
		slog.Error("keys: dotfiles root", "err", err)
		os.Exit(1)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		slog.Error("keys: config dir", "err", err)
		os.Exit(1)
	}
	kittyConf := filepath.Join(configDir, "kitty", "kitty.conf")
	if dir := os.Getenv("KITTY_CONFIG_DIRECTORY"); dir != "" {
		kittyConf = filepath.Join(dir, "kitty.conf")
	}
	nvimConfig := filepath.Join(configDir, "nvim")
	tui := filepath.Join(configDir, "opencode", "tui.json")
	kittyBin, nvimBin, opencodeBin := binary("kitty"), binary("nvim"), binary("opencode")

	s := server{
		static:     filepath.Join(root.Dotfiles, "cmds", "cmd", "keys"),
		definition: root.Share("keyboards", "svalboard.json"),
		vil:        root.Share("keyboards", "svalboard.vil"),
		apps: []*cached{
			{
				inputs:  func() []string { return []string{kittyConf, kittyBin} },
				extract: kitty,
			},
			{
				inputs:  func() []string { return append(tree(nvimConfig), nvimBin) },
				extract: nvim,
			},
			{
				inputs:  func() []string { return []string{tui, opencodeBin} },
				extract: func() (App, []string) { return opencode(opencodeBin, tui), nil },
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.Handle("GET /fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir(root.Share("fonts")))))
	mux.Handle("GET /", noCache(http.FileServer(http.Dir(s.static))))

	slog.Info("keys listening", "addr", "http://"+*addr, "static", s.static)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		slog.Error("keys: serve", "err", err)
		os.Exit(1)
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func (s server) handleState(w http.ResponseWriter, r *http.Request) {
	var st state
	st.Definition, st.DefinitionError = readJSON(s.definition)
	st.Vil, st.VilError = readJSON(s.vil)
	st.Apps = []App{hyprland()}
	for _, c := range s.apps {
		st.Apps = append(st.Apps, c.get())
	}

	body, err := json.Marshal(st)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(body)
	st.Version = hex.EncodeToString(sum[:])[:16]

	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("v") == st.Version {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(st); err != nil {
		slog.Warn("keys: write state", "err", err)
	}
}

func readJSON(path string) (json.RawMessage, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err.Error()
	}
	if !json.Valid(data) {
		return nil, fmt.Sprintf("%s is not valid JSON", path)
	}
	return data, ""
}
