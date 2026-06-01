package main

import "testing"

func TestEventLoopMonitorEvents(t *testing.T) {
	var got []string
	events := &EventLoop{}
	events.OnMonitorChanged(func(event, data string) {
		got = append(got, event+">>"+data)
	})

	events.handleEvent("monitorremoved>>DP-1")
	events.handleEvent("monitoradded>>DP-1")
	events.handleEvent("monitoraddedv2>>0,DP-1,Samsung")
	events.handleEvent("ignored")

	want := []string{
		"monitorremoved>>DP-1",
		"monitoradded>>DP-1",
		"monitoraddedv2>>0,DP-1,Samsung",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d monitor events, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %q, want %q", i, got[i], want[i])
		}
	}
}
