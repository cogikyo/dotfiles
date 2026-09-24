---
name: workflow
description: Use when a proposal or run needs a compact numbered graph for parallel lanes, mixed dependencies, conditional routes, bounded repair, or attended return boundaries; ordinary linear steps and source-derived documentation illustrations are outside this skill.
---

# Workflow diagram

Draw a graph when numbered prose would hide a dependency, branch, parallel lane, or loop.
The user reads these graphs to approve and follow work, so prefer drawing one whenever the shape is more than a straight chain.
The approved contract sets authority and routing; this skill supplies notation and checks.
Use `docs` for source-derived architecture diagrams and annotated trees, not execution graphs.

## Number the work

Give each step a unique number, short title, owner, and concise acceptance condition.
For delegated work, name the agent, lane, model variant, and effort once per route or beside the step.
Keep write scope, permitted checks, and parallelism in the numbered prose.
Avoid splitting one coherent assignment by file, and keep cheap checks inside the step that owns them.

- `N` is a delegated step: a one-shot leaf, or a lane when the prose names one.
- `(N)` is work Collab does in the session.
- `<N>` is Collab's acceptance or decision point for step N.
- `you: N` is a step the user performs, such as a restart or smoke test.
- `RETURN user` is a terminal result.

A node and its gate share a number.
Draw `<N>` only where Collab selects a route, accepts a consequential boundary, or chooses repair versus return.
Routine receipt of a report is bookkeeping and needs no gate.

## Connect the topology

Use a fenced, compact Unicode drawing with light rails, corners, tees, and directional arrows.
Align vertical trunks by display column and attach every branch to its trunk.
Keep the graph mostly glyphs and numbers; put conditions and acceptance evidence in the numbered prose.
An edge means a dependency, not only chronological order.

A split means parallel work unless its gate selects one alternative.
A join waits for every required predecessor; an alternative merge takes only the selected route.
State that difference beside conditional graphs, because geometry alone cannot show it.
Reroute unrelated crossings; `┼` means a real connected junction.

Every loop names its repair owner, attempt limit, affected evidence, and re-entry point.
Repairs usually resume the same lane with the findings as a delta.
Denied permission, exhausted repair, and missing approval return a blocker to the user.
An attended Git boundary is a gate owned by Collab, with its own approval.

## Examples

These are shape examples, not approved work.
Routes: `scout/*` uses `[xhigh • openai/gpt-6-luna-fast]`, builders and `verify/*` use `[high • xai/grok-4.6]`, and `review/*` uses `[high • openai/gpt-6-sol]`.

### Parallel fan-out and required join

```text
        ┌─→ 2 ─┐
(1) ────┤      ├─→ (4) ─→ RETURN user
        └─→ 3 ─┘
```

1. `self`: Frame the question and note the baseline.
2. `scout/context`: Trace the reachable cancellation paths, with source locations.
3. `verify/source`: Verify the named dependency guarantee, with its exact version.
4. `self`: Reconcile both reports and return the answer or the missing evidence.

Step 4 already owns synthesis, so no gate is drawn.

### Mixed dependencies

```text
        ┌─→ 2 ────────┐
(1) ────┤             ├─→ (5) ─→ RETURN user
        └─→ 3 ─→ 4 ───┘
```

1. `self`: Fix two independent evidence questions.
2. `scout/context`: Trace shutdown reachability across the named boundary.
3. `scout/context`: Find the renewal primitive and its pinned ref.
4. `verify/source`: Verify that primitive's cancellation semantics from step 3's locations.
5. `self`: Combine step 2's callers with step 4's guarantee.

Step 4 can start while step 2 runs, but not before step 3 returns.

### Conditional alternative

```text
(1) ─→ <1> ─┬─→ 2 ─┐
            │      ├─→ (3) ─→ RETURN user
            └──────┘
```

1. `self`: Compare changed claims to supplied evidence; gate 1 selects step 2 only when an external guarantee changed.
2. `verify/source`: Verify that guarantee, or report that evidence is unavailable.
3. `self`: Judge the selected evidence and return a conclusion or blocker.

The merge is exclusive: step 3 never waits for the skipped branch.

### Lane with bounded repair

```text
1 ─→ 2 ─→ (3) ─→ <3> ─→ RETURN user
↑                 │
└─────────────────┘ repair ≤2
```

1. `build/general`, lane `parser`: Implement the approved change and run its cheap checks.
2. `review/debug`: Review the change against the approved behavior.
3. `self`: Accept, return a blocker, or relay supported findings to lane `parser` at gate 3.

Each repair resumes lane `parser` with the findings, then repeats step 2 on the changed code.
Old review evidence never satisfies gate 3 after a repair.

### Parallel lanes and an attended commit

```text
         ┌─→ 2 ─→ 4 ─┐
(1) ─┬───┤           ├─→ (6) ─→ <6> ─→ you: 7 ─→ (8) ─→ RETURN user
     ↑   └─→ 3 ─→ 5 ─┘           │
     └───────────────────────────┘ repair ≤2
```

1. `self`: Freeze the baseline and the two write scopes.
2. `build/owner`, lane `plugins`: Implement the plugin side and run its cheap checks.
3. `build/owner`, lane `prose`: Rewrite the instructions and run `git diff --check`.
4. `review/debug`: Review lane `plugins`.
5. `review/simplify`: Review lane `prose`.
6. `self`: Relay findings; gate 6 repairs or moves on after a final stale-reference sweep.
7. You restart OpenCode and smoke-test.
8. `self`: Commit with `commit` after separate Git approval.

Lanes 2 and 3 run in parallel and write disjoint files.
Repairs re-enter at the fork but resume only the lanes with findings, at most twice each.

## Check the drawing

Before presenting or executing it:

1. Match the graph's numbers to the list; a node and its gate may share one.
2. Confirm every gate is a real decision and every delegated node fits its role's permissions.
3. Trace each join and each alternative, including routes that skip optional work.
4. Compare parallel lanes' write sets for overlap.
5. Trace each repair loop through its invalidated evidence, bound, and blocked return.
6. Read each vertical column through corners and tees; reject gaps, false crossings, tabs, and detached arrow tails.

Workflow graphs need no boxes, titles, or the `docs` canvas procedure.
Keep rows within the host width, and verify the rendered fence after editing.
