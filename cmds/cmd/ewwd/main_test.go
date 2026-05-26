package main

import "testing"

func TestDaemonResponseFailed(t *testing.T) {
	tests := []struct {
		name string
		resp string
		want bool
	}{
		{name: "plain success", resp: "close: all windows", want: false},
		{name: "leading space error", resp: "  error: eww close-all failed", want: true},
		{name: "newline error", resp: "error: unknown provider\n", want: true},
		{name: "embedded error text", resp: "ok: previous error: ignored", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daemonResponseFailed(tt.resp); got != tt.want {
				t.Fatalf("daemonResponseFailed(%q) = %v, want %v", tt.resp, got, tt.want)
			}
		})
	}
}
