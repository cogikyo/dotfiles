package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const ruleWidth = 80

type Block struct {
	u      *UI
	start  time.Time
	width  int
	live   bool
	open   bool
	shown  bool
	attach bool
	cr     bool
	buffer []byte
}

func (u *UI) Command(line, reason string) *Block {
	b := &Block{u: u, start: time.Now()}
	if u.opts.JSON {
		return b
	}
	if w, _, err := term.GetSize(int(u.stdout.Fd())); err == nil {
		b.width = w
		b.live = !u.opts.Plain
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.ensure()
	u.flush(false)
	if b.width > 20 {
		line = ansi.Wrap(line, b.width-10, "")
	}
	rows := strings.Split(line, "\n")
	u.print(u.out, fmt.Sprintf("%s %s %s\n", styleStep.Render(u.glyph.branch), pill(command), styleCommand.Render(rows[0])))
	for _, row := range rows[1:] {
		u.print(u.out, styleStep.Render(u.glyph.stem)+strings.Repeat(" ", 9)+styleCommand.Render(row)+"\n")
	}
	if reason != "" {
		u.pending = &child{level: Info, msg: inline(reason)}
	}
	return b
}

func (b *Block) Write(p []byte) (int, error) {
	u := b.u
	if u.opts.JSON {
		return u.stderr.Write(p)
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, c := range p {
		switch c {
		case '\n':
			b.commit()
		case '\r':
			b.cr = true
		default:
			if b.cr {
				b.buffer, b.cr = b.buffer[:0], false
			}
			b.buffer = append(b.buffer, c)
		}
	}
	if b.live && len(b.buffer) > 0 {
		b.redraw()
	}
	return len(p), nil
}

func (b *Block) Attach() (stdout, stderr *os.File) {
	u := b.u
	if u.opts.JSON {
		return u.stderr, u.stderr
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	b.rule()
	b.attach = true
	return u.stdout, u.stderr
}

func (b *Block) End(err error) {
	u := b.u
	if u.opts.JSON {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(b.buffer) > 0 {
		b.commit()
	}
	if b.attach {
		u.print(u.out, "\n")
	}
	if b.open {
		u.print(u.out, u.rule("END OF OUTPUT"))
	}
	u.flush(false)
	level := OK
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled) || errors.Is(err, ErrCanceled):
		level = Warn
	default:
		level = Err
	}
	row := styleStep.Render(u.glyph.branch) + " " + pill(level)
	if e := elapsed(time.Since(b.start)); e != "" {
		row += " " + styleDim.Render(e)
	}
	u.print(u.out, row+"\n")
	if err != nil {
		u.pending = note(level, err.Error())
	}
}

func (b *Block) rule() {
	if b.open {
		return
	}
	b.open = true
	b.u.flush(false)
	b.u.print(b.u.out, b.u.rule("OUTPUT"))
}

func (b *Block) commit() {
	text := clean(b.buffer)
	b.buffer, b.cr = b.buffer[:0], false
	b.rule()
	if b.shown {
		b.u.print(b.u.out, "\r\x1b[K")
		b.shown = false
	}
	if b.width > 2 {
		text = ansi.Hardwrap(text, b.width-2, true)
	}
	for row := range strings.SplitSeq(text, "\n") {
		b.u.print(b.u.out, b.u.margin(row)+"\n")
	}
}

func (b *Block) redraw() {
	text := clean(b.buffer)
	if text == "" {
		return
	}
	b.rule()
	b.u.print(b.u.out, "\r\x1b[K"+b.u.margin(ansi.Truncate(text, b.width-2, "")))
	b.shown = true
}

func (u *UI) margin(row string) string {
	if row == "" {
		return styleStep.Render(u.glyph.stem)
	}
	return styleStep.Render(u.glyph.stem) + " " + row
}

func (u *UI) rule(label string) string {
	label = " " + label + " "
	fill := ruleWidth - 1 - ansi.StringWidth(label)
	left := fill / 2
	return styleStep.Render(strings.TrimSuffix(u.glyph.branch, "─")) +
		styleDim.Render(strings.Repeat("─", left)+label+strings.Repeat("─", fill-left)) + "\n"
}

func clean(raw []byte) string {
	s := ansi.Strip(strings.ToValidUTF8(string(raw), "\uFFFD"))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString(strings.Repeat(" ", 8-ansi.StringWidth(b.String())%8))
		case r < 0x20 || r == 0x7f || r >= 0x80 && r < 0xa0:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
