package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	svgRoot  = regexp.MustCompile(`(?s)^.*?<svg[^>]*>`)
	svgGroup = regexp.MustCompile(`(?s)<g id="([^"]+)">(.*?)</g>`)
)

func splitIcons(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	masters, err := filepath.Glob(filepath.Join(src, "*-symbolic.svg"))
	if err != nil {
		return err
	}
	for _, path := range masters {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		root := svgRoot.Find(data)
		if root == nil {
			return errors.New(path + ": no <svg> root")
		}
		name := strings.TrimSuffix(filepath.Base(path), "-symbolic.svg")
		for _, g := range svgGroup.FindAllSubmatch(data, -1) {
			part := slices.Concat(root, g[2], []byte("</svg>\n"))
			if err := os.WriteFile(filepath.Join(dst, name+"-"+string(g[1])+"-symbolic.svg"), part, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
