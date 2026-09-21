---
name: workflow
description: Use when a proposal or activity example needs a compact numbered graph for concurrent owners, mixed dependencies, conditional joins, bounded repair, or attended return boundaries; ordinary linear steps and source-derived documentation illustrations are outside this skill.
---

# Workflow diagram

Draw a graph when numbered prose would hide a dependency, branch, or loop.
Keep ordinary linear work as numbered steps.
The current owner and approved contract determine authority and routing; this skill supplies notation and checks.
Use `docs` for source-derived architecture diagrams and annotated trees, not execution graphs.

## Number the work

Give each step a unique number, short title, owner, and concise acceptance condition.
For executable delegated work, supply the exact agent, model variant, and effort once per route or beside the step.
Keep write scope, permitted checks, concurrency, and unattended authority in the accompanying contract.
Do not split one coherent assignment by file or turn each cheap check into a leaf.

- `N` is a leaf assignment.
- `(N)` is work performed by the owner of this graph.
- `{N}` is a delegated Orchestrator, permitted only in a Collab-owned graph.
- `<N>` is the current owner's acceptance or decision point for step N.
- `RETURN Collab` or `RETURN user` is a terminal result, never a dispatch.

Label the graph's owner.
An Orchestrator can contain only its own work and leaves; it cannot dispatch another Orchestrator or Collab.
A node and its gate share a number, but a completed assignment does not automatically need a drawn gate.

> [!IMPORTANT] Draw decisions that matter
>
> Use `<N>` where the owner selects a conditional route, accepts a consequential boundary, or judges repair versus return.
> Routine receipt of a leaf report belongs to execution bookkeeping, not another graph node.

## Connect the topology

Use a fenced, compact Unicode drawing with light rails, corners, tees, and directional arrows.
Align vertical trunks by display column and attach every branch to its trunk.
Keep the graph mostly glyphs and node numbers; put conditions and acceptance evidence in numbered prose.
An edge expresses a dependency, not merely chronological proximity.
Do not substitute a list of edge statements for the connected drawing.

A split permits concurrency unless its numbered decision selects alternatives.
A join waits for every required predecessor; an alternative merge takes only the selected route.
State that distinction beside conditional examples because the geometry alone cannot encode it.
Failed required evidence cannot satisfy a success join, though a complete failure report can feed an owner's diagnosis.
Reroute unrelated crossings; `┼` means a real connected junction.

Every loop names its repair owner, total attempt limit, affected evidence, and re-entry point.
Denied permission, exhausted repair, missing approval, and invalid design assumptions return a blocker.
Split long workflows at coherent phases and name the exact preceding output that each phase consumes.
An attended Git boundary ends the child run; a later run needs a new Collab dispatch and baseline.

## Example series

These are shape fixtures, not approved work in this repository.
For these examples, Orchestrator uses `[high • openai/gpt-5.6-sol-fast]`, `scout/context` uses `[xhigh • openai/gpt-5.6-luna-fast]`, and `verify/source` and `verify/test` use `[high • xai/grok-4.7]`.
Settled `build/general` implementation and `build/patch` repair use `[high • xai/grok-4.7]`.
Owners pass `unattended: true` and exact authority in task fields and briefs, tightening evidence leaves to read-only.
These route labels are example contracts; they neither grant check approval nor override an explicit task route.

### Concurrent fan-out and required join

Owner: Orchestrator, with at most two read-only leaves.

```text
        ┌─→ 2 ─┐
(1) ────┤      ├─→ (4) ─→ RETURN Collab
        └─→ 3 ─┘
```

1. `self`: Frame the question and freeze the inspected baseline.
2. `scout/context`: Trace the reachable cancellation paths, with source locations.
3. `verify/source`: Verify the named cached dependency guarantee, with its exact version.
4. `self`: Reconcile both required reports and return the answer or missing evidence.

No extra gates are useful here: step 4 already owns synthesis and the terminal result.

### Mixed dependencies

Owner: Orchestrator, with at most two leaves at once.

```text
        ┌─→ 2 ────────┐
(1) ────┤             ├─→ (5) ─→ RETURN Collab
        └─→ 3 ─→ 4 ───┘
```

1. `self`: Fix two independent evidence questions.
2. `scout/context`: Trace shutdown reachability across the supplied source boundary.
3. `scout/context`: Identify the exact renewal primitive and pinned ref.
4. `verify/source`: Verify that primitive's cancellation semantics using step 3's locations.
5. `self`: Combine step 2's caller evidence with step 4's guarantee, retaining step 3's provenance.

