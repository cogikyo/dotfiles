# Five tabs

The daily workflow lives in five named Kitty tabs (scout, build, drive, plan, learn), each an independent full TUI process attached to one shared backend, and the migration preserves that topology exactly.

## Topology

- Each project workspace runs five tabs, each a separate full TUI process attached to the shared backend over loopback.
- Tabs are independently switchable, closable, and relaunchable; no tab's lifecycle depends on another.
- Tab names are launcher concerns only; backend and clients know nothing about them.
- Every tab launches with the project directory as its working context, matching the current launcher behavior.
- Process-name detection, tab styling, restart commands, and session-context tracking all recognize the V2 client without changing the five-tab interaction model.

## Client behavior

- Any tab can host any primary mode; nothing binds a mode to a tab name.
- All tabs share one session surface: a session started in one tab is continuable in any other.
- Launch commands live in the repository, so each branch owns the client invocation for its own runtime.

## Failure behavior

- A tab that loses its backend either reconnects cleanly or exits with an explicit message; a silently stale tab is a defect.
- After a backend restart, a surviving tab reconnects and rehydrates durable session state; volatile events missed during disconnection never masquerade as durable state.

## Acceptance

- Five tabs launch from the workspace definition on the migration branch, each attaching an independent TUI to the same project session surface.
