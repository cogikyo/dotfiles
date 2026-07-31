package main

import "testing"

func TestDaemonResponseFailed(t *testing.T) {
	tests := []struct {
		name string
		resp string
		want bool
	}{
		{name: "daemon error", resp: "error: no focused kitty", want: true},
		{name: "success", resp: "ok", want: false},
		{name: "embedded error text", resp: "ok: error: ignored", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daemonResponseFailed(tt.resp); got != tt.want {
				t.Fatalf("daemonResponseFailed(%q) = %v, want %v", tt.resp, got, tt.want)
			}
		})
	}
}
