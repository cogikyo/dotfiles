# Orchestrate: three primaries, one hop, durable specs

Status: draft, all phases pending.
This file inaugurates the `.spec/` convention it defines, and it must obey its own contract.
It lives in `config/opencode/.spec/` because the concern is owned by the opencode config, per the directory-scoping rule below.

## Goal / end state

Replace the five public modes and every middle manager with three primary modes and a flat leaf fleet.
End state: `scheme`, `collab`, and `drive` primaries; leaves under `scout/`, `review/`, `build/`, `scribe/`, `verify/`; coordinators deleted; `.spec/` docs carry coordination.
Core invariant: every unit of work sits at most one hop from a session a human can step into.
Primaries delegate directly to leaves and synthesize results themselves; leaves never delegate.

## The three primaries

- `scheme`: conjecture mode, human present and arguing back.
  Reads everything, verifies docs/source/APIs, weighs architecture tradeoffs, writes and edits plan docs in `.spec/`.
  Expects heavy human input; values opinionated, critical conjectures.
- `collab`: selection and steering mode, the WIP mode, human present.
  Mostly autonomous but manages mixed concerns, pivots, and branches; dispatches forked sessions when needed; synthesizes agent progress and decides next steps with the user.
- `drive`: execution mode, the AFK mode.
  Implements toward a known end state; self-serves planning, review, and verification by dispatching leaves; resolves unclear goals itself.
  Never blocks waiting on the user; prefers deny-and-report over ask, because an approval prompt at hour 2 of an unattended run is a silent hang.
  Sequential by default, token-thrifty over fast.
  Canonical rhythm: scout → build → review → scribe → commit, landing clean atomic commits continuously; the pre-commit scribe pass polishes comments and docs, distinct from the phase-exit spec condensation.

The mode flip carries the presence bit.
Work in scheme or collab, then flip the session to drive when stepping away; context stays, the permission envelope flips at that moment.

## Permission envelopes

Open tools, focused instructions: role clarity comes from short unambiguous prompts, and permission walls stay rare.
Only the three primaries carry distinct envelopes.

- scheme: read everything; write `.spec/` files only as its normal mode of operation; never agent prompts, `AGENTS.md` files, or code.
- collab: full edit; prompts on the risky tail.
- drive: full edit; auto-allow within bounds; outright deny the irreversible tail such as history-rewriting git and system-level installs.
- Leaves inherit the spawning session's envelope.

## `.spec/` contract

`.spec/` is a directory-scoped convention for plan, spec, and logbook docs, named to decouple the artifact from mode branding.
Place a `.spec/` inside the directory that owns the concern; the repo root gets one only for genuinely whole-repo concerns.
Committed by default; a repo opts out with one `.gitignore` line.

Every doc includes:

- Goal and end state.
- Phase partition with file ownership per phase.
- Per-phase status blocks updated by the owning drive session.
- Decisions log and deviations.
- Open questions queued for the user.
- Condensed next steps.

Specs must shrink over time (ΔS < 0; entropy exports to git history).
Drive's phase-exit duty, after the phase's commits land, is a `scribe/spec` condensation pass: summarize what landed, prune finished phases, condense next steps, delete the doc when next steps is empty.
Deletion is a commit too.

## Parallelism and forked sessions

Big parallel work uses forked opencode sessions, never nested subagents.
Sibling sessions seed from a `.spec/` doc and coordinate through artifacts, the spec plus the git tree, stigmergy-style.
The user can step into any forked session and flip it to collab to steer.
Only collab forks sessions; drive and scheme never do.
Collab recommends a fork when threads have diverged sufficiently or when parallel spec buildout would let the user steer each; the user confirms before spawn.
Spawn support is in scope for this migration (phase 6): at minimum a documented flow, possibly a small helper.

Concurrency asymmetry: unattended drive runs sequential phases on the shared tree.
Parallel forked drives are a collab-mode option used only when the human is present to referee.
No git worktrees; code is read as it lands.
When parallel, the spec must partition file ownership per phase up front.

## Refusal recovery

Sessions are cattle, `.spec/` is the pedigree.
When a provider refusal lodges in a session's history, that session is unrecoverable; never resume it.
The delegate plugin should treat this as a recoverable error class: detect it, discard the child, return a compact blocked report, and the parent re-briefs a fresh child from the durable brief.
Escalation ladder: reword the brief first, switch provider as last resort.
Primary-session refusals are handled manually: revert the message or compact the session so a summary replaces the original wording.
Automation here is deliberately deferred until a real refusal is encountered in practice.
Do not overindex; provider refusal behavior will drift.

## Model routing

Delegate plugin per-call `{model, effort}` routing stays.
Affinity guidance carries forward: cheap fast models for relays, scouts, and summaries; strong models for deep review and acceptance; fable or opus for long-context synthesis and writing.

## Leaf fleet (user-approved)

