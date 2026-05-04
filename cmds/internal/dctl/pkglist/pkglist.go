// Package pkglist parses package list files shared by install, update, and ISO commands.
//
// Responsibilities:
// - Strip blank lines and comments from simple package manifests.
// - Deduplicate and sort package names before callers compare or write lists.
package pkglist

// pkglist.go defines package-list parsing and normalization helpers.

import (
	"bufio"
	"io"
	"os"
	"slices"
	"strings"
)

func Read(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f), nil
}

func ParseString(data string) []string {
	return Parse(strings.NewReader(data))
}

func Parse(r io.Reader) []string {
	var packages []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) > 0 {
			packages = append(packages, fields[0])
		}
	}
	return Unique(packages)
}

func Unique(packages []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(packages))
	for _, pkg := range packages {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	slices.Sort(out)
	return out
}
