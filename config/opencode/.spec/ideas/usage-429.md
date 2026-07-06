# Idea ledger: usage sidebar 429s

Seeded 2026-07-06 after the Anthropic usage row briefly showed `429` while model auth and model calls still worked.
Goal: keep a small investigation note without blocking the continuity cleanup.
End state: the issue is reproduced and fixed, or it is deleted as transient provider-side rate limiting.

## Observation

The Anthropic usage adapter briefly rendered `429` in the usage sidebar.
The user later reported the display recovered without intervention.
Model auth and normal model usage appeared functional during the incident.

## Current conjecture

The private Anthropic usage endpoint was probably rate limited independently from inference.
The existing docs already note a tighter usage endpoint and a fifteen-minute 429 backoff.

## Next checks

1. If it recurs, inspect the usage-sidebar cache entry and timestamp under `${XDG_CACHE_HOME:-~/.cache}/opencode/usage-sidebar/`.
2. Confirm the Anthropic adapter keeps stale successful usage visible during transient 429s.
3. If stale data is discarded on 429, change the adapter to render stale usage with a muted rate-limit marker.

## Non-goals

- Do not touch provider auth while the issue is not reproducing.
- Do not increase polling frequency.
