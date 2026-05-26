package session

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/state"
)

type fakeHypr struct {
	dispatches []string
	fail       map[string]error
}

func (f *fakeHypr) Dispatch(args string) error {
	f.dispatches = append(f.dispatches, args)
	if err := f.fail[args]; err != nil {
		return err
	}
	return nil
}

func (f *fakeHypr) Request(command string) ([]byte, error) {
	return []byte(`{"id":3}`), nil
}

func TestPseudoRollsBackWhenSubmapDispatchFails(t *testing.T) {
	s := state.NewState(&config.HyprConfig{})
	s.SetWorkspace(3)
	h := &fakeHypr{fail: map[string]error{"submap pseudolock": errors.New("boom")}}
	l := &Lock{hypr: h, state: s}

	msg, err := l.Pseudo()
	if err == nil {
		t.Fatalf("Pseudo() err = nil, want submap failure")
	}
	if msg != "" {
		t.Fatalf("Pseudo() msg = %q, want empty failure message", msg)
	}
	if !strings.Contains(err.Error(), "enter pseudolock") {
		t.Fatalf("Pseudo() err = %q, want pseudolock context", err)
	}
	if l.saved != nil {
		t.Fatalf("Pseudo() saved lock state after failed required transition")
	}

	want := []string{"workspace 6", "submap pseudolock", "submap reset", "workspace 3"}
	if !reflect.DeepEqual(h.dispatches, want) {
		t.Fatalf("dispatches = %v, want %v", h.dispatches, want)
	}
}

func TestUnlockPropagatesResetFailureAndKeepsState(t *testing.T) {
	s := state.NewState(&config.HyprConfig{})
	h := &fakeHypr{fail: map[string]error{"submap reset": errors.New("boom")}}
	saved := &lockState{workspace: 2, restoreWidgets: true}
	l := &Lock{hypr: h, state: s, saved: saved}

	msg, err := l.Unlock()
	if err == nil {
		t.Fatalf("Unlock() err = nil, want reset failure")
	}
	if msg != "" {
		t.Fatalf("Unlock() msg = %q, want empty failure message", msg)
	}
	if !strings.Contains(err.Error(), "reset submap") {
		t.Fatalf("Unlock() err = %q, want reset context", err)
	}
	if l.saved != saved {
		t.Fatalf("Unlock() discarded saved state after failed reset")
	}

	want := []string{"submap reset"}
	if !reflect.DeepEqual(h.dispatches, want) {
		t.Fatalf("dispatches = %v, want %v", h.dispatches, want)
	}
}
