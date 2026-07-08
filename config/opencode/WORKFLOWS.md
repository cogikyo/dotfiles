# Workflows

Shared orchestration doctrine for the primary modes: scheme, collab, drive, learn.
Mode prompts stay lean; this doc owns the mechanics they share.
Frontmatter permissions in each mode file are the enforcement membrane; prose here never overrides them.

## Sources of truth

- `WORKFLOWS.md`: how to orchestrate leaves and sessions.
- `MODELS.md`: model, effort, council, and failure-handling policy.
- Mode files: persona, permissions, write boundaries, and terminal products.
- Leaf agent files: exact leaf behavior; the catalog below is a routing summary only.

Read this doc before the first dispatch of a session, and `MODELS.md` before routing leaves.

## One hop only

Every unit of work sits at most one hop from a session a human can step into.
Primaries delegate directly to leaves and synthesize results themselves.
Leaves never delegate; there are no middle-manager agents.
Managed primary sessions are sibling roots with their own artifacts, never nested leaf managers.

## Leaf fleet

Scouts map and warn, reviewers judge, builders edit code, scribes write prose and commits, verifiers collect evidence.
Mode frontmatter decides which leaves a mode can actually call.

- `scout/context`: maps governing instructions, `AGENTS.md` scopes, conventions, and task-relevant files.
- `scout/dirty`: reviews uncommitted and in-flight change state and cross-session interference.
- `scout/session`: maps previous and active OpenCode sessions, continuity ledgers, owners, and recovery context.
- `scout/library`: maps existing utils, stdlib, and language facilities that already solve the need.
- `scout/web`: open-ended web reconnaissance; maps the option space, prior art, and ecosystem direction.
- `build/worker`: one bounded edit slice with verification.
- `build/proto`: shape discovery, fast and throwaway; no polish.
- `build/canal`: mechanical execution of an approved reorg or refactor plan.
- `build/test`: approved product test artifacts only; never production code.
- `review/debug`: root-causes correctness issues with discriminating checks.
- `review/security`: adversarial trust-boundary review with credible exploit paths.
- `review/architect`: system shape, boundaries, ownership; the selection judge in canalization.
- `review/critic`: adversarial detail critique of plans, specs, options, and acceptance criteria.
- `review/simplify`: cognitive load, slop, duplication, and dead code.
- `review/modernize`: deprecated APIs, stale idioms, and compatibility cruft.
- `review/profile`: performance shape backed by hotness evidence.
- `review/test`: test necessity, quality, and maintenance entropy.
- `scribe/spec`: creates, updates, condenses, and deletes `.spec/` docs per the contract.
- `scribe/doc`: READMEs and human-facing prose.
- `scribe/comment`: code and doc comments.
- `scribe/banner`: glyph-width banners, via Python.
- `scribe/agents`: agent prompts, skills, and `AGENTS.md` files, on explicit user approval only.
- `scribe/commit`: atomic conventional commits for approved scopes.
- `verify/test`: runs suites and commands and QAs results.
- `verify/web`: verifies claims against current official docs, with citations.
- `verify/source`: verifies claims against upstream source.
- `verify/x`: second-opinion verification via Grok, weighing live community signal from X against docs.

## Mode envelopes

- scheme: writes `.spec/` only; dispatches scouts, reviewers, `scribe/spec`, and verifiers; asks freely; never commits or forks.
- collab: full fleet; asks at real decision points; `scribe/agents` on explicit approval; recommends forks the user confirms.
- drive: full fleet minus `scribe/agents`; never asks; denies irreversible operations and reports them; spawns managed sessions only from durable specs.
- learn: scouts, `review/architect`, and verifiers only; no artifacts; asks freely.

## Leaf briefs

Include objective and scope, target files or search bounds, governing context files and `AGENTS.md` paths, constraints and non-goals, verification expectations, and known traps.
Name the review axis for every reviewer and the claim under test for every verifier; otherwise they waste context or check the wrong thing.
Keep briefs small; include only context that changes the task.
Leaves inherit the session's permission envelope.

## Workflow notation

Use this notation in leaf briefs and `.spec/` docs when a diagram helps:

- `──▶` sequence.
- `? condition` branch point.
- `∨` choose one alternative.
- `∥` parallel work.
- `*` optional.
- `+` repeat loop.
- `{user input: ...}` explicit user decision or approval.
- `[context: ...]` durable or shared context packet.
- `[parent: ...]` parent-supplied context to a leaf.

## `.spec/` coordination

`.spec/` is a directory-scoped convention for plan, spec, and logbook docs.
Place it inside the directory that owns the concern; the repo root gets one only for genuinely whole-repo concerns.
Committed by default; a repo opts out with one `.gitignore` line.

Every doc includes: goal and end state, phase partition with file ownership per phase, per-phase status blocks, decisions log and deviations, open questions for the user, and condensed next steps.
Specs shrink over time (ΔS < 0); entropy exports to git history.
When parallel forked work is anticipated, partition file ownership per phase up front.

After loading a governing `.spec` packet in a durable root thread, modes with `continuity_track` call it to name the jump target with 3-4 ALL-CAPS words, <= 28 chars; if unavailable pre-restart, continue without blocking.

## Managed sessions and forks

Big parallel work uses managed OpenCode sessions, never nested subagents.
Use one when a single context would become the bottleneck: long unattended work, compaction pressure, diverged phases, or parallel phase ownership.
Write or update the owning `.spec/` packet before spawning: objective, scope, files, current dirty state, expected commits, verification, and recovery checks.
The spawned session should be a bounded phase, usually in drive, never an open-ended copy of the whole objective.
Siblings coordinate through artifacts only, the spec plus the git tree; no worktrees, code is read as it lands.
The parent reconciles by git state, `.spec/`, and scout evidence; chat memory is never the authority.
Keep one owner per dirty thread and prefer fewer sessions than phases.
Approval split: collab recommends and the user confirms; drive spawns only from a durable spec; scheme and learn never fork.

## Canalization

Use when the end state is approved but the shape is unknown: variation → selection → inheritance.

1. One or more `build/proto` passes produce working variants with no abstraction.
2. `review/architect` assesses the survivors and proposes the reorg.
3. The shape is approved: by the user in collab; by drive only when the end state was pre-approved.
4. `build/canal` executes the reorg fast; verify and commit fix the shape into the lineage.

## Recovery

Treat an empty or interrupted child result as unknown completion state; the child may have edited files before losing its report.
Reconcile with `scout/dirty` when available, otherwise direct reads, then continue from durable state instead of blindly re-running the slice.
A refusal-tainted child session is unrecoverable; never resume it.
Discard it and re-brief a fresh child from the durable brief: reword the brief first, switch provider as last resort.
Sessions are cattle; `.spec/` docs and the git tree are the pedigree.

## Commit discipline

- `scribe/commit` commits only the current thread's approved scope and files.
- The user may edit files concurrently; include their edits when related.
- Extremely unrelated dirty files likely belong to another session; leave them alone.
- No history rewriting: no amend, rebase, force-push, or reset; a bad commit gets a follow-up commit.
