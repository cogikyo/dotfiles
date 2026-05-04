package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPlainOutputHasNoANSI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	out := New(&stdout, &stderr, Options{Plain: true})

	out.OK("ready")
	out.KV("root", "/tmp/dotfiles")
	out.Step("link")

	got := stdout.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("plain output contains ANSI escape: %q", got)
	}
	for _, want := range []string{"OK", "ready", "root", "/tmp/dotfiles", "==> link"} {
		if !strings.Contains(got, want) {
			t.Fatalf("plain output missing %q in %q", want, got)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestJSONSuppressesIncidentalLogs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	out := New(&stdout, &stderr, Options{JSON: true})

	out.Info("ignored")
	out.KV("ignored", true)
	if err := out.Emit(map[string]string{"status": "ok"}); err != nil {
		t.Fatal(err)
	}

	var doc map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %q: %v", stdout.String(), err)
	}
	if doc["status"] != "ok" {
		t.Fatalf("unexpected document: %#v", doc)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
