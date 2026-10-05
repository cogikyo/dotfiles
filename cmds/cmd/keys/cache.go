package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type cached struct {
	inputs  func() []string
	extract func() (App, []string)

	mu    sync.Mutex
	stamp string
	app   *App
	extra []string
}

func (c *cached) get() App {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := stamp(slices.Concat(c.inputs(), c.extra))
	if c.app != nil && current == c.stamp {
		return *c.app
	}
	start := time.Now()
	app, extra := c.extract()
	if !slices.Equal(extra, c.extra) {
		current = stamp(slices.Concat(c.inputs(), extra))
	}
	c.app, c.extra, c.stamp = &app, extra, current
	slog.Info("keys: extracted", "app", app.ID, "binds", len(app.Binds), "err", app.Error, "took", time.Since(start).Round(time.Millisecond))
	return app
}

func stamp(paths []string) string {
	sum := sha256.New()
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(sum, "%s\x00missing\n", p)
			continue
		}
		fmt.Fprintf(sum, "%s\x00%d\x00%d\n", p, st.ModTime().UnixNano(), st.Size())
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func tree(root string) []string {
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	files := []string{root}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && p != root {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func binary(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return name
	}
	return path
}
