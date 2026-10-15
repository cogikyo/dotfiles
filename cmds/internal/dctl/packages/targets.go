package packages

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Targets map[string][]string

const common = "common"

func (l Lists) Targets() Targets {
	t := Targets(maps.Clone(l.Vendor))
	t[common] = unique(slices.Concat(l.Base, l.AUR, l.Local))
	return t
}

func (t Targets) Select(vendors []string) []string {
	names := slices.Clone(t[common])
	for _, v := range vendors {
		names = append(names, t[v]...)
	}
	return unique(names)
}

func (t Targets) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, names := range t {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func ReadTargets(dir string) (Targets, error) {
	t := Targets{}
	var errs []error
	for _, name := range append([]string{common}, Vendors...) {
		names, err := Read(filepath.Join(dir, name))
		t[name] = names
		errs = append(errs, err)
	}
	if errs[0] == nil && len(t[common]) == 0 {
		errs = append(errs, fmt.Errorf("%s: no common packages", dir))
	}
	return t, errors.Join(errs...)
}
