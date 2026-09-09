---
name: drive
description: Use to prepare or execute an approved multi-phase workflow, especially AFK work with concurrent owners, explicit check authority, bounded repair, and attended commit or decision boundaries.
---

# Drive

Design an execution workflow in Collab, then carry the approved contract through its dependencies, evidence decisions, and bounded repairs.
Collab can own a long outer workflow with several Orchestrator objectives, including concurrent siblings where their scopes are independent.
Each Orchestrator can own a long inner run through leaves only.
Drive is a procedure, not a primary mode or a child agent type.
Follow the owner's routing, permission, Git, and continuity rules; do not invent missing design or expand authority after dispatch.

## Prepare in Collab

Load Drive while shaping the workflow, before dispatch approval.
Start from the desired terminal outcome and work backward through real contract, implementation, evidence, and attended dependencies.
Use `scheme` for unresolved product design; Drive arranges execution of settled intent rather than burying new design choices inside a child brief.

1. Identify coherent objectives whose investigation or execution working sets deserve separate owners.
2. Freeze the interfaces and inputs that make independent streams possible, assigning shared changes to an earlier foundation or later integration scope.
3. Connect ready work, required joins, conditional routes, bounded repairs, and attended returns with `workflow` where prose would hide dependencies.
4. Name routes, authority, checks, and completion evidence for each scope, then present the complete contract for approval.

Long work may need many Orchestrator scopes over time and three sibling Orchestrators at once; the useful unit is an outcome with stable inputs, not a file or a fixed agent quota.
Only Collab starts these siblings or a later Orchestrator run.
A stream is independent only when its reads do not depend on sibling writes, its writes are disjoint, and its accepted contract can be checked without unfinished sibling behavior.
Serialize shared source, schemas, registries, lockfiles, generated outputs, and documentation; separate directories alone do not establish independence.
If independence fails during execution, stop the affected streams and return the contract conflict rather than coordinating an unapproved cross-stream rewrite.

## Execution contract

Before execution, the approved brief must name:

- Objective, exclusions, governing specs, resolved worktrees, baseline, and exact write scope.
- Each assignment's owner, model, effort, and permitted fallback routes.
- Dependencies, concurrency, disjoint write boundaries, and any branch integration strategy.
- Required checks, approved resource cost, and success or failure edges.
- Repair owners, attempt limits, terminal acceptance conditions, and return events.
- Decisions, Git boundaries, or blocked permissions that require Collab.

Choose routes using the current owner policy, then freeze them in the approval.
Routine coordination should not inherit a premium parent model; preserve an explicitly approved expensive model or effort when the task warrants it.
Do not silently substitute models or variants after dispatch.

For delegated AFK execution, Collab passes `unattended: true` and explicit authority to Orchestrator.
Implementation requires `authority: "write"`; evidence-only execution uses `"read-only"`.
The runtime carries unattended restrictions to every descendant and converts permission requests into denials.
Loading Drive in an attended session does not create that envelope or guarantee prompt-free AFK operation.

## Execute

1. Reconcile durable state and identify work with sufficient inputs and authority.
2. Dispatch ready assignments on their exact approved routes, concurrently where independent.
3. Judge returned evidence and follow the required join, conditional route, bounded repair, or return.
4. Update todos with accepted boundaries, active work, loop counts, and blockers until the terminal condition is reached.

Use `workflow` when the dependency structure needs a connected diagram.
Do not create gates solely to receive reports, or split one coherent implementation into file-sized assignments.
An Orchestrator dispatches leaves only; planning and review phases use its own skills plus permitted evidence or specialist leaves.
Keep broad implementation with an approved builder when it would crowd the execution owner's working set.
Direct product edits require explicit ownership and write authority.

> [!IMPORTANT] The dispatch already approves internal execution
>
> Do not request confirmation after each step or add owners, checks, substitutions, edges, or repair attempts beyond the contract.
> Return when a condition has no approved edge, a loop is exhausted, or evidence invalidates a governing design assumption.

## Evidence and recovery

Builders own formatting, source comparison, and other approved cheap checks within their implementation boundary.
Independent verification runs only when the user explicitly approved its check classes or commands and resource bounds.
Keep compact conclusions rather than raw investigations and mark completion only after required evidence passes.
Later edits invalidate affected review or verification; return through the approved affected evidence, not directly to acceptance.

