package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

// nestEnv tells a dotfiles command started inside an open tree to continue it instead of opening its own.
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
)

var (
	ErrNoTTY    = errors.New("interactive terminal required")
	ErrCanceled = errors.New("canceled")
)

type UI struct {
	opts   Options
	in     *os.File
	stdout *os.File
	out    io.Writer
	err    io.Writer
	glyph  glyphs
	nested bool
	open   bool
}

func New(opts Options) *UI {
	out := colorprofile.NewWriter(os.Stdout, os.Environ())
	errw := colorprofile.NewWriter(os.Stderr, os.Environ())
	if opts.Plain {
		out.Profile = colorprofile.NoTTY
		errw.Profile = colorprofile.NoTTY
	}
	nested := os.Getenv(nestEnv) != "" && !opts.JSON
	return &UI{opts: opts, in: os.Stdin, stdout: os.Stdout, out: out, err: errw, glyph: pick(), nested: nested, open: nested}
}

func (u *UI) JSON() bool  { return u.opts.JSON }
func (u *UI) Yes() bool   { return u.opts.Yes }
func (u *UI) Plain() bool { return u.opts.Plain }

// Env carries the open tree into a child process whose environment is reset, such as under sudo.
func (u *UI) Env() []string {
	if !u.open {
		return nil
	}
	return []string{nestEnv + "=1"}
}

func (u *UI) Can() bool {
	return !u.opts.JSON && term.IsTerminal(int(u.in.Fd())) && term.IsTerminal(int(u.stdout.Fd()))
}

func (u *UI) Emit(v any) error {
	enc := json.NewEncoder(u.stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Open starts the tree that every later row hangs from; a nested process continues its parent's tree instead.
func (u *UI) Open(format string, args ...any) {
	if u.opts.JSON || u.open {
		return
	}
	u.open = true
	os.Setenv(nestEnv, "1")
	fmt.Fprintf(u.out, "%s %s %s\n", styleStep.Render(u.glyph.open), pill(Info), inline(line(format, args)))
}

// Close ends an owned tree with a summary row; it does nothing once the tree is closed or when nested.
func (u *UI) Close(level Level, format string, args ...any) {
	u.close(u.out, level, line(format, args))
}

func (u *UI) close(w io.Writer, level Level, msg string) {
	if u.opts.JSON || !u.open || u.nested {
		return
	}
	fmt.Fprintf(w, "%s %s %s\n", styleStep.Render(u.glyph.close), pill(level), inline(msg))
	u.open = false
	os.Unsetenv(nestEnv)
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
	fmt.Fprintf(u.out, "%s %s %s\n", styleStep.Render(u.glyph.branch), pill(level), msg)
}

func (u *UI) Section(title, note string) { u.Node(Info, title, note) }

func (u *UI) Info(format string, args ...any) { u.Row(Info, line(format, args)) }
func (u *UI) OK(format string, args ...any)   { u.Row(OK, line(format, args)) }
func (u *UI) Warn(format string, args ...any) { u.Row(Warn, line(format, args)) }

// Row reports a result under the current branch.
func (u *UI) Row(level Level, msg string) {
	if u.opts.JSON {
		return
	}
	u.row(u.out, level, msg)
}

func (u *UI) row(w io.Writer, level Level, msg string) {
	c := connector(level)
	fmt.Fprintf(w, "%s     %s %s %s\n", c.Render(u.glyph.stem), c.Render(u.glyph.arrow), pill(level), inline(strings.TrimSpace(msg)))
}

// Error closes an owned tree with the failure; a nested process reports it as a result row.
func (u *UI) Error(format string, args ...any) {
	msg := line(format, args)
	switch {
	case u.opts.JSON:
		enc := json.NewEncoder(u.err)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}{"error", msg})
	case u.open && !u.nested:
		u.close(u.err, Err, msg)
	default:
		u.row(u.err, Err, msg)
	}
}

// Detail adds supporting lines under the current row.
func (u *UI) Detail(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	for l := range strings.Lines(line(format, args)) {
		fmt.Fprintf(u.out, "%s%s %s\n", u.pad(), styleDim.Render(">"), inline(strings.TrimRight(l, "\n")))
	}
}

// KV is a detail with an aligned key.
func (u *UI) KV(key string, value any) {
	u.Detail("%s %v", styleDim.Render(fmt.Sprintf("%-18s", key)), value)
}

func (u *UI) lead() string {
	return styleStep.Render(u.glyph.stem) + "     " + styleStep.Render(u.glyph.arrow) + " "
}

func (u *UI) pad() string {
	return styleStep.Render(u.glyph.stem) + strings.Repeat(" ", 10)
}

type glyphs struct{ open, branch, stem, arrow, close string }

// pick falls back to code page 437 glyphs on the Linux console, whose built-in font lacks arcs and ▶.
func pick() glyphs {
	if os.Getenv("TERM") == "linux" {
		return glyphs{"┌─", "├─", "│", "└─►", "└─"}
	}
	return glyphs{"╭─", "├─", "│", "╰─▶", "╰─"}
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
	Info: {"INFO", "4"},
	OK:   {" OK ", "2"},
	Warn: {"WARN", "3"},
	Err:  {"ERR ", "1"},
}

func pill(level Level) string {
	p := pills[level]
	return lipgloss.NewStyle().
		Background(lipgloss.Color(p.color)).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Padding(0, 1).
		Render(p.label)
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
