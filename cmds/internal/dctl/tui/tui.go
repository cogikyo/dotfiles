// Package tui provides the small terminal launcher used by dctl commands.
//
// Responsibilities:
// - Render command catalogs as keyboard-navigable menus.
// - Return argv fragments without executing commands directly.
package tui

// tui.go defines the launcher model, key handling, rendering, and terminal raw-mode loop.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

type CommandCatalog = CommandNode

type CommandNode struct {
	Name       string
	Help       string
	GroupKey   string
	GroupTitle string
	Children   []CommandNode
}

func (n CommandNode) IsGroup() bool { return len(n.Children) > 0 }

type LauncherModel struct {
	root   CommandNode
	stack  []CommandNode
	cursor []int
	Chosen []string
}

func NewLauncher(root CommandNode) *LauncherModel {
	return &LauncherModel{root: root, stack: []CommandNode{root}, cursor: []int{0}}
}

func RunLauncher(w io.Writer, root CommandCatalog) ([]string, bool, error) {
	m := NewLauncher(root)
	if err := m.run(w); err != nil {
		return nil, false, err
	}
	if len(m.Chosen) == 0 {
		return nil, false, nil
	}
	return m.Chosen, true, nil
}

func (m *LauncherModel) run(w io.Writer) error {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)
	fmt.Fprint(w, "\x1b[?25l")
	defer fmt.Fprint(w, "\x1b[?25h")

	buf := make([]byte, 8)
	for {
		fmt.Fprint(w, "\x1b[H\x1b[2J")
		fmt.Fprint(w, rawScreen(m.Render()))
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return err
		}
		if done := m.handleKey(string(buf[:n])); done {
			fmt.Fprint(w, "\x1b[H\x1b[2J\r")
			return nil
		}
	}
}

func rawScreen(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func (m *LauncherModel) handleKey(key string) bool {
	switch key {
	case "\x03", "q":
		return true
	case "\x1b", "\x1b[D", "h":
		if len(m.stack) == 1 {
			return true
		}
		m.stack = m.stack[:len(m.stack)-1]
		m.cursor = m.cursor[:len(m.cursor)-1]
	case "\x1b[A", "k":
		m.move(-1)
	case "\x1b[B", "j":
		m.move(1)
	case "\r", "\n", "\x1b[C", "l":
		if done := m.selectCurrent(); done {
			return true
		}
	}
	return false
}

func (m *LauncherModel) move(delta int) {
	children := m.current().Children
	if len(children) == 0 {
		return
	}
	i := m.cursor[len(m.cursor)-1] + delta
	if i < 0 {
		i = 0
	}
	if i >= len(children) {
		i = len(children) - 1
	}
	m.cursor[len(m.cursor)-1] = i
}

func (m *LauncherModel) selectCurrent() bool {
	children := m.current().Children
	if len(children) == 0 {
		return false
	}
	i := m.cursor[len(m.cursor)-1]
	if i < 0 || i >= len(children) {
		return false
	}
	child := children[i]
	if child.IsGroup() {
		m.stack = append(m.stack, child)
		m.cursor = append(m.cursor, 0)
		return false
	}
	m.Chosen = append(m.path(), child.Name)
	return true
}

func (m *LauncherModel) current() CommandNode {
	return m.stack[len(m.stack)-1]
}

func (m *LauncherModel) path() []string {
	path := make([]string, 0, len(m.stack))
	for _, node := range m.stack[1:] {
		path = append(path, node.Name)
	}
	return path
}

func (m *LauncherModel) Render() string {
	if len(m.Chosen) > 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", titleStyle.Render("dctl"))
	fmt.Fprintf(&b, "%s\n\n", dimStyle.Render("dotfiles control plane"))

	title := "dctl"
	if path := m.path(); len(path) > 0 {
		title += " " + strings.Join(path, " ")
	}
	fmt.Fprintf(&b, "%s\n\n", stepStyle.Render(title))

	children := m.current().Children
	if len(children) == 0 {
		fmt.Fprintf(&b, "    %s\n\n", dimStyle.Render("(no commands)"))
		b.WriteString(keyHints())
		return b.String()
	}
	maxLabel := 0
	for _, child := range children {
		label := displayLabel(child)
		if len(label) > maxLabel {
			maxLabel = len(label)
		}
	}
	prevGroup := ""
	for i, child := range children {
		if child.GroupKey != prevGroup {
			if child.GroupKey != "" {
				if i > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(&b, "    %s\n", sectionStyle.Render(strings.ToUpper(child.GroupTitle)))
			}
			prevGroup = child.GroupKey
		}
		label := displayLabel(child)
		padded := label + strings.Repeat(" ", maxLabel-len(label))
		hint := strings.TrimSpace(child.Help)
		if i == m.cursor[len(m.cursor)-1] {
			fmt.Fprintf(&b, "  %s%s", accentStyle.Render("> "), accentStyle.Render(padded))
			if hint != "" {
				fmt.Fprintf(&b, "  %s", hintStyle.Render(hint))
			}
			b.WriteByte('\n')
			continue
		}
		fmt.Fprintf(&b, "    %s", dimStyle.Render(padded))
		if hint != "" {
			fmt.Fprintf(&b, "  %s", dimStyle.Render(hint))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(keyHints())
	return b.String()
}

func displayLabel(node CommandNode) string {
	if node.IsGroup() {
		return node.Name + "/"
	}
	return node.Name
}

func keyHints() string {
	return dimStyle.Render("enter select") + "  " + dimStyle.Render("j/k move") + "  " + dimStyle.Render("esc back") + "  " + dimStyle.Render("q quit") + "\n"
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	stepStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
	hintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("3"))
	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("4")).
			Padding(0, 1)
)
