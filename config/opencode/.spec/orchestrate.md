# Orchestrate: three primaries, one hop, durable specs

Status: migration executed; phases 1-6 complete, committed as `d0549529`, `6c121ec2`, `89cdba0d`.
Remaining: opencode restart plus per-group smoke delegation, pending the user.

## Goal / end state

Three primary modes (`scheme`, `collab`, `drive`) and a five-group leaf fleet replaced the five public modes and all middle managers.
Core invariant: every unit of work sits at most one hop from a session a human can step into.
Primaries delegate directly to leaves and synthesize results themselves; leaves never delegate.
The mode flip carries the presence bit: work in scheme or collab, flip the session to drive when stepping away; context stays, the permission envelope flips.

## Fleet

- `scout/`: context, dirty, library.
- `review/`: debug, security, architect, critic, simplify, modernize, profile, test.
- `build/`: worker, proto, canal, test.
- `scribe/`: spec, doc, comment, banner, agents, commit.
- `verify/`: test, web, source.

Leaf roles and focus boundaries live in the leaf files under `config/opencode/agents/`.

## `.spec/` contract

Directory-scoped: place `.spec/` inside the directory that owns the concern; repo root only for genuinely whole-repo concerns.
Committed by default; a repo opts out with one `.gitignore` line.
Docs include: goal/end state, phase partition with file ownership, per-phase status blocks, decisions log and deviations, open questions, condensed next steps.
Specs must shrink over time (ΔS < 0; entropy exports to git history).
Phase-exit duty after commits land: a `scribe/spec` pass summarizes what landed, prunes finished phases, condenses next steps, and deletes the doc when next steps is empty; deletion is a commit too.
Forked sibling sessions coordinate through the spec plus the git tree, stigmergy-style.

## Migration phases

- Phase 1, primaries (`scheme.md`, `collab.md`, `drive.md`, including collab fork flow): done.
- Phase 2, shared doctrine ported into the three primaries: done.
- Phase 3, leaves refactored into the five groups: done.
- Phase 4, coordinators and manager retired, `opencode.json` rewired: done.
- Phase 5, repo `AGENTS.md` references updated: done.
- Phase 6, session-spawn flow: done, folded into phase 1.
- Phase 7, delete dead files, verify, commit: partial; `jq` validation passed and commits landed; restart plus smoke remain.

## Decisions log (durable)

- Three primaries replace five modes; all coordinators and middle managers retired, permanently: they made delegation unobservable and added lossy relay hops.
- One-hop invariant; leaves never delegate.
- Forked sessions over nested subagents for big parallel work; only collab forks, with user confirmation per spawn; drive and scheme never fork.
- Sequential unattended on the shared tree; parallel forked drives only when the human referees; no git worktrees.
- `.spec/` is directory-scoped, committed by default, and bound by the shrink contract above.
- Refusal policy: sessions are cattle, `.spec/` is the pedigree; never resume a refusal-tainted session; reword the brief first, switch provider last; primary-session recovery stays manual until encountered in practice.
- Open tools, focused instructions; distinct permission envelopes only on primaries (scheme writes `.spec/` files only; collab prompts on the risky tail; drive auto-allows within bounds and denies the irreversible tail).
- Delegate plugin `{model, effort}` per-call routing and model affinity guidance carry forward.
- Agent self-modification routes through `scribe/agents` on explicit user approval; primaries never edit their own prompts.
- Drive rhythm: scout → build → review → scribe → commit, landing clean atomic commits continuously; pre-commit scribe polish is distinct from phase-exit spec condensation.
- Canalization workflow: `build/proto` variation → `review/architect` selection → `build/canal` inheritance; the named shape-discovery loop.

## Deviations

- Phase 6 folded into phase 1; `collab.md` is the same file, so the fork flow landed there; documented flow only, no spawn helper built.
- `default_agent: collab` set in `opencode.json`; the key's schema validity is unverified until restart.
- `hidden: true` dropped from all leaves.
- `verify/source`'s ask-ladder moved from permissions into prose.
- `scribe/commit` kept at ~128 lines, above the leaf budget, deliberately: cheap models need the commit craft verbatim.

## Risks for the remaining smoke

- Scheme's pattern-object edit permission is untested against the live loader.
- Leaf envelope inheritance semantics are asserted but unverified.

## Next steps

1. User restarts opencode, then smoke one trivial delegation per leaf group from each primary, watching the two risks above.
2. Follow-on: delegate plugin refusal detection; `.spec/delegate.md` affinity updates.
3. Follow-on maintenance hazard: shared-doctrine triplication across the three primaries (~90 lines × 3) and the 24× leaf-contract duplication want a sync ritual through `scribe/agents`.
