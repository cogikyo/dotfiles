# Idea ledger: harness skills and leaves

Seeded 2026-07-05 from the user's request for a durable idea bank feeding future skill/agent planning.
This is a ledger, not an execution spec: it holds candidate skills, candidate leaves, and the selection doctrine that gates them.
Goal: let a future plan writer start skill/leaf work without replaying this session's model sweep and source verification.
End state: candidates are selected by a future plan, rejected, or drained until this file empties and gets deleted.

During the seed session, no agent, config, or code was edited; only this ledger was created.

## Thread ownership

This idea thread owns only this file.
Ledger owner: `scribe/spec` or primaries own this ideas file.
`scribe/agents` may read this ledger, but must not edit `.spec/` docs.
Concurrent continuity or compaction dirty files, if present, are unrelated unless a future parent explicitly links them.

## Evidence (session-verified)

- At the seed session start, the git tree was clean and the branch was ahead of origin by 4.
- Current fleet: primaries `plan`, `build`, `drive`, `learn`; leaf groups scout/build/review/scribe/verify; no checked-in local `SKILL.md` files.
- The closed fleet thread recorded the 26-leaf fleet at its cognitive ceiling; invented leaves were declined unless a gap is felt twice.
- OpenCode v1.17.13 source facts are source-verified at commit `10c894bdeef3618f5666fb506ef7f9491bb964d8`, not run-verified in this repo.
  - Source paths checked: `packages/opencode/src/skill/index.ts`, `packages/opencode/src/tool/skill.ts`, and `packages/core/src/v1/config/skills.ts`.
  - `config/opencode/skills/<name>/SKILL.md` symlinks to `~/.config/opencode/skills/<name>/SKILL.md` and is discovered by default; no `skills.paths` entry needed.
  - `skills.paths` is optional `{ paths: string[], urls: string[] }`; relative paths resolve against the current session directory.
  - Only `name` is source-required, but described skills are the model-visible ones, so a `description` is effectively mandatory for triggering.
  - OpenCode does not auto-run scripts inside skills; sibling files are resources the model may read or invoke, never automatic hooks.
- Web scout: the Agent Skills standard is converging and OpenCode supports it; Claude Code, Roo, Cursor, Kiro, and MCP ecosystems point to progressive disclosure, mode-targeted skills, plugins/hooks for deterministic enforcement, subagent-scoped tools, and run/verify-style skills.
  - Source URLs: `https://opencode.ai/docs/skills/`, `https://agentskills.io/specification`, `https://docs.anthropic.com/en/docs/claude-code/skills`, `https://cursor.com/docs/context/rules`, `https://docs.roocode.com/features/skills`, and `https://docs.claude.com/en/docs/claude-code/mcp`.
- X scout returned nothing usable; X access failed with HTTP 403.

## Conjecture (model sweep, weaker evidence)

- Grok and GLM idea probes generated many mutations; stronger-model selection converged on skills and scripts first, with very few new leaves.
- Model roles observed:
  - Grok: broad and concrete, over-produced taxonomy and script lists.
  - GLM: strongest at simplification and layout/pruning rules; argued skills over leaves for most frontend/design/QA ideas.
  - GPT baseline: conservative; small global skill library, deterministic harness checks, `scout/session` as the one obvious new leaf.
  - Anthropic baseline: framed the loop as variation → selection → inheritance; proposed `idea-council` plus maybe `scout/ideas` as the one ideation leaf.
  - X/XAI `verify/x`: no usable community findings, X unavailable; recorded as a gap, not a signal.

## Decisions log

- Treat Grok and GLM as high-mutation variation engines, not selectors.
- Agreement across cheap models is weak evidence unless backed by independent verification.
- If a skill tranche is approved, prefer a global `config/opencode/skills/` corpus and gate every skill hard because this repo is the live global OpenCode config.
- Ownership split: skills own reusable procedures; scripts own deterministic helpers; leaves own separate context, permission, and model membranes.
- No literal per-agent script directories; skill-local scripts are fine only while one skill owns them, and reusable tools graduate to `cmds/` or plugins.
- Future instruction edits route through `scribe/agents` after explicit approval; the seed session only created the ledger.

## Selection rule

Add a new leaf only when it needs at least one of:

- a distinct permission envelope,
- a separate context window,
- a stable selection surface a primary can target,
- or model/provider as a defining dissent membrane.

Otherwise prefer a skill, script, command, plugin, or model routing.
Use a skill for a reusable procedure, long context ritual, or procedure that multiple agents need.
Do not make a skill when it duplicates one leaf's prompt, the built-in `customize-opencode`, or an existing `AGENTS.md` section without reducing context tax.

