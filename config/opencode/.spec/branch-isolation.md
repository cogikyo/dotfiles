# Branch isolation

V1 and V2 remain recoverable as separate installations with separate storage, while a dedicated migration branch owns the complete V2 port and the default branch preserves the working V1 setup, so either runtime is one branch switch plus restart away and neither can corrupt the other.

## Branch ownership

- The default branch preserves the working V1 implementation: configuration, plugins, dependency pins, service definition, and launch commands.
- The migration branch owns the V2 port of those same surfaces and carries no dual-runtime compatibility shims.
- No file is required to satisfy both runtimes; the branches may diverge freely.

## Installation

- V1 keeps the canonical binary name; V2 installs under its own distinct name (currently opencode2) and never shadows V1 during the migration.
- V2 runs pinned to one verified beta revision at a time; advancing the pin is a deliberate act on the migration branch, never an automatic update.
- The pinned artifact is the object of truth: published packages carry no commit provenance, so capabilities are proven by probing the running artifact, with source snapshots as evidence only.
- Only capabilities verified present in the pinned revision may carry load in contracts or acceptance criteria.

## Storage isolation

- V2 runs through its pinned beta channel with a V2-only database identity; environment overrides never disable channel isolation or point either runtime at the other's database.
- Shared global state such as service registration never carries session history, and after each switch it resolves to the runtime just started.
- Either branch reopens its own runtime's history after a switch; the other runtime's history is untouched.
- V2 storage is disposable: the beta may reset it without warning, so nothing load-bearing lives only there.
- No session continuity between runtimes is required, and nothing attempts to bridge them.

## Switching

- Because the repository is symlinked live into the active config locations, a checkout instantly changes what any restarted OpenCode process loads.
- A runtime switch is: stop the backend and every attached TUI, check out the other branch, start the backend, launch the clients.
- No OpenCode process outlives a branch switch; a process started before the checkout is stale by definition and is replaced.
- A mixed state, where a running process loads configuration from the wrong branch generation, is the failure mode this boundary exists to prevent.

## Failure behavior

- A V2 crash, storage reset, or bad beta upgrade never changes the default branch, so the V1 workflow remains one switch away.
- V1 behavior is the reference record for every parity question.

## Acceptance

- Switch from the default branch to the migration branch and back with the full restart boundary each time; each runtime reopens its own history, no process from the previous generation survives the switch, and neither database changes while its runtime is inactive.
