---
name: drive
description: Use to prepare or execute an approved multi-step workflow; Collab runs parallel lanes with bounded repair and attended decision boundaries.
---

# Drive

Design an execution workflow, get approval, then carry it through its dependencies, evidence, and bounded repairs.
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

1. After approval, call **`workflow({ start: true })`** to store the fence; the `workflow` skill owns fence rules.
2. Reconcile the tree and Git state, then launch ready children with **`subagent({ …, step: n })`**.
3. Judge results at gates and record accepted, skipped, or failed steps with **`workflow`**.
4. Continue, repair through the gate, or return within the approved plan.

Dispatch ready steps in the background; release dependent work only after required results arrive and are accepted.
The approval covers the internal steps, so continue between them without asking again.
Keep owners, checks, models, and repair passes within the approval.
Return to the user when a condition has no approved next step, a repair limit runs out, or evidence breaks a design assumption.

## Evidence and repair

Builders own formatting, lint, and other cheap checks inside their scope.
Later edits invalidate affected review and verification evidence; repeat only the affected checks.
Resume only lanes with findings for repairs, sending each finding as a delta to the lane that built the change.
Mark the rejecting gate failed and call **`workflow({ repair: [n…] })`** for its repair targets before relaunching them.
This spends the gate's repair round and reopens the targets and all downstream steps, whose evidence must be renewed.
Use a fresh review leaf when the verdict needs independence from earlier rounds.
After an interruption, reconcile the tree before you reissue write work, because the earlier attempt may have changed files.
Inspect `lanes` too, because a parent interruption can leave background children running.

## Replan

The user's **`/replan`** pauses dispatch and supplies workflow status and each lane's step, status, idle time, context size, and cache warmth.
Load `recommended` and give each lane a reasoned verdict: **resume**, **start fresh**, or **close**.
Use its result and remaining task to judge relevance, and its running state and cache facts to judge reuse; `COLLAB.md` owns cold-lane expiry.
Give fresh children full briefs, and reconcile running children before replacing or closing them.
Ask the open questions, then compact; `/replan` pre-approves that compaction.
After compaction, read stored status first, then start the next workflow revision to unpause dispatch.

## Return boundaries

Hold dependent work until the Git boundary it needs is finished, so later lanes use the committed baseline.
Run `commit` at each Git boundary.
Update the user after each finished boundary or wave with the verdict, the material change, and the next action.
At the end, report accepted work, changed paths, checks and outcomes, dissent, repair counts, blockers, and residual risk.