Each leaf is nearly a skill: one clear job defined by focus, with permissions inherited from the spawner.
Five worker groups, regrouped by function in this migration: scouts map and warn, reviewers judge, builders edit code, scribes write prose and commits, verifiers collect evidence.

### scout/

- `scout/context`: maps the governing skills, instructions, `AGENTS.md` scopes, conventions, and task-relevant files, so sessions load the right instructions and none of the wrong ones (from the `review/scout` split).
  Must not review code quality or change state.
- `scout/dirty`: reviews uncommitted and in-flight change state; multiple WIP threads, staged vs unstaged clusters, recently squashed or reset commit sets, interference between concurrent sessions.
  Change-state reconnaissance only; must not judge code quality or map instructions.
- `scout/library`: reuse truth; reviews whether existing shared utils, helpers, stdlib, and modern language facilities already solve the need, verifies they are used correctly, and flags ambiguities or better shared-lib opportunities.
  Existing-capability mapping and misuse warnings only; must not implement.

### build/

- `build/worker`: default bounded implementer for one edit slice with verification.
  Must not broaden into cleanup, tests, or adjacent improvements.
- `build/proto`: shape-discovery builder; makes it work fast with zero abstraction ceremony, throwaway quality accepted.
  Must not polish, refactor, add tests, or reorganize files.
- `build/canal`: canalizer; executes an approved reorg or refactor plan mechanically and fast.
  Must not redesign or second-guess the approved shape.
- `build/test`: implements approved product test artifacts only: tests, fixtures, snapshots, goldens, harnesses.
  Must not touch production code.

### review/

- `review/debug`: root-cause and correctness review; hypotheses plus the next discriminating check.
  Must not drift into style review or implementation.
- `review/security`: adversarial trust-boundary review; findings require a credible exploit or exposure path.
  Must not run destructive scans or broaden past the named threat model.
- `review/architect`: system shape, boundaries, ownership, and conceptual truth, both prospective mapping and retrospective critique (absorbs `plan/architect`); the selection judge in the canalization workflow.
  Must not do line-level lint or write implementation steps.
- `review/critic`: adversarial detail critique of plans, specs, options, and acceptance criteria (moved from `plan/critic`).
  Must not write replacement plans.
- `review/simplify`: cognitive load, slop, duplication, and dead code (absorbs `review/janitor`).
  Must not turn findings into speculative rewrite plans.
- `review/modernize`: deprecated APIs, stale idioms, obsolete fallbacks, compatibility cruft.
  Must not recommend novelty churn.
- `review/profile`: performance shape backed by hotness or blast-radius evidence.
  Must not micro-optimize cold paths.
- `review/test`: judges test necessity, quality, and maintenance entropy; recommends delete/keep/consolidate/rewrite/defer.
  Must not write or run tests.

### scribe/

- `scribe/spec`: spec hygiene; creates, updates, condenses, and deletes `.spec/` docs per the contract (folds `plan/writer` and the doc half of `verify/scribe`).
  Must not edit code or non-spec docs.
- `scribe/doc`: READMEs and human-facing prose in repo writing style.
  Must not touch code, comments, or specs.
- `scribe/comment`: code comments and doc comments; drift, tiers, per-language conventions.
  Must not change code behavior.
- `scribe/banner`: section headers, comment boxes, and glyph-width banners; edits via Python because patch tools corrupt Nerd Font glyphs.
  Must not rewrite prose or comments beyond structure.
- `scribe/agents`: writes and maintains agent prompts, skills, and `AGENTS.md` context files; keeper of accumulated craft knowledge on what makes agents and skills work well with opencode and this user's project setups.
  Harness and instruction artifacts only, always on explicit user approval; must not edit code or `.spec/` docs.
  Agent self-modification routes through this one approved leaf; primaries never edit their own prompts.
- `scribe/commit`: atomic conventional commits for approved scopes (moved from `verify/commit`; committing is scribe-shaped work).
  Mutates git state only; never file contents.

### verify/

- `verify/test`: runs suites and commands, QAs results, owns bounded verification artifacts.
  Must not write product tests or fix production code.
- `verify/web`: verifies claims against current official docs, APIs, and published constraints, with citations.
  Read-only; must not use SEO slop as primary evidence.
- `verify/source`: verifies claims against upstream source via the src cache and registries.
  Read-only toward the target repo; must not run untrusted build scripts.

Retired: `build/manager` (middle manager), `review/scout` (split into `scout/context` and `scout/library`), `review/janitor` (into `review/simplify`), `plan/architect`, `plan/critic`, `plan/writer`, `verify/scribe` (merged or moved as above); the `plan/` leaf directory dissolves.
Moved: `review/dirty` → `scout/dirty`, `verify/commit` → `scribe/commit`.

## Canalization workflow (variation → selection → inheritance)

A named workflow for discovering shape before fixing it, Waddington-style.

