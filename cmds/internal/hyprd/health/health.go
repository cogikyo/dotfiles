package health

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Finding struct {
	ID       string    `json:"id"`
	Check    string    `json:"check"`
	Subject  string    `json:"subject"`
	Summary  string    `json:"summary"`
	Evidence string    `json:"evidence"`
	Opened   time.Time `json:"opened"`
}

func (f Finding) key() string {
	return f.Check + "\x00" + f.Subject
}

type event struct {
	Event string    `json:"event"`
	At    time.Time `json:"at"`
	Finding
}

type Monitor struct {
	dir    string
	onOpen func(context.Context, []Finding)

	mu   sync.Mutex
	hist *history
	open map[string]Finding
}

func New(onOpen func(context.Context, []Finding)) (*Monitor, error) {
	dir, err := stateDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	m := &Monitor{dir: dir, onOpen: onOpen}
	m.hist, err = loadHistory(m.historyPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd health: %v; starting fresh history\n", err)
	}
	if m.open, err = replay(m.Log()); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Monitor) Log() string {
	return filepath.Join(m.dir, "findings.jsonl")
}

func (m *Monitor) historyPath() string {
	return filepath.Join(m.dir, "history.json")
}

func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		m.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Monitor) tick(ctx context.Context) {
	m.mu.Lock()
	now := time.Now()
	procs := m.sample(ctx, now)
	if ctx.Err() != nil {
		m.mu.Unlock()
		return
	}
	if m.hist.Evaluated == 0 {
		m.hist.Evaluated = now.Unix()
	}
	var opened []Finding
	if procs != nil && now.Sub(time.Unix(m.hist.Evaluated, 0)) >= Window {
		var err error
		if opened, err = m.evaluate(ctx, now, Window, procs); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd health: %v; retrying next sample\n", err)
		} else {
			m.hist.Evaluated = now.Unix()
		}
	}
	m.save()
	m.mu.Unlock()

	if len(opened) > 0 {
		m.onOpen(ctx, opened)
	}
}

func (m *Monitor) Check(ctx context.Context, window time.Duration) ([]Finding, error) {
	m.mu.Lock()
	now := time.Now()
	procs := m.sample(ctx, now)
	if procs == nil {
		m.mu.Unlock()
		return nil, errors.New("process sample failed")
	}
	opened, err := m.evaluate(ctx, now, window, procs)
	m.save()
	open := m.sorted()
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if len(opened) > 0 {
		m.onOpen(ctx, opened)
	}
	return open, nil
}

func (m *Monitor) Lookup(ids []string) ([]Finding, error) {
	if len(ids) == 0 {
		m.mu.Lock()
		open := m.sorted()
		m.mu.Unlock()
		if len(open) == 0 {
			return nil, errors.New("no open findings")
		}
		return open, nil
	}

	data, err := os.ReadFile(m.Log())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	latest := map[string]Finding{}
	for e := range records(data) {
		if slices.Contains(ids, e.ID) {
			latest[e.ID] = e.Finding
		}
	}
	found := make([]Finding, 0, len(ids))
	for _, id := range ids {
		f, ok := latest[id]
		if !ok {
			return nil, fmt.Errorf("unknown finding %s", id)
		}
		found = append(found, f)
	}
	return found, nil
}

func (m *Monitor) sorted() []Finding {
	return slices.SortedFunc(maps.Values(m.open), func(a, b Finding) int {
		return strings.Compare(a.key(), b.key())
	})
}

func (m *Monitor) sample(ctx context.Context, now time.Time) []proc {
	procs, errs := m.hist.sample(ctx, now)
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "hyprd health: sample: %v\n", err)
	}
	return procs
}

func (m *Monitor) save() {
	if err := m.hist.save(m.historyPath()); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd health: save history: %v\n", err)
	}
}

func (m *Monitor) evaluate(ctx context.Context, now time.Time, window time.Duration, procs []proc) ([]Finding, error) {
	found, held := m.hist.findings(ctx, now, window, procs)

	next := maps.Clone(m.open)
	var opened []Finding
	var events []event
	current := map[string]bool{}
	for _, f := range found {
		key := f.key()
		current[key] = true
		if prev, ok := next[key]; ok {
			prev.Summary, prev.Evidence = f.Summary, f.Evidence
			next[key] = prev
			continue
		}
		f.ID = newID(f.Check)
		f.Opened = now
		next[key] = f
		opened = append(opened, f)
		events = append(events, event{Event: "open", At: now, Finding: f})
	}
	for _, key := range slices.Sorted(maps.Keys(next)) {
		f := next[key]
		if current[key] || held[f.Check] {
			continue
		}
		delete(next, key)
		events = append(events, event{Event: "resolve", At: now, Finding: f})
	}

	if err := m.append(events); err != nil {
		return nil, fmt.Errorf("append %s: %w", m.Log(), err)
	}
	m.open = next
	for _, f := range opened {
		fmt.Fprintf(os.Stderr, "hyprd health: open %s: %s\n", f.ID, f.Summary)
	}
	return opened, nil
}

func (m *Monitor) append(events []event) error {
	if len(events) == 0 {
		return nil
	}
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(m.Log())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if err := os.Truncate(m.Log(), int64(bytes.LastIndexByte(data, '\n')+1)); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(m.Log(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(buf.String()); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func replay(path string) (map[string]Finding, error) {
	open := map[string]Finding{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return open, nil
	}
	if err != nil {
		return nil, err
	}
	for e := range records(data) {
		switch e.Event {
		case "open":
			open[e.key()] = e.Finding
		case "resolve":
			delete(open, e.key())
		}
	}
	return open, nil
}

func records(data []byte) iter.Seq[event] {
	return func(yield func(event) bool) {
		for line := range bytes.Lines(data) {
			// Skip a torn final line; append truncates it before its next write.
			if !bytes.HasSuffix(line, []byte("\n")) {
				return
			}
			var e event
			if json.Unmarshal(line, &e) == nil && !yield(e) {
				return
			}
		}
	}
}

func newID(check string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return check + "-" + hex.EncodeToString(b)
}