After interruptions or context limits, follow the owner's child continuity rules and reconcile the tree and Git before reissuing writes.
The previous attempt may already have edited files.
Use fresh children for repair-loop passes; never resume a hard-stopped or compacted child.
The task surface guarantees neither a progress heartbeat nor a permission-wait watchdog.
Recover from returned failures, interruptions, and blockers without claiming stronger supervision or bypassing a denied operation through another tool or child.

## Return boundaries

Only attended Collab owns Git mutation.
A child returns at a required commit, rebase, merge, publication, or other unapproved side effect; it never dispatches Collab or delegates Git to a builder.
Do not start dependent overlapping work that requires an unfinished Git boundary.

A named spec remains governing input, not a redesign target or execution journal.
Change or remove it only through its exact approved artifact step.
Return the completed workflow or a precise continuation brief with accepted work, changed paths, checks and outcomes, dissent, repair counts, blockers, and residual risk.
At a commit boundary, include exact changed scope and a proposed message without committing.
Name the decision or event needed to continue and do not claim completion while it remains blocked.

## Extended example: queue leases through two attended commits

This illustrative service migration is a worked contract, not permission to execute anything in this repository.
The approved `.spec/queue.md` fixes lease ownership, fencing, cancellation, tenant admission, event semantics, and the public API.
Collab supplies the repository, worktree, branch, baseline, dirty-file exclusions, and sanitized failure traces.
The user requested regression tests and two commits: foundation and independent domain behavior first, integrated API exposure and rollout guidance second.
Length comes from distinct evidence, ownership, and migration dependencies; a prose-only update should use one writer instead.

### Outer workflow: Collab owns the service migration

The outer graph uses its own numbering; the inner Run A and Run B graphs below number their leaves separately.
Collab loads Drive to prepare this contract, secure approval, and keep the long-lived user and Git context while children execute bounded objectives.
All `{N}` nodes use `[high • openai/gpt-5.6-sol-fast] orchestrator`, load Drive, and receive `unattended: true` with the authority named below.
Every dispatch and resume repeats both task fields in the brief and call, tightening all evidence leaves to `authority: "read-only"`.
Only Collab may start them; none may dispatch another Orchestrator or Collab.

```text
(1) ─→ {2} ─→ (3) ─→ <3> ─┬─→ {4} ─┐
                          ├─→ {5} ─┼─→ (7) ─→ <7>
                          └─→ {6} ─┘
```

1. `self`: Freeze the approved migration and execution envelope.
   - Supply resolved paths, baseline and dirty exclusions, stable public behavior, exact child scopes, check budgets, and both attended commit boundaries.
2. `orchestrator`, write authority: Establish the shared foundation.
   - Reconcile the fixed design against unchanged consumers with Scheme, own only `internal/contract/**` through a Grok high `build/general` leaf, and judge the resulting fencing value, admission interface, and event shape against all three streams.
3. `self`: Accept the foundation as immutable input for the domain wave.
   - Gate 3 requires contract fidelity and passing cheap checks, records the exact source snapshot, and returns any missing design decision before fan-out.
4. `orchestrator`, write authority: Own queue leases and worker lifetime as Run A below.
   - Complete coupled implementation, requested regressions, `docs/queue.md`, source judgment, and approved core and cancellation evidence without changing the foundation.
5. `orchestrator`, write authority: Implement tenant admission against the frozen interface.
   - Own only `internal/admission/**` and `docs/admission.md`, including requested quota regressions and approved admission evidence, without importing unfinished lease implementation.
6. `orchestrator`, write authority: Implement lease event aggregation against the frozen event shape.
   - Own only `internal/telemetry/**` and `docs/telemetry.md`, including requested event-count regressions and approved telemetry evidence, without reading changing queue or admission source.
7. `self`: Reconcile all three returned streams and perform the first attended commit.
   - Gate 7 requires current passing scoped evidence and no contract conflict; Collab inspects and commits only the approved changed scope, then confirms the committed baseline before new dependent dispatches.

Nodes 4, 5, and 6 are three sibling Orchestrators, each running Drive inside its own boundary.
The foundation stops writing before they start; each stream reads its own scope and frozen `internal/contract/**` only, plus the named unchanged inputs needed by its brief.
Run A's source questions may read unchanged API callers, but not its siblings' changing implementations.
Each stream owns its own documentation; shared API composition and `docs/rollout.md` wait until after the join.
The single supplied worktree needs no branch integration, and children do not stage or commit its changes.
Separate candidate implementations would instead need Collab-prepared worktrees and an approved attended integration strategy.

