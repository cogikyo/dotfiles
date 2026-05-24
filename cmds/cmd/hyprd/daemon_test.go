package main

import "testing"

func TestParseTabArg(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantName string
		wantPath string
	}{
		{name: "plain tab", arg: "nvim", wantName: "nvim"},
		{name: "path", arg: "nvim -- /tmp/example.md", wantName: "nvim", wantPath: "/tmp/example.md"},
		{name: "alias with path", arg: "nvim::fe-nvim -- /tmp/a path.md", wantName: "nvim::fe-nvim", wantPath: "/tmp/a path.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotPath := parseTabArg(tt.arg)
			if gotName != tt.wantName || gotPath != tt.wantPath {
				t.Fatalf("parseTabArg(%q) = (%q, %q), want (%q, %q)", tt.arg, gotName, gotPath, tt.wantName, tt.wantPath)
			}
		})
	}
}

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
