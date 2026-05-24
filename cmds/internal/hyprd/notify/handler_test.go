package notify

import (
	"dotfiles/cmds/internal/config"
	"strconv"
	"testing"
)

func TestReplacementNotificationIDUsesEventGroup(t *testing.T) {
	ctx := &kittyContext{PID: 1234, WindowID: 7}

	complete := replacementNotificationID(ctx, "complete")
	idle := replacementNotificationID(ctx, "idle")

	if complete == 0 || idle == 0 {
		t.Fatalf("replacement IDs should be set for valid kitty context: complete=%d idle=%d", complete, idle)
	}
	if complete == idle {
		t.Fatalf("complete and idle replacement IDs collided: %d", complete)
	}
	if got := replacementNotificationID(ctx, "complete"); got != complete {
		t.Fatalf("complete replacement ID should be stable: got %d, want %d", got, complete)
	}
}

func TestDunstArgsUseTimeoutDrivenStickiness(t *testing.T) {
	n := &Notifier{cfg: &config.HyprConfig{Notify: config.NotifyConfig{
		AgentEvents: map[string]config.AgentEvent{
			"complete": {Timeout: 0},
			"idle":     {Timeout: 5000},
		},
	}}}
	ctx := &kittyContext{PID: 1234, WindowID: 7}

	completeArgs := n.buildDunstArgs(notificationSpec{
		Title:       "done",
		Style:       "complete",
		FocusAction: true,
	}, ctx, n.style("complete"))
	idleArgs := n.buildDunstArgs(notificationSpec{
		Title:       "waiting",
		Style:       "idle",
		FocusAction: true,
	}, ctx, n.style("idle"))

	completeID := strconv.Itoa(replacementNotificationID(ctx, "complete"))
	idleID := strconv.Itoa(replacementNotificationID(ctx, "idle"))

	assertFlagValue(t, completeArgs, "-t", "0")
	assertFlagValue(t, completeArgs, "-r", completeID)
	assertFlagValue(t, idleArgs, "-t", "5000")
	assertFlagValue(t, idleArgs, "-r", idleID)

	if hasFlagValue(idleArgs, "-r", completeID) {
		t.Fatalf("timed idle notification used sticky complete replacement ID: %v", idleArgs)
	}
}

func assertFlagValue(t *testing.T, args []string, flag, want string) {
	t.Helper()
	if !hasFlagValue(args, flag, want) {
		t.Fatalf("missing %s %s in args: %v", flag, want, args)
	}
}

func hasFlagValue(args []string, flag, want string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == want {
			return true
		}
	}
	return false
}