## First tranche acceptance criteria

- Explicit user approval is cited before any instruction edit; `scribe/agents` is the editing route.
- A canary skill confirms discovery and triggering after restart as required runtime verification before a corpus lands.
- Each skill has `name` and a sharp `description`.
- Each `description` front-loads trigger keywords and hard-gates global versus dotfiles-only behavior.
- Skill-local scripts have one owner and no automatic execution assumption.
- Reusable scripts graduate to `cmds/` or plugins.
- Each accepted candidate reduces prompt/context tax or closes a real recurring gap.

## Candidate skills (queue, not approval)

1. `idea-council`: status: speculative; same-brief fanout across Grok/GLM/GPT/Anthropic, evidence matrix, disagreement ledger.
2. `agent-craft`: status: overlaps existing; overlaps `scribe/agents` and `customize-opencode`, so narrow before authoring.
3. `leaf-brief`: status: speculative; bounded leaf brief template covering objective, scope, files, context, constraints, verification, traps.
4. `source-truth`: status: speculative; evidence ladder and citation discipline for local behavior, source, docs, community, and model claims.
5. `spec-packet`: status: deferred to `compaction.md`; defers to managed-session packet shape there, with only ΔS<0 reminders possibly reusable.
6. `delegate-routing`: status: speculative; model/effort/pin/capacity route selection, deduping repeated doctrine.
7. `go-modern`: status: overlaps existing; repo Go workflow and modern stdlib idioms must prove a delta beyond `AGENTS.md`.
8. `hyprd-rebuild`: status: ready-ish once plan-approved; script-backed rebuild workflow for `hyprd` edits.
9. `install-dotfiles`: status: speculative; symlink/copy/restart semantics for install steps.
10. `opencode-config`: status: deferred; defer unless a delta beyond `customize-opencode` is proven.

## Candidate leaves

- `scout/session`: status: tracked elsewhere; already shaped in `compaction.md` and likely justified by managed-session recovery.
- `scout/ideas`: status: speculative; read-only variation generator for Grok/GLM idea sweeps, but validate by repeated real use before implementing.
- `verify/build`: status: speculative; maybe useful if build/install falsification keeps recurring, otherwise `verify/test` plus skills suffice.
- Rejected: provider-flavored twins (`review/x`, `review/glm`, `build/glm-bulk`) are model routing, not leaf identities.

## Backlog cards

- `mode-targeted skills`: borrow Roo-style mode-specific skill bundles, but keep it a naming and selection convention unless OpenCode gains native mode scoping.
- `run/verify skills`: model after Claude `/run` and `/verify`; for dotfiles, start with `hyprd`, `ewwd`, and `newtab` workflows.
- `theme-oracle`: skill or script for color and theme consistency across eww, newtab, opencode, and hypr.
- `doctrine-sync`: status: strong script/check candidate because duplicated primary and leaf doctrine is already a known drift risk.
- `provider-health`: skill or check for xAI/OpenCode-Go auth, model availability, effort variants, capacity-report handling, and build-mode guidance once real usage signals exist.
- `upstream-x-search`: watch or file the OpenCode ask for a provider native-tool hook because xAI X-search stayed upstream-blocked in the closed fleet thread.
- `skill-registry`: defer until count crosses ~15; maybe generate it from `SKILL.md` frontmatter instead of maintaining it by hand.

## Rejected for now

- Full frontend, design, and QA leaves lack enough local evidence.
- Current UI surfaces are eww, vanilla newtab, and opencode TUI plugins, not React-heavy app work.

## Proposed future workflow

```text
variation (Grok/GLM) ──▶ selection (GPT/Anthropic/primary synthesis) ──▶ inheritance (skill/spec/leaf/script) ──▶ pruning
```

## Pruning ritual

- Promote a skill or leaf only after the gap is felt twice.
- Delete stale experimental skills once they go unused with no real invocation.

## Next steps

1. A plan writer must select one tranche before any authoring, then cite two observed gaps or explicitly mark an experimental canary.
2. Verify actual OpenCode skill discovery and trigger behavior after restart with a tiny canary skill as required runtime verification before writing a corpus.
3. Decide whether `scout/ideas` is worth adding or whether the `idea-council` skill plus routed review/scout leaves is enough.
4. Keep `scout/session` tracked in `compaction.md`, not duplicated here.
5. Apply the pruning ritual as skills and leaves accumulate.

## Questions for parent

- None blocking; the ledger records candidates without requesting a decision that changes the artifact.
