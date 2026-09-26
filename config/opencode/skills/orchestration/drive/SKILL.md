---
name: drive
description: Use to prepare or execute an approved multi-step workflow; Collab runs parallel lanes and leaves with explicit check approval, bounded repair, and attended commit or decision boundaries.
---

# Drive

Design an execution workflow, get approval unless the run is armed, then carry it through its dependencies, evidence, and bounded repairs.
Collab runs this procedure in-session and keeps the user conversation, integration, and Git.
Return to `scheme` when execution uncovers unresolved product design.

## Prepare

Start from the terminal outcome and work backward through real contract, implementation, evidence, and attended dependencies.

1. Split the work into coherent scopes that deserve their own lane or leaf.
2. Freeze the interfaces that let scopes run in parallel, and put shared changes in an earlier foundation step or a later integration step.
3. Name each step's agent, lane, model, effort, write scope, checks, and repair limit.
4. Present the workflow as plain numbered steps for approval; in an armed run, write it as your record and start at once.

Scopes can run in parallel lanes in one worktree when their writes are mostly disjoint.
Shared files such as schemas, registries, lockfiles, and generated outputs usually want one owner or a serial order.
A patch that fails because another lane or the user changed the file is normal; the lane re-reads and adjusts.
Stop and report when two scopes turn out to need the same design change.

## Execute

1. Reconcile the tree and Git state, and find the steps whose inputs are ready.
2. Dispatch ready steps on their approved routes, in parallel where they are independent.
3. Judge each result and take the approved next step: continue, repair, or return.
4. Keep todos current with accepted steps, active lanes, repair counts, and blockers.

The approval covers the internal steps, so continue between them without asking again.
Keep owners, checks, models, and repair passes within the approval.
Return to the user when a condition has no approved next step, a repair limit runs out, or evidence breaks a design assumption.
In an armed run, these conditions are your decisions; record each one and continue, as `Armed runs` describes.

## Evidence and repair

Builders own formatting, lint, and other cheap checks inside their scope.
Expensive checks run only when the user approved the command class and cost; in an armed run, you choose the class and record the decision.
Later edits invalidate affected review and verification evidence; repeat only the affected checks.
Send repairs to the lane that built the change, with the finding as the delta.
Use a fresh review leaf when the verdict needs independence from earlier rounds.
After an interruption, reconcile the tree before you reissue write work, because the earlier attempt may have changed files.

## Return boundaries

Commits, rebases, merges, publication, and other external effects are attended boundaries in an unarmed run.
Hold dependent work until the Git boundary it needs is finished.
Update the user after each finished boundary or wave with the verdict, the material change, and the next action.
At the end, report accepted work, changed paths, checks and outcomes, dissent, repair counts, blockers, and residual risk.
At a commit boundary, include the exact scope and a proposed message.

## Armed runs

The user arms drive mode when they invoke `/drive` or `/drive <task>` in a top-level session.
The plugin replaces that message with a "Drive mode armed" note and the task text, if any.
That invocation is the user's grant of authority for this session and its children until the user runs `/drive off` or OpenCode restarts.
Loading this skill yourself does not arm drive mode.
When a drive note arrives with no task, acknowledge it in one line and do nothing else.

- Write the workflow plan in the conversation as your record, then execute it at once without waiting for approval.
- Attended boundaries become your decisions: check classes, repairs, compaction, commits through `build/git`, and rebases.
- The plugin approves each permission ask once, and `question` returns to you with an instruction to decide.
- Deny rules still apply; route around a denial, or return when no route exists.
- Do not end your turn to ask for approval, and do not use `question`.
- Record each decision and any dissent in todos and in the final report.
- Return only at the terminal outcome, or on a hard deny that you cannot route around.
- After compaction, the plugin sends a prompt to continue; resume from the summary.

## Examples

These are shapes, not approved work; `workflow` defines the notation and default routes.
Drive runs longer by chaining phases with frozen interfaces, lanes, and gates.

### Foundation, parallel lanes, integration

```text
                 ┌─→ 3 ─┐
1 ─→ (2) ─→ <2> ─┼─→ 4 ─┼─→ 6 ─→ 7 ─→ <7> ─→ (8) ─→ RETURN user
                 └─→ 5 ─┘   ↑          │
                            └──────────┘ repair ≤2
```

1. `[high • openai/gpt-6-astra] build/owner`, lane `core`: Freeze the shared types and interfaces.
2. `self`: Inspect the foundation; gate 2 releases the parallel lanes only on a stable interface.
3. `build/general`, lane `list`: Build the list view against the frozen interface.
4. `build/general`, lane `detail`: Build the detail view.
5. `build/general`, lane `settings`: Build the settings view.
6. Lane `core`, resumed: Integrate the three views and fix shared wiring.
7. `verify/test`: Run the approved suite.
8. `self`: Report and propose the commit scope and message.

Gate 7 relays failures to the lane that owns them, then repeats steps 6 and 7.
Lanes 3 to 5 write disjoint files; shared registries stay with lane `core`.

### A queue of independent requests

```text
        ┌─→ 2 ───────┐
        ├─→ 3 ─→ 4 ──┤
(1) ────┼─→ (5) ─────┼─→ (7) ─→ RETURN user
        └─→ 6 ───────┘
```

1. `self`: Triage four queued requests and name a lane for each delegated one.
2. `build/patch`, lane `rename`: Apply the settled rename across callers.
3. `scout/library`: Check whether the stdlib already covers the cache helper.
4. `build/general`, lane `cache`: Implement the cache change using step 3's answer.
5. `self`: Make the one-line config fix directly.
6. `[high • anthropic/claude-fable-5-1] build/owner`, lane `sync`: Own the hard sync rewrite.
7. `self`: Review each result and report per request.

Collab dispatches the ready steps in one message and waits for them today.
With OpenCode 2 background tasks, the same lanes can run while the conversation continues.

### Long AFK run with a commit boundary

```text
         ┌─→ 2 ─→ 4 ─┐
(1) ─┬───┤           ├─→ (6) ─→ <6> ─→ RETURN user
     ↑   └─→ 3 ─→ 5 ─┘           │
     └───────────────────────────┘ repair ≤2
```

1. `self`: Freeze the baseline, write scopes, and approved checks.
2. `build/owner`, lane `store`: Migrate the storage layer.
3. `build/general`, lane `cli`: Update the CLI commands.
4. `[medium • anthropic/claude-opus-5-5] review/debug`: Review lane `store`.
5. `[high • openai/gpt-6-sol] review/simplify`: Review lane `cli`.
6. `self`: Relay findings; gate 6 repairs or returns at the commit boundary.

Repairs re-enter at the fork but resume only the lanes that have findings.
In an unarmed run, children run unattended, so any permission ask becomes a blocker for gate 6.
An unarmed run ends at gate 6 because a commit needs attended approval.
In an armed run, the plugin approves child asks, and Collab approves step 7 itself and continues.

```text
you: 7 ─→ (8) ─→ 9 ─→ 10 ─→ (11) ─→ RETURN user
```

7. You approve the commit.
8. `self`: Commit with `commit`.
9. Lane `store`, resumed: Build the API on the committed baseline.
10. `verify/test`: Run the approved suite.
11. `self`: Report the final result.

Phase two starts only after the commit lands, so every lane builds on a known baseline.
