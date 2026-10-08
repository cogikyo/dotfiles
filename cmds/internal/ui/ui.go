package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// nestEnv tells a dotfiles command started inside a transcript how to continue it:
// "open" continues the parent's open tree, and "closed" starts new trees after a blank line.
const nestEnv = "DOTFILES_TREE"

type Options struct {
	Context context.Context
	JSON    bool
	Plain   bool
	Yes     bool
}

type Level int

const (
	Info Level = iota
	OK
	Warn
	Err
	command
	ask
	Todo
	Fix
	Later
	Optional
	Wait
)

var (
	ErrNoTTY    = errors.New("interactive terminal required")
	ErrCanceled = errors.New("canceled")
)

type UI struct {
	opts   Options
	in     *os.File
	stdout *os.File
	stderr *os.File
	out    io.Writer
	err    io.Writer
	glyph  glyphs

	mu      sync.Mutex
	title   string
	nested  bool
	drawn   bool
	printed bool
	trees   []tree
	pending *child
}

type tree struct {
	title string
	start time.Time
	flat  bool
}

type child struct {
	level   Level
	pill    bool
	msg     string
	details []string
}

func New(opts Options) *UI {
	out := colorprofile.NewWriter(os.Stdout, os.Environ())
	errw := colorprofile.NewWriter(os.Stderr, os.Environ())
	if opts.Plain {
		out.Profile = colorprofile.NoTTY
		errw.Profile = colorprofile.NoTTY
	}
	u := &UI{opts: opts, in: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, out: out, err: errw, glyph: pick()}
	if !opts.JSON {
		switch os.Getenv(nestEnv) {
		case "":
		case "closed":
			u.printed = true
		default:
			u.nested = true
		}
	}
	return u
}

func (u *UI) JSON() bool  { return u.opts.JSON }
func (u *UI) Yes() bool   { return u.opts.Yes }
func (u *UI) Plain() bool { return u.opts.Plain }

// Trap closes the tree as interrupted on SIGINT or SIGTERM, for commands without a signal-aware context.
func (u *UI) Trap() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		u.Error("interrupted")
		os.Exit(130)
	}()
}

// Env carries the transcript into a child process whose environment is reset, such as under sudo.
// It flushes pending rows first, and afterwards the UI assumes the child may have drawn trees.
func (u *UI) Env() []string {
	if u.opts.JSON {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.flush(false)
	state := u.state()
	u.printed = true
	u.sync()
	if state == "" {
		return nil
	}
	return []string{nestEnv + "=" + state}
}

func (u *UI) Can() bool {
	return !u.opts.JSON && term.IsTerminal(int(u.in.Fd())) && term.IsTerminal(int(u.stdout.Fd()))
}

func (u *UI) Emit(v any) error {
	enc := json.NewEncoder(u.stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Open names the command tree; its header prints only when the first row outside a section needs it.
// A nested process continues its parent's tree instead.
func (u *UI) Open(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.title = line(format, args)
}

// Close ends the command with a summary row: it closes open sections, then the drawn command tree.
// With no sections or drawn tree, OK prints no summary; nested failures stay in the parent's tree.
func (u *UI) Close(level Level, format string, args ...any) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.finish(u.out, level, line(format, args))
}

func (u *UI) finish(w io.Writer, level Level, msg string) {
	defer u.sync()
	sections := len(u.trees) > 0
	for len(u.trees) > 0 {
		note := ""
		if len(u.trees) == 1 && !u.drawn {
			note = msg
		}
		u.end(w, level, note)
	}
	switch {
	case u.drawn:
		u.flush(false)
		u.closing(w, level, msg)
		u.drawn = false
	case sections:
		u.flush(false)
	case u.nested:
		if level == Err {
			u.flush(false)
			u.print(w, u.render(&child{level: level, pill: true, msg: inline(msg)}, false, true))
		}
		u.flush(false)
	case level == OK:
		u.flush(false)
	default:
		u.flush(false)
		u.gap(w)
		if u.title != "" {
			u.print(w, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.open), pill(Info), inline(u.title)))
		}
		u.closing(w, level, msg)
	}
}

func (u *UI) closing(w io.Writer, level Level, msg string) {
	head, rest, _ := strings.Cut(msg, "\n")
	u.print(w, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.close), pill(level), inline(head)))
	for l := range strings.Lines(rest) {
		u.print(w, strings.Repeat(" ", 10)+inline(strings.TrimRight(l, "\n"))+"\n")
	}
}

