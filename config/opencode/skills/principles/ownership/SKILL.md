---
name: ownership
description: "Use when placing state, coordinating writers, or maintaining synchronized copies."
---

# Ownership

One authority per fact; derive, don't sync.
Shared writes need an enforced owner, while synchronized copies create more places for state to disagree.

- Choose ownership for the context; avoid a universal placement rule.
- Keep local state local and enforce invariants with the owner that can protect them.
- Remove competing authorities in mixed or duplicated state; derive values from their source.
- Track capture and lifetime across UI, database, config, process memory, and derived state.
- Give independent writers separate targets and combine their facts when reading.
- Keep a shared write target only for a real invariant, then enforce access with a single writer, locks, or another concurrency mechanism.

See: `representation`, `converge`.
