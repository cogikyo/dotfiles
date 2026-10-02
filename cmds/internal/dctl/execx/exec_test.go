package execx

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestOutputError(t *testing.T) {
	_, err := OSRunner{}.Output(t.Context(), "", "sh", "-c", `for i in $(seq 30); do echo "line $i" >&2; done; exit 3`)
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() != 3 {
		t.Fatalf("err = %v, want *exec.ExitError with exit 3", err)
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "sh -c for i") || !strings.Contains(msg, "exit status 3: line 11\n") || !strings.HasSuffix(msg, "line 30") || strings.Contains(msg, "line 10\n") {
		t.Fatalf("message lacks the command, exit code, or 20-line stderr tail:\n%s", msg)
	}
}