// Line prints a heading without opening a section or drawing the command tree.
func (u *UI) Line(level Level, title, note string) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	head := styleTarget.Render(inline(title))
	if note != "" {
		head += "  " + styleDim.Render(note)
	}
	glyph := u.glyph.branch
	if u.rail() {
		u.flush(false)
	} else {
		u.gap(u.out)
		glyph = "──"
	}
	u.print(u.out, fmt.Sprintf("%s %s %s\n", styleStep.Render(glyph), pill(level), head))
	u.sync()
}

func (u *UI) Begin(title, note string) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	head := styleTarget.Render(inline(title))
	if note != "" {
		head += "  " + styleDim.Render(note)
	}
	t := tree{title: title, start: time.Now(), flat: u.rail()}
	if t.flat {
		u.flush(false)
		u.print(u.out, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.branch), pill(Info), head))
	} else {
		u.gap(u.out)
		u.print(u.out, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.open), pill(Info), head))
	}
	u.trees = append(u.trees, t)
	u.sync()
}

func (u *UI) End(level Level, format string, args ...any) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.trees) == 0 {
		return
	}
	u.end(u.out, level, line(format, args))
	u.sync()
}

func (u *UI) end(w io.Writer, level Level, msg string) {
	t := u.trees[len(u.trees)-1]
	u.trees = u.trees[:len(u.trees)-1]
	u.flush(false)
	head := pill(level) + " " + styleTarget.Render(inline(t.title))
	if e := elapsed(time.Since(t.start)); e != "" {
		head += " " + styleDim.Render("· "+e)
	}
	if t.flat {
		u.print(w, fmt.Sprintf("%s %s\n", styleStep.Render(u.glyph.branch), head))
		u.pending = note(level, msg)
		return
	}
	u.print(w, fmt.Sprintf("%s %s\n", styleStep.Render(u.glyph.close), head))
	if c := note(level, msg); c != nil {
		u.print(w, u.render(c, false, false))
	}
}

// Node heads a branch, such as a step or an inspected item; the dim note shares its row.
func (u *UI) Node(level Level, title, note string) {
	if u.opts.JSON {
		return
	}
	msg := styleTarget.Render(inline(title))
	if note != "" {
		msg += "  " + styleDim.Render(note)
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ensure()
	u.flush(false)
	u.print(u.out, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.branch), pill(level), msg))
}

func (u *UI) Section(title, note string) { u.Node(Info, title, note) }

func (u *UI) Info(format string, args ...any) { u.Row(Info, line(format, args)) }
func (u *UI) OK(format string, args ...any)   { u.Row(OK, line(format, args)) }
func (u *UI) Warn(format string, args ...any) { u.Row(Warn, line(format, args)) }

// Row reports a result under the current branch.
// It prints when the next event shows whether a sibling follows, so connectors stay joined.
func (u *UI) Row(level Level, msg string) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ensure()
	u.flush(true)
	u.pending = &child{level: level, pill: true, msg: inline(strings.TrimSpace(msg))}
}

// Error closes open sections and the drawn tree with the failure; a nested process reports it as a result row.
func (u *UI) Error(format string, args ...any) {
	msg := line(format, args)
	if u.opts.JSON {
		enc := json.NewEncoder(u.err)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}{"error", msg})
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.finish(u.err, Err, msg)
}

// Detail adds supporting lines under the current row.
func (u *UI) Detail(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ensure()
	for l := range strings.Lines(line(format, args)) {
		l = inline(strings.TrimRight(l, "\n"))
		if u.pending != nil {
			u.pending.details = append(u.pending.details, l)
			continue
		}
		u.print(u.out, fmt.Sprintf("%s%s %s\n", u.pad(), styleDim.Render(">"), l))
	}
}

// KV is a detail with an aligned key.
func (u *UI) KV(key string, value any) {
	u.Detail("%s %v", styleDim.Render(fmt.Sprintf("%-18s", key)), value)
}

func (u *UI) rail() bool {
	return u.nested || u.drawn || len(u.trees) > 0
}

func (u *UI) ensure() {
	if u.rail() || u.title == "" {
		return
	}
	u.gap(u.out)
	u.print(u.out, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.open), pill(Info), inline(u.title)))
	u.drawn = true
	u.sync()
}

func (u *UI) gap(w io.Writer) {
	if u.printed {
		u.print(w, "\n")
	}
}

func (u *UI) print(w io.Writer, s string) {
	fmt.Fprint(w, s)
	u.printed = true
}

func (u *UI) state() string {
	switch {
	case u.rail():
		return "open"
	case u.printed:
		return "closed"
	}
	return ""
}

