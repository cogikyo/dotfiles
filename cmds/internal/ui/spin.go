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
	if u.opts.JSON {
		return work(ctx)
	}
	u.mu.Lock()
	u.ensure()
	u.flush(true)
	live := u.Can() && !u.opts.Plain
	u.mu.Unlock()
	var err error
	if live {
		err = spin(ctx, u.stdout, u.lead(), label, work)
	} else {
		err = work(ctx)
	}
	switch {
	case err == nil:
		u.Row(OK, label)
	case errors.Is(err, ErrCanceled) || errors.Is(err, context.Canceled):
		u.Row(Warn, label)
	default:
		u.Row(Err, label)
	}
	return err
}

func (u *UI) Await(label string, relabel <-chan string, done <-chan struct{}) {
	if u.opts.JSON {
		<-done
		return
	}
	u.mu.Lock()
	u.ensure()
	u.flush(true)
	live := u.Can() && !u.opts.Plain
	u.mu.Unlock()
	if live {
		m := &busy{lead: u.lead(), label: label, done: done, relabel: relabel, spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styleStep))}
		_, _ = tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(u.stdout), tea.WithoutSignalHandler()).Run()
		<-done
		return
	}
	for {
		select {
		case <-done:
			return
		case l := <-relabel:
			u.Row(Warn, l)
		}
	}
}

func spin(parent context.Context, out io.Writer, lead, label string, work func(context.Context) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = work(ctx)
	}()
	m := &busy{lead: lead, label: label, done: done, spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(styleStep))}
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

type labelMsg string

type busy struct {
	lead     string
	label    string
	done     <-chan struct{}
	relabel  <-chan string
	spinner  spinner.Model
	finished bool
}

func (m *busy) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.next, func() tea.Msg {
		<-m.done
		return doneMsg{}
	})
}

func (m *busy) next() tea.Msg {
	select {
	case l := <-m.relabel:
		return labelMsg(l)
	case <-m.done:
		return nil
	}
}

func (m *busy) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case doneMsg:
		m.finished = true
		return m, tea.Quit
	case labelMsg:
		m.label = string(msg)
		return m, m.next
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *busy) View() tea.View {
	if m.finished {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("%s%s %s", m.lead, m.spinner.View(), m.label))
}
