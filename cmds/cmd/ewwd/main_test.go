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

func TestParseEwwListenerCounts(t *testing.T) {
	data := []byte(`Num RefCount Protocol Flags Type St Inode Path
one: 00000002 00000000 00010000 0001 01 10 /run/user/1000/eww-server_same
two: 00000002 00000000 00010000 0001 01 11 /run/user/1000/eww-server_same
three: 00000002 00000000 00010000 0001 01 12 /run/user/1000/eww-server_other
connected: 00000002 00000000 00000000 0001 03 13 /run/user/1000/eww-server_same
foreign: 00000002 00000000 00010000 0001 01 14 /run/user/2000/eww-server_same
`)

	got := parseEwwListenerCounts(data, "/run/user/1000")
	if got["/run/user/1000/eww-server_same"] != 2 {
		t.Fatalf("same listener count = %d, want 2", got["/run/user/1000/eww-server_same"])
	}
	if got["/run/user/1000/eww-server_other"] != 1 {
		t.Fatalf("other listener count = %d, want 1", got["/run/user/1000/eww-server_other"])
	}
	if len(got) != 2 {
		t.Fatalf("listener paths = %d, want 2", len(got))
	}
}