func (u *UI) sync() {
	if s := u.state(); s != "" {
		os.Setenv(nestEnv, s)
		return
	}
	os.Unsetenv(nestEnv)
}

func (u *UI) flush(more bool) {
	c := u.pending
	if c == nil {
		return
	}
	u.pending = nil
	u.print(u.out, u.render(c, more, true))
}

func (u *UI) render(c *child, more, stem bool) string {
	style := connector(c.level)
	rail := style.Render(u.glyph.stem)
	if !stem {
		rail = " "
	}
	arrow, mid := u.glyph.arrow, " "
	if more {
		arrow, mid = u.glyph.tee, style.Render(u.glyph.stem)
	}
	indent := 10
	head := rail + "     " + style.Render(arrow) + " "
	if c.pill {
		head += pill(c.level) + " "
		indent += 7
	}
	var b strings.Builder
	for i, row := range u.fold(c.msg, indent) {
		if i > 0 {
			head = rail + "     " + mid + strings.Repeat(" ", indent-7)
		}
		b.WriteString(head + row + "\n")
	}
	for _, d := range c.details {
		for i, row := range u.fold(d, 13) {
			lead := styleDim.Render(">") + " "
			if i > 0 {
				lead = "  "
			}
			b.WriteString(rail + "     " + mid + "    " + lead + row + "\n")
		}
	}
	return b.String()
}

func (u *UI) fold(s string, indent int) []string {
	w, _, err := term.GetSize(int(u.stdout.Fd()))
	if err != nil || w < indent+20 {
		return []string{s}
	}
	return strings.Split(ansi.Wrap(s, w-indent, ""), "\n")
}

func note(level Level, msg string) *child {
	if msg == "" {
		return nil
	}
	lines := strings.Split(msg, "\n")
	c := &child{level: level, msg: inline(lines[0])}
	for _, l := range lines[1:] {
		c.details = append(c.details, inline(l))
	}
	return c
}

func (u *UI) lead() string {
	return styleStep.Render(u.glyph.stem) + "     " + styleStep.Render(u.glyph.arrow) + " "
}

func (u *UI) pad() string {
	return styleStep.Render(u.glyph.stem) + strings.Repeat(" ", 10)
}

type glyphs struct{ open, branch, stem, arrow, tee, close string }

// pick falls back to code page 437 glyphs on the Linux console, whose built-in font lacks arcs and ▶.
func pick() glyphs {
	if os.Getenv("TERM") == "linux" {
		return glyphs{"┌─", "├─", "│", "└─►", "├─►", "└─"}
	}
	return glyphs{"╭─", "├─", "│", "╰─▶", "├─▶", "╰─"}
}

func elapsed(d time.Duration) string {
	switch {
	case d < 100*time.Millisecond:
		return ""
	case d < 10*time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return d.Round(time.Second).String()
}

func line(format string, args []any) string {
	return strings.TrimSpace(fmt.Sprintf(format, args...))
}

func inline(msg string) string {
	parts := strings.Split(msg, "`")
	if len(parts) < 3 {
		return msg
	}
	var b strings.Builder
	for i, part := range parts {
		if i%2 == 1 && i < len(parts)-1 {
			b.WriteString(styleCommand.Render(part))
			continue
		}
		if i%2 == 1 {
			b.WriteString("`")
		}
		b.WriteString(part)
	}
	return b.String()
}

var pills = [...]struct {
	label string
	color string
}{
	Info:     {"INFO", "4"},
	OK:       {"OK", "2"},
	Warn:     {"WARN", "3"},
	Err:      {"ERR", "1"},
	command:  {"RUN", "6"},
	ask:      {"ASK", "6"},
	Todo:     {"TODO", "4"},
	Fix:      {"FIX", "3"},
	Later:    {"LATER", "5"},
	Optional: {"OPT", "7"},
	Wait:     {"WAIT", "7"},
}

func pill(level Level) string {
	p := pills[level]
	pad := 6 - len(p.label)
	return lipgloss.NewStyle().
		Background(lipgloss.Color(p.color)).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Render(strings.Repeat(" ", pad/2) + p.label + strings.Repeat(" ", pad-pad/2))
}

func connector(level Level) lipgloss.Style {
	if level == Info {
		return styleStep
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(pills[level].color))
}

var (
	styleStep    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	styleTarget  = lipgloss.NewStyle().Bold(true)
	styleAccent  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	styleCommand = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim     = lipgloss.NewStyle().Faint(true)
)
