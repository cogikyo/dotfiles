package ui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	m := &confirm{lead: u.lead(), question: question, yes: yes, plain: u.opts.Plain}
	if err := u.run(m, &m.canceled); err != nil {
		return false, err
	}
	return m.yes, nil
}

func (u *UI) Select(title string, options []string, initial int) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("select: no options")
	}
	m := &choose{lead: u.lead(), pad: u.pad(), title: title, options: options, cursor: min(max(initial, 0), len(options)-1)}
	if err := u.run(m, &m.canceled); err != nil {
		return 0, err
	}
	return m.cursor, nil
}

func (u *UI) Checklist(title string, options []string) ([]bool, error) {
	if len(options) == 0 {
		return nil, errors.New("checklist: no options")
	}
	m := &checklist{lead: u.lead(), pad: u.pad(), title: title, options: options, checked: make([]bool, len(options))}
	for i := range m.checked {
		m.checked[i] = true
	}
	if err := u.run(m, &m.canceled); err != nil {
		return nil, err
	}
	return m.checked, nil
}

func (u *UI) Text(label, initial string) (string, error) {
	m := newInput(u.lead(), styleStep.Render(label), false, !u.opts.Plain)
	m.field.SetValue(initial)
	if err := u.run(m, &m.canceled); err != nil {
		return "", err
	}
	return strings.TrimSpace(m.field.Value()), nil
}

func (u *UI) Secret(label string) (string, error) {
	m := newInput(u.lead(), styleStep.Render(label), true, !u.opts.Plain)
	if err := u.run(m, &m.canceled); err != nil {
		return "", err
	}
	return m.field.Value(), nil
}

type prompt interface {
	tea.Model
	answer() string
}

func (u *UI) run(m prompt, canceled *bool) error {
	if !u.Can() {
		return ErrNoTTY
	}
	u.mu.Lock()
	u.ensure()
	u.flush(true)
	u.mu.Unlock()
	opts := []tea.ProgramOption{tea.WithContext(u.opts.Context), tea.WithInput(u.in), tea.WithOutput(u.stdout)}
	if u.opts.Plain {
		opts = append(opts, tea.WithColorProfile(colorprofile.NoTTY))
	}
	_, err := tea.NewProgram(m, opts...).Run()
	if a := m.answer(); a != "" {
		u.mu.Lock()
		u.pending = &child{level: ask, pill: true, msg: a}
		u.mu.Unlock()
	}
	if err != nil {
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
	lead, question string
	yes, plain     bool
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

func (m *confirm) answer() string {
	question := styleStep.Render(m.question)
	switch {
	case m.canceled:
		return question + " " + styleDim.Render("canceled")
	case m.done && m.yes:
		return question + " " + connector(OK).Render("yes")
	case m.done:
		return question + " " + styleDim.Render("no")
	}
	return ""
}

func (m *confirm) View() tea.View {
	if m.done || m.canceled {
		return tea.NewView("")
	}
	yes := toggle("Yes", m.yes, OK, m.plain)
	no := toggle("No", !m.yes, Err, m.plain)
	return tea.NewView(fmt.Sprintf("%s%s  %s %s ", m.lead, styleStep.Render(m.question), yes, no))
}

// toggle marks the selection with brackets too, because plain output drops the pill colors.
func toggle(label string, on bool, level Level, plain bool) string {
	if plain && on {
		return "[" + label + "]"
	}
	style := lipgloss.NewStyle().Padding(0, 1)
	if on {
		return style.Background(lipgloss.Color(pills[level].color)).Foreground(lipgloss.Color("0")).Bold(true).Render(label)
	}
	return style.Background(lipgloss.Color("8")).Foreground(lipgloss.Color("7")).Render(label)
}

type choose struct {
	lead, pad      string
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

func (m *choose) answer() string {
	switch {
	case m.done:
		return styleStep.Render(m.title) + ": " + m.options[m.cursor]
	case m.canceled:
		return styleStep.Render(m.title) + " " + styleDim.Render("canceled")
	}
	return ""
}

func (m *choose) View() tea.View {
	if m.done || m.canceled {
		return tea.NewView("")
	}
	title := m.lead + styleStep.Render(m.title)
	var b strings.Builder
	b.WriteString(title + "\n")
	for i, option := range m.options {
		if i == m.cursor {
			b.WriteString(m.pad + "  " + styleAccent.Render("> "+option) + "\n")
			continue
		}
		b.WriteString(m.pad + "    " + styleDim.Render(option) + "\n")
	}
	return tea.NewView(b.String())
}

type checklist struct {
	lead, pad      string
	title          string
	options        []string
	checked        []bool
	cursor         int
	done, canceled bool
}

func (m *checklist) Init() tea.Cmd { return nil }

func (m *checklist) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case "space":
		m.checked[m.cursor] = !m.checked[m.cursor]
	case "enter":
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *checklist) answer() string {
	title := styleStep.Render(m.title)
	switch {
	case m.canceled:
		return title + " " + styleDim.Render("canceled")
	case m.done:
		n := 0
		for _, on := range m.checked {
			if on {
				n++
			}
		}
		return fmt.Sprintf("%s %d of %d checked", title, n, len(m.options))
	}
	return ""
}

func (m *checklist) View() tea.View {
	if m.done || m.canceled {
		return tea.NewView("")
	}
	title := m.lead + styleStep.Render(m.title)
	var b strings.Builder
	b.WriteString(title + "  " + styleDim.Render("space toggles, enter confirms") + "\n")
	for i, option := range m.options {
		box := "[ ] "
		if m.checked[i] {
			box = "[x] "
		}
		switch {
		case i == m.cursor:
			b.WriteString(m.pad + "  " + styleAccent.Render("> "+box+option) + "\n")
		case m.checked[i]:
			b.WriteString(m.pad + "    " + box + option + "\n")
		default:
			b.WriteString(m.pad + "    " + styleDim.Render(box+option) + "\n")
		}
	}
	return tea.NewView(b.String())
}

type input struct {
	lead, label    string
	secret, blink  bool
	field          textinput.Model
	done, canceled bool
}

func newInput(lead, label string, secret, blink bool) *input {
	field := textinput.New()
	field.Prompt = ""
	styles := textinput.DefaultStyles(true)
	styles.Cursor.Blink = blink
	field.SetStyles(styles)
	if secret {
		field.EchoMode = textinput.EchoNone
	}
	field.Focus()
	return &input{lead: lead, label: label, secret: secret, blink: blink, field: field}
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

func (m *input) answer() string {
	switch {
	case m.canceled:
		return m.label + " " + styleDim.Render("canceled")
	case m.done && !m.secret:
		return m.label + " " + m.field.Value()
	case m.done:
		return m.label
	}
	return ""
}

func (m *input) View() tea.View {
	if m.done || m.canceled {
		return tea.NewView("")
	}
	return tea.NewView(m.lead + m.label + " " + m.field.View())
}
