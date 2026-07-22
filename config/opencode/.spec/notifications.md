# Notifications

OS-level notifications through the window manager daemon fire from V2 sessions with the same event set and per-tab routing as V1, with every event backed by an explicit runtime source instead of an assumed name.

## Event sources

- The desired OS event set is: start, complete, subagent, idle, permission, question, todo-complete, and error.
- Each event is mapped to a runtime event, tool event, status transition, or derived durable state verified on the pinned V2 revision; V1 event names are evidence, never assumed mappings.
- Start, complete, idle, permission, question, and error use verified runtime sources; subagent completion derives from native child-session completion with parent linkage, and todo-complete derives from observed todo state where V2 has no dedicated event.
- An event without a verified source blocks notification parity rather than disappearing silently.
- A background child's completion raises its OS notification even though the chat result arrives natively; OS delivery never depends on in-chat mechanics.
- The daemon keeps owning presentation: styles, sounds, and urgency follow its existing configuration, and the plugin only emits events.

## Tab routing

- Every event targets the Kitty tab owning the event's session; child session events follow their parent chain to the parent's tab.
- The context writer keeps publishing tab ownership with the same constraints: runtime directory location, owner-only permissions, staleness pruning, and dead socket cleanup.
- Idle reminders fire only against fresh context.
- Permission and question toasts dedupe within the existing short window.

## Failure behavior

- An absent daemon socket means sessions proceed normally without notifications; delivery failure never blocks or errors a chat.

## Acceptance

- Each desired event is triggered once from a V2 session and observed with correct styling on the correct tab, including derived todo completion and a background child completion routed to its parent's tab while that tab is unfocused.