For nodes 2, 5, and 6, permitted leaves are `[high • xai/grok-4.6] build/general`, `build/patch`, `build/scribe`, and `verify/test`, plus `[xhigh • openai/gpt-5.6-luna-fast] scout/context` for bounded facts.
No independent critic is required for these settled interfaces.
At most two leaves run within each of these scopes; Run A allows three, so the domain wave permits at most seven active leaves across three owners.
Serialize each stream's writes before its evidence collection, and scope source comparison to that stream while siblings write elsewhere.
Each scope allows at most one known-cause repair by Grok `build/general`, or `build/patch` when the exact mechanics are settled, with fresh children and repeated affected evidence; Run A has the separate two-pass counter below.
Unavailable evidence, a foundation defect, or exhausted repair returns to Collab without enlarging a stream's write set.

The next phase consumes gate 7's confirmed committed baseline, not merely three completion reports.

```text
<7> ─→ {8} ─→ {9} ─→ {10} ─→ (11) ─→ <11> ─→ RETURN user
```

8. `orchestrator`, write authority: Integrate the committed domains through the API as Run B below.
   - Own only `internal/api/**` and `docs/queue.md`, composing lease, admission, and telemetry contracts with requested API regressions and current API and cross-boundary evidence.
9. `orchestrator`, read-only authority: Establish migration compatibility on the frozen integrated tree.
   - Use `[xhigh • openai/gpt-5.6-luna-fast] scout/context` for old/new caller reachability and `[high • xai/grok-4.6] verify/test` for the approved migration command, then reconcile both against the fixed compatibility conditions.
10. `orchestrator`, write authority: Prepare rollout and rollback guidance from passing compatibility evidence.
    - Reconcile compatibility evidence with the supplied operational constraints, then own only `docs/rollout.md` through `[high • xai/grok-4.6] build/scribe` and judge activation order and observable rollback triggers against the implemented behavior.
11. `self`: Accept the integrated result and perform the second attended commit.
    - Gate 11 requires current evidence and faithful rollout guidance; Collab commits the approved integration and documentation scope, then returns the result without deploying or publishing it.

Node 9 allows two concurrent read-only leaves; node 10 has one writer and one permitted fresh documentation correction, followed by repeated source-fidelity checks.
Neither owner adds other internal routes or check classes; rollout completion requires the owner's source comparison and the scribe's Markdown and diff checks, not a live operational trial.
Node 9 has no repair authority: a failed migration check stops this example for Collab's decision rather than silently restarting node 8 or borrowing its expired scope.
The rollout objective is preparation only; production access, deployment, and operational trials are excluded.
Changing product source after proof invalidates affected proof and review; no forward edge remains until Collab approves any missing repair and renewed evidence.
Every Orchestrator returns one complete scoped result or a blocker, while Collab retains the outer workflow until both attended boundaries finish.

> [!IMPORTANT] The drawing is not execution permission
>
> Each scope needs the named approval before it can run, including the independent check commands below.
> A denied operation or failed commit stops dependent work; no child changes tools, routes, or authority to bypass it.

### Approved envelope

Each inner Run A or Run B has one `[high • openai/gpt-5.6-sol-fast] orchestrator`, `authority: "write"`, and `unattended: true`.
Every leaf call repeats both fields in its task and brief, tightening evidence leaves to `authority: "read-only"`.
At most three leaves run concurrently in either run, with no overlapping writers and no writes in the inspected scope during evidence collection.
No fallbacks, nested Orchestrators, installs, generators, benchmarks, restarts, publication, branch merges, or autonomous Git mutation are approved.
Code integration occurs in the supplied worktree only after affected writers finish.
The spec stays read-only; progress lives in todos and returned evidence.

Run A permits `internal/queue/**`, `internal/worker/**`, and `docs/queue.md` writes.
Run B permits `internal/api/**` and `docs/queue.md` writes, consuming the committed core as read-only input.
A Run B defect requiring edits to any committed domain or shared contract returns to Collab rather than borrowing another run's expired authority.
Builders run `gofmt` on changed Go files, source comparison, and `git diff --check`; the scribe owns Markdown and source-fidelity checks.

The user separately approves these independent commands, each with a five-minute ceiling:

