package ui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func (u *UI) Confirm(question string) (bool, error) {
	return u.confirm(question, false)
}

func (u *UI) Proceed(question string) (bool, error) {
	return u.confirm(question, true)
}

func (u *UI) confirm(question string, yes bool) (bool, error) {
	if u.opts.Yes {
		return true, nil
	}
	m := &confirm{question: question, yes: yes}
	if err := u.run(m, &m.canceled); err != nil {
		return false, err
	}
	return m.yes, nil
}

func (u *UI) Select(title string, options []string, initial int) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("select: no options")
	}
	m := &choose{title: title, options: options, cursor: min(max(initial, 0), len(options)-1)}
	if err := u.run(m, &m.canceled); err != nil {
		return 0, err
	}
	return m.cursor, nil
}

func (u *UI) Text(label, initial string) (string, error) {
	m := newInput(label, false, !u.opts.Plain)
	m.field.SetValue(initial)
	if err := u.run(m, &m.canceled); err != nil {
		return "", err
	}
	return strings.TrimSpace(m.field.Value()), nil
}

func (u *UI) Secret(label string) (string, error) {
	m := newInput(label, true, !u.opts.Plain)
	if err := u.run(m, &m.canceled); err != nil {
		return "", err
	}
	return m.field.Value(), nil
}

func (u *UI) run(m tea.Model, canceled *bool) error {
	if !u.Can() {
		return ErrNoTTY
	}
	opts := []tea.ProgramOption{tea.WithContext(u.opts.Context), tea.WithInput(u.in), tea.WithOutput(u.stdout)}
	if u.opts.Plain {
		opts = append(opts, tea.WithColorProfile(colorprofile.NoTTY))
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		return err
	}
	if *canceled {
		return ErrCanceled
	}
	return nil
}

func cancelKey(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyPressMsg)
	return ok && (key.String() == "ctrl+c" || key.String() == "esc")
}

type confirm struct {
	question       string
	yes            bool
	done, canceled bool
}

func (m *confirm) Init() tea.Cmd { return nil }

func (m *confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cancelKey(msg) {
		m.canceled = true
		return m, tea.Quit
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "left", "right", "h", "l", "tab":
		m.yes = !m.yes
	case "y", "Y":
		m.yes, m.done = true, true
	case "n", "N":
		m.yes, m.done = false, true
	case "enter":
		m.done = true
	}
	if m.done {
		return m, tea.Quit
	}
	return m, nil
}

func (m *confirm) View() tea.View {
	question := styleStep.Render(m.question)
	if m.done || m.canceled {
		answer := "no"
		if m.yes && !m.canceled {
			answer = "yes"
		}
		return tea.NewView(fmt.Sprintf("%s %s\n", question, answer))
	}
	no, yes := styleAccent.Render("> no"), styleDim.Render("  yes")
	if m.yes {
		no, yes = styleDim.Render("  no"), styleAccent.Render("> yes")
	}
	return tea.NewView(fmt.Sprintf("%s  %s  %s ", question, no, yes))
}

type choose struct {
	title          string
	options        []string
	cursor         int
	done, canceled bool
}

func (m *choose) Init() tea.Cmd { return nil }

func (m *choose) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cancelKey(msg) {
		m.canceled = true
		return m, tea.Quit
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, len(m.options)-1)
	case "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *choose) View() tea.View {
	title := styleStep.Render(m.title)
	if m.done {
		return tea.NewView(fmt.Sprintf("%s %s\n", title, m.options[m.cursor]))
	}
	if m.canceled {
		return tea.NewView(title + "\n")
	}
	var b strings.Builder
	b.WriteString(title + "\n")
	for i, option := range m.options {
		if i == m.cursor {
			b.WriteString("  " + styleAccent.Render("> "+option) + "\n")
			continue
		}
		b.WriteString("    " + styleDim.Render(option) + "\n")
	}
	return tea.NewView(b.String())
}

type input struct {
	label          string
	secret, blink  bool
	field          textinput.Model
	done, canceled bool
}

func newInput(label string, secret, blink bool) *input {
	field := textinput.New()
	field.Prompt = ""
	styles := textinput.DefaultStyles(true)
	styles.Cursor.Blink = blink
	field.SetStyles(styles)
	if secret {
		field.EchoMode = textinput.EchoNone
	}
	field.Focus()
	return &input{label: label, secret: secret, blink: blink, field: field}
}

func (m *input) Init() tea.Cmd {
	if !m.blink {
		return nil
	}
	return textinput.Blink
}

func (m *input) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cancelKey(msg) {
		m.canceled = true
		return m, tea.Quit
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "enter" {
		m.done = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.field, cmd = m.field.Update(msg)
	return m, cmd
}

func (m *input) View() tea.View {
	label := styleStep.Render(m.label) + " "
	switch {
	case m.done && !m.secret:
		return tea.NewView(label + m.field.Value() + "\n")
	case m.done || m.canceled:
		return tea.NewView(label + "\n")
	}
	return tea.NewView(label + m.field.View())
}
