package src

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type pruneOptions struct {
	all       bool
	olderThan time.Duration
	entries   []string
}

func (a app) prune(args []string) error {
	opts, err := parsePrune(args)
	if err != nil {
		return err
	}
	if opts.all {
		if err := os.RemoveAll(a.cache); err != nil {
			return fmt.Errorf("remove cache: %w", err)
		}
		fmt.Fprintln(a.stdout, "pruned all entries")
		return nil
	}

	entries, err := a.entries()
	if err != nil {
		return err
	}
	targets, err := pruneTargets(entries, opts)
	if err != nil {
		return err
	}
	for _, e := range targets {
		if err := os.RemoveAll(e.path); err != nil {
			return fmt.Errorf("prune %s: %w", e.name, err)
		}
	}
	fmt.Fprintf(a.stdout, "pruned %d entries\n", len(targets))
	return nil
}

func parsePrune(args []string) (pruneOptions, error) {
	opts := pruneOptions{olderThan: 60 * 24 * time.Hour}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			opts.all = true
		case "--older-than":
			i++
			if i >= len(args) {
				return opts, errors.New("--older-than needs a duration like 60d")
			}
			d, err := parseDays(args[i])
			if err != nil {
				return opts, err
			}
			opts.olderThan = d
		default:
			if value, ok := strings.CutPrefix(args[i], "--older-than="); ok {
				d, err := parseDays(value)
				if err != nil {
					return opts, err
				}
				opts.olderThan = d
				continue
			}
			if strings.HasPrefix(args[i], "-") {
				return opts, fmt.Errorf("unknown prune flag %q", args[i])
			}
			opts.entries = append(opts.entries, args[i])
		}
	}
	if opts.all && len(opts.entries) > 0 {
		return opts, errors.New("--all cannot be combined with explicit entries")
	}
	return opts, nil
}

func parseDays(s string) (time.Duration, error) {
	if !strings.HasSuffix(s, "d") {
		return 0, fmt.Errorf("duration %q must use days, e.g. 60d", s)
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("duration %q must use a non-negative day count", s)
	}
	return time.Duration(n) * 24 * time.Hour, nil
}

func pruneTargets(entries []entry, opts pruneOptions) ([]entry, error) {
	if len(opts.entries) > 0 {
		return namedEntries(entries, opts.entries)
	}
	cutoff := time.Now().Add(-opts.olderThan)
	var targets []entry
	for _, e := range entries {
		info, err := os.Stat(e.path)
		if err != nil {
			return nil, err
		}
		if info.ModTime().Before(cutoff) {
			targets = append(targets, e)
		}
	}
	return targets, nil
}

func namedEntries(entries []entry, names []string) ([]entry, error) {
	var targets []entry
	for _, name := range names {
		var matches []entry
		for _, e := range entries {
			base := filepath.Base(e.name)
			if e.name == name || base == name {
				matches = append(matches, e)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("unknown entry %q", name)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("entry %q is ambiguous; use the full cache-relative path", name)
		}
		targets = append(targets, matches[0])
	}
	return targets, nil
}
