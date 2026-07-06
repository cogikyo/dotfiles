# Idea ledger: agent-owned skills

Seeded 2026-07-05 from the user's request for a durable idea bank feeding future skill and agent planning.
Goal: preserve the skill lifecycle and candidate ideas without replaying the model sweep or source check.
End state: candidates are authored, rejected, promoted, or drained until this file empties and gets deleted.

No agent, config, code, or non-spec doc was edited during the seed session.

## Problem

The problem is not missing skills in general.
The problem is evolving agent-specific craft without creating a shared skill pile, accidental cross-agent coupling, or project-specific global baggage.
Each agent should grow its own skill ecology and scripts, isolated from other agents unless a later promotion is explicit.
OpenCode skills and agents should stay general; project details belong in project agent/context/spec files or explicit files the agent is told to read.
Most current candidate skills are premature because no repeated task friction has selected them yet.

## Thread ownership

This idea thread owns only this file.
Ledger owner: `scribe/spec` or primaries own this ideas file.
`scribe/agents` may read this ledger and is the route for approved agent or skill authoring, but it must not edit `.spec/` docs.

## Phase partition and status

- Seed ledger: owns only `config/opencode/.spec/ideas/agent-skills.md`; status done; no runtime OpenCode behavior changed.
- Isolation canary: owns no files yet; status next; discover whether owner isolation works by config, naming, prompt discipline, or plugin/check.
- Approved authoring: owns future agent or skill files only after user approval; status blocked on observed friction, canary evidence, and approval.

## Evidence

- Current fleet: primaries `scheme`, `collab`, `drive`, `learn` (built-in `plan`/`build` disabled to avoid name collisions); leaf groups scout/build/review/scribe/verify; no checked-in local `SKILL.md` files were observed in the seed session.
- The closed fleet thread recorded the 26-leaf fleet at its cognitive ceiling; invented leaves were declined unless a gap is felt twice.
- OpenCode v1.17.13 source facts were checked at commit `10c894bdeef3618f5666fb506ef7f9491bb964d8`, but not run-verified here.
- Checked source says default skills live under `config/opencode/skills/<name>/SKILL.md`, `skills.paths` is optional, `name` is required, and `description` is effectively required for triggering.
- Checked source says OpenCode does not auto-run scripts inside skills; sibling files are resources the model may read or invoke.
- Web scout found Agent Skills support and related patterns in OpenCode, Agent Skills, Claude Code, Cursor, Roo, and MCP docs.
- X community search returned nothing usable because X access failed with HTTP 403.

## Sweep provenance

- Grok Build, GLM 5.2, GPT, Anthropic, web scout, and failed X/community scout contributed ideas or critiques.
- This section records provenance only and does not rank model usefulness.
- In this idea loop, `x` means Grok or Grok Build, currently `grok-build`.
- Live X community search was unavailable and should not be conflated with Grok output.
- Grok/GLM/GPT/Anthropic sweeps are optional variation probes for idea cards, not selectors.

## How this solves it

The lifecycle keeps craft local until repeated friction proves a reusable procedure.

```text
real task friction ──▶ idea card ──▶ owner agent selected ──▶ isolation canary if needed ──▶ scribe/agents authors approved skill/script ──▶ owner uses it ──▶ prune or promote
```

Idea sweeps can widen the mutation pool, but selection comes from observed work, owner fit, canary evidence, and user approval.

## How skills are made

- Start with an observed need from real task friction.
- Pick one owner agent or leaf before authoring.
- Capture only generic procedure, commands, checks, or scripts that fit that owner.
- Keep project-specific details in project context, specs, or explicit files the owner is told to read.
- Get explicit user approval before changing agent prompts, skill files, config, or scripts.
- Route approved skill or script authoring through `scribe/agents`.
- Run a runtime or canary verification before relying on owner isolation or script behavior.
- Reject or park candidates that only sound useful from a sweep.

## How skills are used

- Owner-only use is the default.
- No other agent should depend on an owner skill by accident.
- Cross-agent reuse requires deliberate promotion and a named reusable pattern.
- Project details are supplied separately at runtime through project files, specs, or files the agent is explicitly told to read.
- If OpenCode cannot enforce per-agent isolation natively, the next canary must discover whether isolation is done by config, naming, prompt discipline, or a plugin/check.

## Decisions log

- Default to agent-isolated skills because accidental cross-agent use is the failure mode to avoid.
- Default each skill to one owner agent or leaf; cross-agent skills require a promotion decision and reusable pattern.
- Keep OpenCode skills and agents general enough to reuse across projects.
- Keep project workflows out of global skills unless they become generic patterns.
- Skills own reusable procedures; scripts own deterministic helpers; leaves own separate context, permission, and model membranes.
- Skill-local scripts are allowed while one skill owns them; reusable tools graduate to `cmds/` or plugins.
- Agreement across model sweeps is weak evidence unless backed by source checks, canaries, or repeated real use.

## Selection rule

Add a new leaf only when it needs a distinct permission envelope, separate context window, stable primary target, or model/provider dissent membrane.
Otherwise prefer a skill, script, command, plugin, or model routing.
Use a skill for a reusable procedure or long context ritual owned by one agent by default.
Do not make a skill when it duplicates one leaf prompt, `customize-opencode`, or an existing `AGENTS.md` section without reducing context tax.

## Backlog cards

- Parked until repeated friction: `mode-targeted skills`, `run/verify skills`, `theme-oracle`, `doctrine-sync`, `provider-health`, `upstream-x-search`, and `skill-registry`.
- `doctrine-sync` has the strongest current signal because duplicated primary and leaf doctrine is already a drift risk.

## Pruning ritual

- Promote a skill or leaf only after the gap is felt twice.
- Delete stale experimental skills once they go unused with no real invocation.

## Next steps

1. Run a tiny isolation canary before authoring any listed skill.
2. Record whether owner isolation is native config, naming, prompt discipline, or a plugin/check.
3. Wait for real task friction before promoting parked candidates.

## Questions for parent

- None blocking.
