package ui

import (
	"context"
	"errors"
	"fmt"
	"io"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (u *UI) Spin(ctx context.Context, label string, work func(context.Context) error) error {
	if !u.Can() || u.opts.Plain {
		u.Step("%s", label)
		return work(ctx)
	}
	return spin(ctx, u.stdout, label, work)
}

func spin(parent context.Context, out io.Writer, label string, work func(context.Context) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = work(ctx)
	}()
	m := &busy{label: label, done: done, spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styleStep))}
	_, runErr := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(out)).Run()
	cancel()
	<-done
	switch {
	case parent.Err() != nil:
		return parent.Err()
	case errors.Is(runErr, tea.ErrInterrupted):
		return ErrCanceled
	}
	return errors.Join(runErr, err)
}

type doneMsg struct{}

type busy struct {
	label    string
	done     <-chan struct{}
	spinner  spinner.Model
	finished bool
}

func (m *busy) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		<-m.done
		return doneMsg{}
	})
}

func (m *busy) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(doneMsg); ok {
		m.finished = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *busy) View() tea.View {
	if m.finished {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("  %s %s", m.spinner.View(), m.label))
}