Step 4 can start while step 2 is still running; it cannot start before step 3 returns.

### Conditional alternative

Owner: Orchestrator, with at most one leaf.

```text
(1) ─→ <1> ─┬─→ 2 ─┐
            │      ├─→ (3) ─→ RETURN Collab
            └──────┘
```

1. `self`: Compare the changed claims to the supplied source evidence; gate 1 selects step 2 only when an external guarantee changed.
2. `verify/source`: Verify that changed guarantee against the approved cached version, or report unavailable evidence.
3. `self`: Judge the selected evidence and return a supported conclusion or blocker.

The lower route uses still-valid supplied evidence and skips step 2.
The merge is exclusive: step 3 never waits for the unselected branch.

### Bounded repair through affected evidence

Owner: Orchestrator, with serialized writes and at most one leaf at a time.

```text
1 ─→ 2 ─→ (3) ─→ <3> ─→ RETURN Collab
     ↑            │
     └──── 4 ←────┘
```

1. `build/general`: Implement the approved change and run its cheap local checks.
2. `verify/test`: Collect the explicitly approved independent evidence on the frozen changed tree.
3. `self`: Accept current evidence, return a blocker, or select one bounded defect for repair at gate 3.
4. `build/patch`: Apply the supplied settled defect correction and repeat affected cheap checks, then re-enter step 2 for affected independent evidence.

Gate 3 permits at most one repair pass, using fresh children for step 4 and renewed step 2 evidence.
Its forward edge returns success when evidence passes, or a blocker when permission is denied, the bound is exhausted, or the defect escapes scope.
Neither a repair report nor old passing evidence bypasses step 2.

### Mixed ownership and an attended return

Owner: Collab, with at most two direct children and no overlapping writes.
Assume an approved core migration plus a read-only caller inventory; no checks or Git actions are authorized by this fixture itself.
Use the example routes above: Sol Fast high Orchestrator, Luna Fast xhigh `scout/context`, and Grok high `build/general`.
The migration owner may use `[high • openai/gpt-6-astra] build/owner` for its coupled state migration, with the same Luna evidence routes and no further owners.

```text
        ┌─→ {2} ─┐
(1) ────┤        ├─→ (4) ─→ <4> ─→ 5 ─→ (6) ─→ RETURN user
        └─→ 3 ───┘
```

1. `self`: Supply the approved baseline, disjoint scopes, child authority, and check contract.
2. Orchestrator owns the core migration and returns its result to this Collab instance without committing or creating another Orchestrator.
3. Scout caller contracts from an unchanged caller boundary without reading files being written by step 2.
4. `self`: Reconcile both results and accept the core boundary; perform a commit here only if separately approved, otherwise return when that commit is required.
5. Update callers only after gate 4 supplies the accepted baseline, with cheap checks owned by this builder.
6. `self`: Inspect the complete result and return it to the user, including any remaining attended Git boundary.

Step 5 is a new Collab dispatch, never continuation inside step 2's terminated run.
Inside step 2, its Orchestrator uses `(N)` and bare leaves, not `{N}`.
For a simple prose update, remove the migration machinery and let one writer own the concern.

## Check the drawing

Before presenting or executing it:

1. Compare the graph's unique step numbers to the list, allowing a node and its gate to repeat the same identifier.
2. Confirm every gate makes a real owner decision and every delegated node fits the role's permissions.
3. Trace required joins and each alternative separately, including the route that skips optional work.
4. Count runnable leaves at each split and compare their read/write sets for overlap.
5. Trace repair back through all invalidated evidence and verify its bound and blocked return.
6. Check that return events terminate runs before attended actions or new dispatches.
7. Read each vertical column through corners and tees; reject gaps, false crossings, tabs, and detached arrow tails.

Workflow graphs do not require node boxes, centered titles, opaque-span registries, or the `docs` canvas workflow.
For ASCII labels and these narrow Unicode glyphs, a small standard-library check can reject controls and wide labels, measure rows under the explicit narrow-ambiguous assumption, and inspect trunk columns.
Use an existing display-width library when labels include wide or combining characters; do not install a dependency just to draw a workflow.
Keep rows within the host budget and verify the hosted fence after editing.
Source geometry checks do not establish target-renderer compatibility; claim that only after observing the rendered graph.