1. Variation: one or more `build/proto` passes produce working variants with no abstraction.
2. Selection: after several complete, `review/architect` assesses the survivors and proposes the reorg.
3. Approval: the human (collab) or drive itself when the end state was pre-approved.
4. Inheritance: `build/canal` workers execute the reorg quickly; verify and commit fix the shape into the lineage.

## Migration phases

Sequential; runnable unattended by drive now that the roster and open questions are settled.
The tree is inconsistent mid-migration; restart opencode only at the end.

### Phase 1: write the three primary prompts

Status: pending.
Owns: `config/opencode/agents/scheme.md`, `config/opencode/agents/collab.md`, `config/opencode/agents/drive.md` (rewrite).
Name the target leaf roster in task permissions even though leaves land in phase 3.

### Phase 2: port shared doctrine

Status: pending.
Owns: the same three primary files.
Distill from the old five modes: workflow notation, parent-brief shape, model affinity, interrupted-child recovery, refusal policy.
Duplicate into each primary; there is no include mechanism.

### Phase 3: refactor leaves

Status: pending.
Owns: `config/opencode/agents/{scout,build,review,scribe,verify}/*.md`.
Apply the roster: create the `scout/` and `scribe/` groups plus `build/proto` and `build/canal`; move `review/dirty` to `scout/dirty` with the sharpened change-state role and `verify/commit` to `scribe/commit`; merge, move, and delete per the retired and moved lists; keep leaf prompts short and focus-defined.

### Phase 4: retire coordinators

Status: pending.
Owns: `config/opencode/agents/{build,plan,review,verify}.md`, `config/opencode/agents/build/manager.md`, `config/opencode/opencode.json`.
Delete the four coordinator modes and the manager; update any opencode.json agent or permission wiring.

### Phase 5: update references

Status: pending.
Owns: `AGENTS.md` (repo root).
Rewrite the Layout and Harness agent references to the new structure and the `.spec/` convention.
`config/opencode/.spec/delegate.md` content is out of scope for this migration.

### Phase 6: session-spawn flow

Status: pending.
Owns: `config/opencode/agents/collab.md`, plus a helper location chosen by the implementer (`cmds/` per repo convention if a helper is built).
Encode the fork flow in collab: recommend on sufficient thread divergence or parallel spec buildout, user confirms, then spawn; at minimum a documented flow, possibly a small helper.

### Phase 7: delete dead files, verify, commit

Status: pending.
Owns: whole migration diff.
Verify: `jq . config/opencode/opencode.json`, restart opencode, smoke one trivial delegation per leaf group from each primary.
Commit per logical story; deletion commits included.

## Decisions log

- Three primaries replace five modes; all coordinators and managers retired (settled).
- One-hop invariant; no middle managers ever, since they made delegation unobservable and added lossy relay hops (settled).
- Mode flip carries the presence bit (settled).
- Forked sessions over nested subagents for big parallel work (settled).
- Sequential unattended, parallel only refereed; no worktrees (settled).
- `.spec/` committed by default, blacklist model, shrink-over-time contract (settled).
- `.spec/` dirs are directory-scoped, placed in the directory owning the concern; repo root only for whole-repo concerns (user decision).
- This spec moved to `config/opencode/.spec/orchestrate.md`, and `DELEGATE.md` moved to `config/opencode/.spec/delegate.md` (user-approved).
- Refusal-tainted sessions are never resumed; re-brief fresh from durable state (settled).
- Open tools, focused instructions; envelopes only on primaries (settled).
- `{model, effort}` routing and affinity guidance carry forward (settled).
- Plan-directory dissolution (`plan/architect`, `plan/critic`, `plan/writer` merged or moved) is user-approved.
- `review/scout` split into context and library leaves, now `scout/context` and `scout/library` (user decision).
- Dirty-state placement corrected by user decision: dirty stays a standalone change-state leaf.
- Scheme's write boundary settled: `.spec/` files only, never agent prompts, `AGENTS.md` files, or code (user decision).
- `scribe/agents` added as keeper of harness and instruction artifacts, on explicit user approval only; agent self-modification routes through it, so no primary edits itself (user decision).
- Primary-session refusal recovery automation is deliberately deferred until encountered in practice; manual handling stays the interim answer (user decision).
- Taxonomy regroup happens now, in this migration: five worker groups `scout/`, `review/`, `build/`, `scribe/`, `verify/`; fleet 24: scout 3, review 8, build 4, scribe 6, verify 3 (user decision).
- Commit autonomy confirmed: drive lands clean atomic commits continuously; canonical rhythm scout → build → review → scribe → commit; the commit leaf moves to `scribe/commit` (user decision).
- Only collab forks sessions, on user confirmation per spawn; drive and scheme never fork; spawn support pulled into migration scope (user decision).
- Roster user-approved after several shaping rounds.

## Deviations

None yet.

## Open questions

None; all resolved into the decisions log.

## Next steps

1. Run phases 1 through 7 sequentially; drive-able now.
2. Follow-on outside this migration: delegate plugin refusal detection, `.spec/delegate.md` affinity updates.
