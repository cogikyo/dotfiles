---
name: etiology
description: "Use when debugging a failure, investigating restart-only behavior, or considering a guard or retry."
---

# Etiology

Fix the first wrong decision, not where the failure surfaced.
The deciding question is whether the failing state belongs in the system.
Retries, swallowed errors, and clamped values can conceal the same fault.

- Reproduce the failure before editing; inspect the error and state, and instrument what you cannot see.
- Follow causes upstream to the first wrong decision; fix the consumer of allowed state or the producer of forbidden state.
- For restart-only failures, inspect persisted config, caches, locks, and serialized state first.
- Find the same faulty pattern elsewhere within scope and fix those instances too.
- After a deep dig, propose the cheap signal that would have exposed the cause at first look.
- After two failed fixes sharing a belief, stop and load `doubt`.

Defaults, retries, and optional handling belong when the contract requires them.

See: `doubt`, `membrane`.
