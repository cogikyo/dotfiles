# Server plugins

Server plugins that survive the migration are ported outright to V2's plugin runtime on the migration branch, and every mechanism the native runtime now owns is deleted instead of emulated.

## Survivors

- Claude subscription auth continues to own Anthropic request shape and credential refresh; nothing else talks to Anthropic directly.
- The usage status tool stays read-only and cache-backed: it never refreshes providers, never mutates chat context, and never logs tokens, cookies, or local paths.
- Spec title, media context prompt, and X tooling retain their current agent-scoped behavior on the V2 runtime; permission-denied defaults remain denied while their authorized specialist consumers continue to work.
- Media handling keeps its boundaries: images only as provider uploads, videos local-only, registry and runtime files owner-readable only.

## Deletions

- The V1 delegate lifecycle implementation is deleted: manual child session creation, polling, wait loops, foreground and background emulation, and result injection all belong to the native runtime.
- The plugin SDK pin moves to the V2-compatible line; no V1-pinned plugin dependency survives in the manifest.

## Persistence boundaries

- A surviving plugin that persists synthetic session data chooses its identity scheme against the V2 data model on the pinned revision; V1 SDK conventions are evidence, never contract.
- Synthetic identities never collide with runtime-generated identities, and persisted plugin data never derives identity from message output fields whose stability is unverified on the pinned revision.
- The plugin tree typechecks against the V2 SDK as a standing gate.

## Acceptance

- The backend starts with the ported set, each ported plugin demonstrates its core behavior once in a live session, and the typecheck gate passes.