- Admission, outer node 5: `go test ./internal/admission`.
- Telemetry, outer node 6: `go test ./internal/telemetry`.
- Core: `go test ./internal/queue ./internal/worker`.
- Cancellation: `go test -race ./internal/queue ./internal/worker -run 'TestLease|TestCancel' -count=1`.
- API: `go test ./internal/api`.
- Cross-boundary: `go test ./internal/queue ./internal/worker ./internal/api -run 'TestLease|TestCancel|TestAPI' -count=1`.
- Migration, outer node 9: `go test ./internal/api -run TestMigrationCompatibility -count=1`.

Admission and telemetry allow one repair-driven repetition each; Run A and Run B allow at most two per command within their shared per-run repair counters; migration runs once.
The package boundaries for parallel checks must not read changing sibling packages or share mutable fixtures, service state, or generated outputs.
If that assumption fails, return for serialization or revised check approval rather than running a racy check.
Only `verify/test` runs these commands; approval excludes broader suites and direct builds.
A missing dependency, timeout, denied operation, unrelated failure, or new check requirement returns without setup or blind retry.
Dependency-source inspection uses the exact pinned version already in a sanctioned cache, without acquisition.

Routes below are fixed for this example, with effort chosen per concern:

- Scouts: `[xhigh • openai/gpt-5.6-luna-fast]` on their named roles.
- Source/test verifiers: `[high • xai/grok-4.6]` on their named roles.
- Coupled implementation: `[medium • openai/gpt-6-astra] build/owner`.
- Settled API mapping and named core repairs: `[high • xai/grok-4.6] build/general`.
- Documentation: `[high • xai/grok-4.6] build/scribe`.
- Independent concurrency judgment: `[medium • anthropic/claude-opus-5] review/debug`.

These are the only permitted inner routes, with `scout/context`, `verify/source`, and `verify/test` as the named factual roles.
The owner may load Scheme to resolve local implementation constraints and Review to judge evidence within the fixed design; neither activity grants new production behavior or a further owner.

### Run A, phase 1: establish the implementation boundary

Owner: Run A Orchestrator.
Its initial work includes reconciling Collab's baseline and authority before dispatch.

```text
        ┌─→ 2 ─→ 5 ─┐
(1) ────┼─→ 3 ──────┼─→ (6) ─→ <6>
        └─→ 4 ──────┘
```

1. `self`: Frame the fixed lease contract against the supplied failure traces.
   - Establish the source questions needed to implement the existing design, without reopening it.
2. `scout/context`: Trace renewal and expiry ownership.
   - Return queue transitions, persistent state, and the exact dependency primitive requiring verification.
3. `scout/context`: Trace cancellation and shutdown reachability.
   - Return worker lifetime, wait ownership, and existing regression entry points across callers.
4. `scout/context`: Trace public error and retry constraints.
   - Identify compatibility obligations that core changes must preserve, without API writes.
5. `verify/source`: Verify the primitive identified by step 2.
   - Establish pinned atomicity, clock, and cancellation guarantees from available source.
6. `self`: Reconcile the implementation constraints.
   - Gate 6 accepts a source-supported boundary or returns a design conflict or missing evidence to Collab.

Step 5 can run while 3 and 4 finish, but never before 2 identifies its source target.
Gate 6 requires all three branches, including step 5's answer.

### Run A, phase 2: implement, then gather independent evidence

This phase consumes gate 6's accepted constraints.
One builder owns the coupled queue/worker change; no separate integration agent is needed.

```text
<6> ─→ 7 ─→ 8 ─┬─→ 9 ───┐
               └─→ 10 ──┴─→ (11) ─→ <11>
```

7. `build/owner`: Implement leases, fencing, and worker lifetime together.
   - Own both core directories, requested regressions, integration, and cheap checks against the settled contract.
8. `build/scribe`: Describe the implemented guarantees.
   - Edit only `docs/queue.md`, deriving claims from completed source and the unchanged spec.
9. `verify/test`: Run the approved core command.
   - Return its exact outcome and tree identity, including failures without fixing them.
10. `review/debug`: Inspect expiry, stale fencing, and shutdown interleavings independently.
    - Return concrete reachable defects or falsification, including any overstated documentation guarantee.
11. `self`: Reconcile source judgment and execution evidence with `review`.
    - Gate 11 requires both reports and routes passing evidence onward, bounded defects to repair, or design and permission blockers to Collab.

Reports 9 and 10 may contain failures; receiving both is not a claim that their evidence passed.
The owner decides that at gate 11, without a second critic ceremony.

### Run A, phase 3: cancellation evidence and bounded recovery

This phase starts at gate 11, carrying the frozen source, documentation, and first evidence wave.
The repair route returns to the evidence split in phase 2 before reconsidering step 11.

