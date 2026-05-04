package health

// health_test.go covers human and JSON rendering of healthcheck results.

import (
	"bytes"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/output"
)

func TestPrintHumanSummaryAndDetails(t *testing.T) {
	var out, err bytes.Buffer
	printer := output.New(&out, &err, output.Options{Plain: true})
	checks := []Check{
		{ID: "link:zshrc", Name: "zshrc", Status: OK, Observed: "linked"},
		{ID: "dns:resolved", Name: "resolved", Status: Warn, Observed: "inactive", Fix: "systemctl start systemd-resolved", Details: []string{"service inactive"}},
	}

	if err := Print(printer, checks); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"health: 1 ok, 1 warn, 0 fail, 0 skip", "dns:resolved: resolved", "fix", "details"} {
		if !strings.Contains(text, want) {
			t.Fatalf("human output missing %q:\n%s", want, text)
		}
	}
}

func TestPrintJSONStillEmitsChecks(t *testing.T) {
	var out, err bytes.Buffer
	printer := output.New(&out, &err, output.Options{JSON: true})

	if err := Print(printer, []Check{{ID: "go:tool", Name: "go", Status: OK}}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "[") || !strings.Contains(got, `"id":"go:tool"`) {
		t.Fatalf("JSON output is not a check array: %s", got)
	}
}
