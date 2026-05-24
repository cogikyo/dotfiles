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

	assertFlagValue(t, completeArgs, "-u", "normal")
	assertFlagValue(t, completeArgs, "-t", "0")
	assertFlagValue(t, completeArgs, "-r", completeID)
	assertFlagValue(t, idleArgs, "-u", "critical")
	assertFlagValue(t, idleArgs, "-t", "5000")
	assertFlagValue(t, idleArgs, "-r", idleID)

	if hasFlagValue(idleArgs, "-r", completeID) {
		t.Fatalf("timed idle notification used sticky complete replacement ID: %v", idleArgs)
	}
}

func TestCanDispatchViewedRequiresOnlyPaneIDs(t *testing.T) {
	n := &Notifier{}
	if !n.CanDispatch(NotifyRequest{Source: "opencode", Event: "viewed", KittyPID: 1234, KittyWindowID: 7}) {
		t.Fatal("viewed event with pane ids should dispatch without resolved app context")
	}
	if n.CanDispatch(NotifyRequest{Source: "opencode", Event: "viewed", KittyPID: 1234}) {
		t.Fatal("viewed event without pane id should not dispatch")
	}
}

func TestPaneNotificationCancellationIsTokenScoped(t *testing.T) {
	ctx := &kittyContext{PID: 1234, WindowID: 7}
	registry := &paneNotificationRegistry{
		active:   make(map[paneNotificationKey]uint64),
		canceled: make(map[paneNotificationKey]map[uint64]struct{}),
		ack:      make(map[paneNotificationKey]uint64),
	}

	key1, token1, ok := registry.Begin(ctx, "complete", 0)
	if !ok {
		t.Fatal("expected valid pane notification key")
	}
	key2, token2, ok := registry.Begin(ctx, "complete", 0)
	if !ok {
		t.Fatal("expected valid replacement complete key")
	}
	if key1 != key2 {
		t.Fatalf("same pane/group should reuse key: %v != %v", key1, key2)
	}
	if !registry.Canceled(key1, token1) {
		t.Fatal("superseded complete notification was not canceled")
	}
	if registry.Canceled(key2, token2) {
		t.Fatal("replacement complete notification inherited stale cancellation")
	}

	registry.End(key1, token1)
	if registry.Canceled(key2, token2) {
		t.Fatal("ending old notification canceled replacement token")
	}

	registry.Cancel(ctx, []string{"complete", "idle"})
	if !registry.Canceled(key2, token2) {
		t.Fatal("active complete notification was not canceled")
	}
	idleKey, ok := notificationKey(ctx, "idle")
	if !ok {
		t.Fatal("expected valid idle key")
	}
	if _, ok := registry.canceled[idleKey]; ok {
		t.Fatal("inactive idle notification should not get a sticky cancellation")
	}

	key3, token3, ok := registry.Begin(ctx, "complete", 0)
	if !ok {
		t.Fatal("expected valid second replacement complete key")
	}
	if key2 != key3 {
		t.Fatalf("same pane/group should reuse key: %v != %v", key2, key3)
	}
	if registry.Canceled(key3, token3) {
		t.Fatal("new complete notification inherited canceled predecessor token")
	}
	if !registry.Canceled(key2, token2) {
		t.Fatal("previous complete notification lost cancellation after replacement began")
	}

	registry.End(key2, token2)
	if registry.Canceled(key3, token3) {
		t.Fatal("ending canceled predecessor canceled replacement token")
	}
	registry.Cancel(ctx, []string{"complete"})
	if !registry.Canceled(key3, token3) {
		t.Fatal("replacement complete notification was not canceled")
	}
	registry.End(key3, token3)
	if len(registry.active) != 0 || len(registry.canceled) != 0 {
		t.Fatalf("registry did not clean up: active=%v canceled=%v", registry.active, registry.canceled)
	}
}

func TestPaneNotificationAckSuppressesEarlierBirthOnly(t *testing.T) {
	ctx := &kittyContext{PID: 1234, WindowID: 7}
	registry := &paneNotificationRegistry{
		active:   make(map[paneNotificationKey]uint64),
		canceled: make(map[paneNotificationKey]map[uint64]struct{}),
		ack:      make(map[paneNotificationKey]uint64),
	}

	earlier := registry.Next()
	registry.Cancel(ctx, []string{"complete"})

	key, token, ok := registry.Begin(ctx, "complete", earlier)
	if !ok {
		t.Fatal("expected valid pane notification key")
	}
	if token != earlier {
		t.Fatalf("begin should preserve request birth token: got %d, want %d", token, earlier)
	}
	if !registry.Canceled(key, token) {
		t.Fatal("ack did not suppress notification born before registration")
	}
	registry.End(key, token)

	later := registry.Next()
	key, token, ok = registry.Begin(ctx, "complete", later)
	if !ok {
		t.Fatal("expected valid later pane notification key")
	}
	if registry.Canceled(key, token) {
		t.Fatal("ack permanently suppressed later notification")
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
