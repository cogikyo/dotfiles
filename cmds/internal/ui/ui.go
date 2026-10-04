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

type Options struct {
	Context context.Context
	JSON    bool
	Plain   bool
	Yes     bool
	// Nested continues a tree that a parent process opened and will close.
	Nested bool
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
	tree   bool
}

func New(opts Options) *UI {
	out := colorprofile.NewWriter(os.Stdout, os.Environ())
	errw := colorprofile.NewWriter(os.Stderr, os.Environ())
	if opts.Plain {
		out.Profile = colorprofile.NoTTY
		errw.Profile = colorprofile.NoTTY
	}
	return &UI{opts: opts, in: os.Stdin, stdout: os.Stdout, out: out, err: errw, tree: opts.Nested && !opts.JSON}
}

func (u *UI) JSON() bool        { return u.opts.JSON }
func (u *UI) Yes() bool         { return u.opts.Yes }
func (u *UI) Plain() bool       { return u.opts.Plain }
func (u *UI) Tree() bool        { return u.tree }
func (u *UI) Writer() io.Writer { return u.out }

func (u *UI) Can() bool {
	return !u.opts.JSON && term.IsTerminal(int(u.in.Fd())) && term.IsTerminal(int(u.stdout.Fd()))
}

func (u *UI) Emit(v any) error {
	enc := json.NewEncoder(u.stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Open starts a tree: later headers become branches and rows become their results until Close.
func (u *UI) Open(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	u.tree = true
	fmt.Fprintf(u.out, "%s %s %s\n", styleStep.Render("╭─"), pill(Info), inline(line(format, args)))
}

func (u *UI) Close(level Level, format string, args ...any) {
	u.close(u.out, level, line(format, args))
}

func (u *UI) close(w io.Writer, level Level, msg string) {
	if u.opts.JSON || !u.tree {
		return
	}
	fmt.Fprintf(w, "%s %s %s\n", styleStep.Render("╰─"), pill(level), inline(msg))
	u.tree = false
}

func (u *UI) Header(format string, args ...any) {
	u.Section(line(format, args), "")
}

// Section heads a step; in a tree the note shares the branch row.
func (u *UI) Section(title, note string) {
	if u.opts.JSON {
		return
	}
	if !u.tree {
		fmt.Fprintf(u.out, "\n%s\n\n", styleHeader.Render("--- "+title+" ---"))
		if note != "" {
			u.Note("%s", note)
		}
		return
	}
	if note != "" {
		note = "  " + styleDim.Render(note)
	}
	u.Node(Info, styleTarget.Render(title)+note)
}

func (u *UI) Node(level Level, msg string) {
	if u.opts.JSON {
		return
	}
	if !u.tree {
		u.Row(level, msg)
		return
	}
	fmt.Fprintf(u.out, "%s %s %s\n", styleStep.Render("├─"), pill(level), inline(strings.TrimSpace(msg)))
}

func (u *UI) Step(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	if u.tree {
		u.Row(Info, line(format, args))
		return
	}
	fmt.Fprintf(u.out, "  %s %s\n", styleStep.Render("==>"), inline(line(format, args)))
}

func (u *UI) Info(format string, args ...any) { u.Row(Info, line(format, args)) }
func (u *UI) OK(format string, args ...any)   { u.Row(OK, line(format, args)) }
func (u *UI) Warn(format string, args ...any) { u.Row(Warn, line(format, args)) }

// Error closes an owned tree with the failure; a nested tree reports it as a result row.
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
	case u.tree && u.opts.Nested:
		u.row(u.err, Err, msg)
	case u.tree:
		u.close(u.err, Err, msg)
	default:
		fmt.Fprintf(u.err, "  %s  %s\n", pill(Err), inline(msg))
	}
}

func (u *UI) Row(level Level, msg string) {
	if u.opts.JSON {
		return
	}
	u.row(u.out, level, msg)
}

func (u *UI) row(w io.Writer, level Level, msg string) {
	msg = inline(strings.TrimSpace(msg))
	if !u.tree {
		fmt.Fprintf(w, "  %s  %s\n", pill(level), msg)
		return
	}
	c := connector(level)
	fmt.Fprintf(w, "%s     %s %s %s\n", c.Render("│"), c.Render("╰─▶"), pill(level), msg)
}

func (u *UI) Dim(format string, args ...any) {
	u.text(styleDim.Render(line(format, args)), "        ")
}

func (u *UI) Detail(text string) {
	if u.opts.JSON {
		return
	}
	marker := ""
	if u.tree {
		marker = styleDim.Render(">") + " "
	}
	for l := range strings.Lines(strings.TrimSpace(text)) {
		u.text(marker+strings.TrimSuffix(l, "\n"), "        ")
	}
}

func (u *UI) Hint(format string, args ...any) {
	u.text(inline(line(format, args)), "  ")
}

func (u *UI) Note(format string, args ...any) {
	u.text(styleDim.Render(line(format, args)), "  ")
}

func (u *UI) KV(key string, value any) {
	if u.tree {
		u.text(fmt.Sprintf("%s %v", styleDim.Render(fmt.Sprintf("%-18s", key)), value), "")
		return
	}
	u.text(fmt.Sprintf("%s %v", styleDim.Render(fmt.Sprintf("        %-18s", key)), value), "")
}

func (u *UI) text(s, indent string) {
	if u.opts.JSON {
		return
	}
	if u.tree {
		indent = u.pad()
	}
	fmt.Fprintf(u.out, "%s%s\n", indent, s)
}

func (u *UI) lead() string {
	if !u.tree {
		return ""
	}
	return styleStep.Render("│") + "     " + styleStep.Render("╰─▶") + " "
}

func (u *UI) pad() string {
	if !u.tree {
		return ""
	}
	return styleStep.Render("│") + strings.Repeat(" ", 10)
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
	styleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	styleTarget  = lipgloss.NewStyle().Bold(true)
	styleAccent  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	styleCommand = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleDim     = lipgloss.NewStyle().Faint(true)
)
