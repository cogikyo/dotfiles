---
name: drive
description: Use to prepare or execute an approved multi-step workflow; Collab runs parallel lanes with bounded repair and attended commit or decision boundaries.
---

# Drive

Design an execution workflow, get approval unless the run is armed, then carry it through its dependencies, evidence, and bounded repairs.
Collab runs this procedure in-session and conserves main-thread context for the user conversation, integration, Git, and gates.
File edits, repairs, and long-output test runs go to lanes by default.
A tiny one-line config or record edit may stay in-session.
Return to `scheme` when execution uncovers unresolved product design.

## Prepare

Start from the terminal outcome and work backward through real contract, implementation, evidence, and attended dependencies.

1. Split the work into coherent scopes that deserve their own lane or leaf.
2. Freeze the interfaces that let scopes run in parallel, and put shared changes in an earlier foundation step or a later integration step.
3. Name the agent, optional lane, and `provider/model#variant` for every delegated step; add `why:` only for a departure from `ROUTING.md`'s route.
4. Name each step's write scope, checks, and repair limit.
5. Present the numbered steps in a `workflow` fence.

Scopes can run in parallel lanes in one worktree when their writes are mostly disjoint.
Shared files such as schemas, registries, lockfiles, and generated outputs usually want one owner or a serial order.
A patch that fails because another lane or the user changed the file is normal; the lane re-reads and adjusts.
Stop and report when two scopes turn out to need the same design change.

## Execute

1. Reconcile the tree and Git state, and find the steps whose inputs are ready. Dispatch them.
2. Judge each result and take the approved next step: continue, repair, or return.

Dispatch ready steps in the background; release dependent work only after required results arrive and are accepted.
The approval covers the internal steps, so continue between them without asking again.
Keep owners, checks, models, and repair passes within the approval.
Return to the user when a condition has no approved next step, a repair limit runs out, or evidence breaks a design assumption.
In an armed run, these conditions are your decisions; record each one and continue, as `Armed runs` describes.

## Evidence and repair

Builders own formatting, lint, and other cheap checks inside their scope.
Later edits invalidate affected review and verification evidence; repeat only the affected checks.
Resume only lanes with findings for repairs, sending each finding as a delta to the lane that built the change.
Use a fresh review leaf when the verdict needs independence from earlier rounds.
After an interruption, reconcile the tree before you reissue write work, because the earlier attempt may have changed files.
Inspect `lanes` too, because a parent interruption can leave background children running.

## Return boundaries

Hold dependent work until the Git boundary it needs is finished, so later lanes use the committed baseline.
An unarmed run stops at the commit gate; an armed run records the commit decision and continues within scope, subject to permission review.
Update the user after each finished boundary or wave with the verdict, the material change, and the next action.
At the end, report accepted work, changed paths, checks and outcomes, dissent, repair counts, blockers, and residual risk.
At a commit boundary, include the exact scope and a proposed message.

## Armed runs

The user arms drive mode with `<leader>d` or `/drive`, and a glyph in the session title marks the armed state, with a `DRIVE` badge as a fallback when the title cannot be decorated.
The plugin adds a system line while armed and hides the `question` tool.
Only the user can change it, and loading this skill yourself does not arm drive mode.
When the line disappears, attended boundaries apply again from the next step.

- Make in-scope execution decisions and record the reason and any dissent in your reply and the final report.
- A reviewer model decides each permission ask before it reaches the user, with `allow`, `deny`, or `ask`.
- Deny rules and explicit user limits still apply; drive mode does not expand scope or grant every permission.
- Adapt to a denial when a permitted action meets the same approved objective, and never retry it through a bypass.
- A reviewer `ask` or failure leaves the permission waiting for the user; report the blocker and continue independent permitted work.
- The plugin journals reviewer decisions, and the user can inspect them with `/drive-log`.
- Do not call `question` or stop only to request routine approval while armed.
- Recover the workflow state from the summary, lanes, and tree before new writes.
