---
name: cutover
description: "Use when tempted to retain an old path, shim, fallback, or compatibility layer."
---

# Cutover

Switch over and delete the old path in the same pass.
An old path kept for convenience becomes another contract to maintain.
A fallback that hides a broken contract also hides the failure that needs fixing.

- Keep an old path only for a named contract or external user, including documented defaults, retries, or optional-feature handling; stop removal while that dependency remains.
- Define the target state and verification points before coordinating a migration.
- Move callers and remove the old API in the same wave, even if delivery takes several steps.
- Permit temporary breakage only when planned, scoped, and reversible; keep focused checks on changed areas.
- Remove stale references and duplicate contracts before calling the change complete.

See: `redesign`, `falsify`.
