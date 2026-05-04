// Package output centralizes dctl human and machine output.
//
// Responsibilities:
// - Render colored or plain human progress.
// - Emit one-shot JSON documents for query-style commands.
// - Keep stdout/stderr routing consistent for command handlers.
package output

// output.go defines the printer contract used by every command package.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Options struct {
	JSON  bool
	Plain bool
}

type Printer struct {
	out   io.Writer
	err   io.Writer
	json  bool
	plain bool
}

type Message struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// New builds a printer for human or JSON output.
//
// NO_COLOR has the same effect as Plain.
func New(out io.Writer, err io.Writer, opts Options) *Printer {
	if out == nil {
		out = os.Stdout
	}
	if err == nil {
		err = os.Stderr
	}
	return &Printer{out: out, err: err, json: opts.JSON, plain: opts.Plain || os.Getenv("NO_COLOR") != ""}
}

func (p *Printer) JSONMode() bool    { return p.json }
func (p *Printer) Writer() io.Writer { return p.out }

// Emit writes exactly one JSON document to stdout.
func (p *Printer) Emit(v any) error {
	enc := json.NewEncoder(p.out)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func (p *Printer) Header(format string, args ...any) {
	p.line("header", fmt.Sprintf(format, args...), false)
}
func (p *Printer) Step(format string, args ...any) { p.step("step", fmt.Sprintf(format, args...)) }
func (p *Printer) SubStep(level string, format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if p.json {
		return
	}
	fmt.Fprintf(p.out, "       %s %s %s\n", p.renderStep("==>"), p.pill(level), msg)
}
func (p *Printer) Info(format string, args ...any) {
	p.line("info", fmt.Sprintf(format, args...), false)
}
func (p *Printer) OK(format string, args ...any) {
	p.line("success", fmt.Sprintf(format, args...), false)
}
func (p *Printer) Warn(format string, args ...any) {
	p.line("warn", fmt.Sprintf(format, args...), false)
}
func (p *Printer) Error(format string, args ...any) {
	p.line("error", fmt.Sprintf(format, args...), true)
}
func (p *Printer) Dim(format string, args ...any) {
	if p.JSONMode() {
		return
	}
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if p.plain {
		fmt.Fprintf(p.out, "        %s\n", msg)
		return
	}
	fmt.Fprintf(p.out, "        %s\n", styleDim.Render(msg))
}
func (p *Printer) KV(key string, value any) {
	if p.json {
		return
	}
	if p.plain {
		fmt.Fprintf(p.out, "        %-18s %v\n", key, value)
		return
	}
	fmt.Fprintf(p.out, "%s %v\n", styleDim.Render(fmt.Sprintf("        %-18s", key)), value)
}

func (p *Printer) line(level string, message string, stderr bool) {
	msg := strings.TrimSpace(message)
	if p.json {
		if stderr {
			enc := json.NewEncoder(p.err)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(Message{Level: level, Message: msg})
		}
		return
	}
	if level == "header" {
		if p.plain {
			fmt.Fprintf(p.out, "\n--- %s ---\n\n", msg)
		} else {
			fmt.Fprintln(p.out, styleHeader.Render(fmt.Sprintf("\n--- %s ---\n", msg)))
		}
		return
	}
	w := p.out
	if stderr {
		w = p.err
	}
	fmt.Fprintf(w, "  %s  %s\n", p.pill(level), msg)
}

func (p *Printer) step(level string, message string) {
	msg := strings.TrimSpace(message)
	if p.json {
		return
	}
	fmt.Fprintf(p.out, "  %s %s\n", p.renderStep("==>"), msg)
}

func (p *Printer) renderStep(s string) string {
	if p.plain {
		return s
	}
	return styleStep.Render(s)
}

func (p *Printer) pill(level string) string {
	label := "INFO"
	s := styleInfo
	switch level {
	case "success", "ok":
		label, s = "OK", styleOK
	case "warn":
		label, s = "WARN", styleWarn
	case "error":
		label, s = "ERR", styleErr
	}
	label = center(label, 4)
	if p.plain {
		return label
	}
	return s.Render(label)
}

func center(label string, width int) string {
	if len(label) >= width {
		return label
	}
	pad := width - len(label)
	return strings.Repeat(" ", pad/2) + label + strings.Repeat(" ", pad-pad/2)
}
