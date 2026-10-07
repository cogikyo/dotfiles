package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

func TestISOHead(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, argv := range [][]string{
		{"iso", "test", "--head"},
		{"iso", "test", "--dctl", exe, "--bundle", exe},
		{"iso", "build", "--fresh"},
	} {
		var c CLI
		parser, err := kong.New(&c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.Parse(argv); err != nil {
			t.Fatalf("%q: %v", argv, err)
		}
	}
	for _, flag := range []string{"--dctl", "--bundle"} {
		var c CLI
		parser, err := kong.New(&c)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parser.Parse([]string{"iso", "test", "--head", flag, exe})
		if err == nil || !strings.Contains(err.Error(), "can't be used together") {
			t.Fatalf("--head with %s: %v", flag, err)
		}
	}
}
