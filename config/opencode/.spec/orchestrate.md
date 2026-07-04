# Orchestrate: three primaries, one hop, durable specs

Migration complete; all phases committed (`d0549529`, `6c121ec2`, `89cdba0d`, `c2f4b954`).
All remaining executable work lives in the `.spec/delegate.md` runbook; this doc closes with it.

## Settled this session

- Smoke-testing the new fleet is settled as natural usage (user decision); no dedicated smoke pass.
- Builtin build/plan agents disabled in `opencode.json` (`c2f4b954`).
- `default_agent: collab` validated by the live loader.
- Refusal policy implemented in the delegate plugin (see `.spec/delegate.md` State); `plugins/delegate/session.ts` is committed at handoff.

## Watch during natural usage

- Scheme's pattern-object edit permission and leaf envelope inheritance semantics are asserted but unverified.

## Decisions log (durable)

- Three primaries (`scheme`, `collab`, `drive`) replace five modes; coordinators and middle managers retired permanently: they made delegation unobservable and added lossy relay hops.
- One-hop invariant; leaves never delegate.
- Forked sessions over nested subagents for big parallel work; only collab forks, with user confirmation per spawn; drive and scheme never fork.
- Sequential unattended on the shared tree; parallel forked drives only when the human referees; no git worktrees.
- `.spec/` is directory-scoped, committed by default, bound by the shrink contract (ΔS < 0; entropy exports to git history); delete when next steps is empty.
- Refusal policy: sessions are cattle, `.spec/` is the pedigree; never resume a refusal-tainted session; reword the brief first, switch provider last; primary-session recovery stays manual until encountered.
- Open tools, focused instructions; distinct permission envelopes only on primaries (scheme writes `.spec/` files only; collab prompts on the risky tail; drive auto-allows within bounds and denies the irreversible tail).
- Delegate plugin `{model, effort}` per-call routing and model affinity guidance carry forward.
- Agent self-modification routes through `scribe/agents` on explicit user approval; primaries never edit their own prompts.
- Drive rhythm: scout → build → review → scribe → commit, landing clean atomic commits continuously.
- Canalization workflow: `build/proto` variation → `review/architect` selection → `build/canal` inheritance.
- `scribe/commit` kept above the leaf budget deliberately: cheap models need the commit craft verbatim.

## Next steps

1. Execute the `.spec/delegate.md` runbook; nothing runs from this doc.
2. When that runbook completes, delete both specs as commits, surfacing the queued items in the handoff.

## Queued for user (record only, do not do)

- Shared-doctrine triplication across the three primaries (~90 lines × 3) and the 24× leaf-contract duplication want a sync ritual through `scribe/agents`.
