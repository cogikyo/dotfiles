package src

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type entry struct {
	name string
	path string
	ref  string
	size int64
	age  time.Duration
}

func (a app) ls(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: src ls")
	}
	entries, err := a.entries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return outputLine(a.stdout, "no entries")
	}
	for _, e := range entries {
		if _, err := fmt.Fprintf(a.stdout, "%s\t%s\t%s\t%s\n", e.name, e.ref, formatBytes(e.size), formatAge(e.age)); err != nil {
			return err
		}
	}
	return nil
}

func (a app) entries() ([]entry, error) {
	var entries []entry
	if !isDir(a.cache) {
		return entries, nil
	}
	err := filepath.WalkDir(a.cache, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == a.cache {
			return nil
		}
		name := d.Name()
		at := strings.LastIndex(name, "@")
		if at < 1 {
			return nil
		}
		ref := name[at+1:]
		if err := validateRef(ref); err != nil {
			return fmt.Errorf("invalid cache entry %q: %w", name, err)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size, err := dirSize(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(a.cache, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{
			name: filepath.ToSlash(rel),
			path: path,
			ref:  ref,
			size: size,
			age:  time.Since(info.ModTime()).Round(time.Second),
		})
		return filepath.SkipDir
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries, err
}

func dirSize(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func formatBytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	v := float64(n)
	unit := units[0]
	for _, u := range units[1:] {
		if v < 1024 {
			break
		}
		v /= 1024
		unit = u
	}
	if unit == "B" {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1f%s", v, unit)
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
