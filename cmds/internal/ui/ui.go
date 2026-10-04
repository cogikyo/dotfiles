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
}

func New(opts Options) *UI {
	out := colorprofile.NewWriter(os.Stdout, os.Environ())
	errw := colorprofile.NewWriter(os.Stderr, os.Environ())
	if opts.Plain {
		out.Profile = colorprofile.NoTTY
		errw.Profile = colorprofile.NoTTY
	}
	return &UI{opts: opts, in: os.Stdin, stdout: os.Stdout, out: out, err: errw}
}

func (u *UI) JSON() bool        { return u.opts.JSON }
func (u *UI) Yes() bool         { return u.opts.Yes }
func (u *UI) Plain() bool       { return u.opts.Plain }
func (u *UI) Writer() io.Writer { return u.out }

func (u *UI) Can() bool {
	return !u.opts.JSON && term.IsTerminal(int(u.in.Fd())) && term.IsTerminal(int(u.stdout.Fd()))
}

func (u *UI) Emit(v any) error {
	enc := json.NewEncoder(u.stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func (u *UI) Header(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "\n%s\n\n", styleHeader.Render("--- "+line(format, args)+" ---"))
}

func (u *UI) Step(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "  %s %s\n", styleStep.Render("==>"), line(format, args))
}

func (u *UI) Info(format string, args ...any) { u.Row(Info, line(format, args)) }
func (u *UI) OK(format string, args ...any)   { u.Row(OK, line(format, args)) }
func (u *UI) Warn(format string, args ...any) { u.Row(Warn, line(format, args)) }

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
	fmt.Fprintf(u.err, "  %s  %s\n", pill(Err), msg)
}

func (u *UI) Row(level Level, msg string) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "  %s  %s\n", pill(level), strings.TrimSpace(msg))
}

func (u *UI) Dim(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "        %s\n", styleDim.Render(line(format, args)))
}

func (u *UI) Detail(text string) {
	if u.opts.JSON {
		return
	}
	for l := range strings.Lines(strings.TrimSpace(text)) {
		fmt.Fprintf(u.out, "        %s\n", strings.TrimSuffix(l, "\n"))
	}
}

func (u *UI) Hint(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "  %s\n", line(format, args))
}

func (u *UI) Note(format string, args ...any) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "  %s\n", styleDim.Render(line(format, args)))
}

func (u *UI) KV(key string, value any) {
	if u.opts.JSON {
		return
	}
	fmt.Fprintf(u.out, "%s %v\n", styleDim.Render(fmt.Sprintf("        %-18s", key)), value)
}

func line(format string, args []any) string {
	return strings.TrimSpace(fmt.Sprintf(format, args...))
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

var (
	styleStep   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	styleAccent = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	styleDim    = lipgloss.NewStyle().Faint(true)
)
