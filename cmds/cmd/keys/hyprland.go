package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/hyprd/hypr"
)

func hyprland() App {
	app := App{ID: "hyprland", Name: "Hyprland", Binds: []Bind{}}
	client, err := connect()
	if err != nil {
		app.Error = err.Error()
		return app
	}
	data, err := client.Request("j/binds")
	if err != nil {
		app.Error = err.Error()
		return app
	}
	var raw []struct {
		Modmask     int    `json:"modmask"`
		Submap      string `json:"submap"`
		Key         string `json:"key"`
		Keycode     int    `json:"keycode"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		app.Error = fmt.Sprintf("hyprland returned %d bytes of unreadable binds: %v", len(data), err)
		return app
	}
	for _, b := range raw {
		if b.Submap != "" {
			continue
		}
		var m mods
		for bit, mod := range map[int]mods{1: shift, 4: ctrl, 8: alt, 64: super} {
			if b.Modmask&bit != 0 {
				m |= mod
			}
		}
		key := strings.ToLower(b.Key)
		if key == "" {
			key = fmt.Sprintf("code:%d", b.Keycode)
		}
		app.Binds = append(app.Binds, Bind{Prefix: []Chord{}, Chord: press(m, key), Label: b.Description})
	}
	return app
}

func connect() (*hypr.Client, error) {
	client, err := hypr.NewClient()
	if err == nil {
		return client, nil
	}
	sig, findErr := newestInstance()
	if findErr != nil {
		return nil, errors.Join(err, findErr)
	}
	if err := os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", sig); err != nil {
		return nil, err
	}
	return hypr.NewClient()
}

func newestInstance() (string, error) {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	dirs, err := filepath.Glob(filepath.Join(runtime, "hypr", "*", ".socket.sock"))
	if err != nil {
		return "", err
	}
	type instance struct {
		sig  string
		unix int64
	}
	var live []instance
	for _, sock := range dirs {
		st, err := os.Stat(sock)
		if err != nil {
			continue
		}
		live = append(live, instance{filepath.Base(filepath.Dir(sock)), st.ModTime().UnixNano()})
	}
	if len(live) == 0 {
		return "", errors.New("no Hyprland instance socket under " + filepath.Join(runtime, "hypr"))
	}
	return slices.MaxFunc(live, func(a, b instance) int { return cmp.Compare(a.unix, b.unix) }).sig, nil
}