```text
<11> ─┬─→ 12 ─→ (13) ─→ <13> ─→ RETURN Collab
      │                  │
      └─────┬────────────┘
            ↓
            14 ─→ 15 ─┬─→ 9 ──┐
                      └─→ 10 ─┴─→ (11)
```

12. `verify/test`: Run the approved cancellation race command.
    - Collect the more expensive interleaving evidence only after gate 11 passes.
13. `self`: Judge current evidence and return the core result.
    - Gate 13 accepts passing coverage for core behavior, requested regressions, and documentation, or selects the same bounded repair route as gate 11.
14. `build/general`: Repair the named in-scope core defect.
    - Restrict edits to the two core directories and run affected cheap checks; report no core edit for a documentation-only defect.
15. `build/scribe`: Reconcile affected guarantees after repair.
    - Correct `docs/queue.md` or establish that it remains accurate before evidence repeats.

The downward routes from gates 11 and 13 are alternatives feeding one shared counter of at most two repair passes in Run A.
A nonlocal defect needing renewed design, unavailable evidence, or an exhausted counter returns a blocker immediately.
Every core repair repeats step 9, affected step 10 review, and step 12 before gate 13 can pass.
For a documentation-only repair, retain unchanged code evidence with its tree provenance and repeat the affected source-to-document judgment at step 10.
On re-entry, the owner may satisfy unaffected evidence nodes with still-valid passing reports instead of redispatching their leaves; failed or invalidated evidence must be collected again.
Use fresh children for each repair pass and its renewed independent judgment.

The terminal return contains changed scope, current checks, dissent, repair count, residual risk, and a proposed core commit message.
Preparing that result is part of step 13, not separate packet, audit, and dispatch nodes.

### Attended boundary

Run A ends before any commit or API work.
Collab waits for outer nodes 5 and 6 as well, inspects the joined result, and performs the separately approved foundation/domain commit at outer gate 7 in the human-facing session.
A hook failure follows the attended Git skill's repair boundary; it does not resume the old autonomous graph.
Run B starts only through a new Collab dispatch after the committed baseline and remaining contract are confirmed.
Changed scope, worktree, or design is resolved before that dispatch.

### Run B: expose the committed core through the API

Owner: a new Run B Orchestrator, with its own entry reconciliation and repair counter.
Numbers continue for readability; no live child, old write authority, or unfinished commit crosses the return boundary.

```text
(16) ─→ 17 ─→ 18 ─┬─→ 19 ─┐
                  ├─→ 20 ─┼─→ (22) ─→ <22> ─→ RETURN Collab
                  ├─→ 21 ─┘             │
                  ↑                     │
                  └───── 24 ←─ 23 ←─────┘
```

16. `self`: Bind the fixed public contract to the committed domains and current callers.
    - Confirm the new baseline, API mappings, and requested regression cases; return if any domain behavior contradicts the supplied contract.
17. `build/general`: Implement the settled API mapping.
    - Own only `internal/api/**`, requested API regressions, and cheap checks without reopening core design.
18. `build/scribe`: Document API-visible behavior.
    - Own only `docs/queue.md`, checking errors and cancellation claims against the completed API source.
19. `verify/test`: Run the approved API command.
    - Return the exact command, outcome, and frozen tree identity.
20. `verify/test`: Run the approved cross-boundary command.
    - Use the same frozen tree as step 19 and distinguish defects from unavailable or unrelated evidence.
21. `review/debug`: Inspect public failure semantics.
    - Challenge error mapping, retry reachability, and cancellation across the committed core and changed API.
22. `self`: Synthesize API evidence and return the result.
    - Gate 22 requires all current evidence, retains dissent and core provenance, and either accepts the API boundary, selects bounded repair, or returns a blocker.
23. `build/general`: Repair the named API defect.
    - Edit only `internal/api/**` with cheap checks; defects in committed domains return to Collab.
24. `build/scribe`: Refresh affected API guarantees.
    - Reconcile `docs/queue.md` before the affected evidence wave repeats.

The back edge rejoins the evidence split after step 18; it never jumps directly to gate 22.
At most two repair passes are allowed, each repeating affected commands and judgment while retaining still-valid evidence explicitly.
Documentation-only findings use a no-core/no-API-edit result at step 23 and repeat only affected documentation judgment.
Gate 22's forward edge returns success or a precise blocked result, with changed scope and proposed API commit message.
Neither run's success means Collab has committed, deployed, published, or removed the governing spec.
