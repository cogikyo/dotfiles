---
name: workflow
description: Use to write workflow fences for proposals with parallel lanes, conditions, joins, or bounded repair, and launches of two or more children; the TUI renders the card and dependency graph.
---

# Workflow fence

Follow `COLLAB.md`'s Delegation rule for proposals, launches, and approved waves.
The approved contract sets authority and routing; this skill defines the declarative fence.
Use `diagram` for source-derived architecture diagrams and annotated trees.

## Declare the work

Every `workflow` fence has a title, then a required summary of one to three sentences on what the work achieves and why it matters to the user; never restate the title or describe scope, constraints, shape, or step interactions.
The renderer word-wraps the summary; nothing else wraps.
Blank lines are ignored; the renderer supplies the graph and styling.
It flags invalid declarations inline, including unknown keys or step numbers, duplicate numbers, and missing child models.
Start each step at column zero as `number owner: headline`, with a unique positive integer, normally in order.
Write **every headline**, including the workflow heading, as **`Title: context`**.
Use a **Title Case** title of **2–4 words** and at most **20 display cells**; the renderer emphasizes it.
`start` refuses colon-less headlines over 20 cells.
Keep checklist items under about 70 characters, with no trailing periods.
Write the fence in **reply text** before `workflow({ start: true })`; it cannot read a fence that exists only in reasoning.

- **Owners:** `self` for Collab's work, `gate` only for Collab's own decisions, and `you` for user actions and approvals, including route changes.
- **Children:** use `role/scope [lane] provider/model[#variant]` as the owner.
  - Roles are `build`, `review`, `scout`, and `verify`; scope is the agent name after the slash.
  - The optional lane is a kebab-case name for its concern, such as `preset-tests`; omit it for a one-shot child.
  - Require the model token to match the child's launch; add `why:` only for a departure from `ROUTING.md`'s routes and guidance.
- **Details:** indent each line by two spaces under its step.
  - Use `- text` for what the step does; builder checklists name the files or behavior they change.
  - Use `done: text` only for specific acceptance beside a checklist, never as the only detail line; omit generic verdicts.
  - Use `after: 1 2` for dependencies; omission defaults to the previous step.
  - Give **parallel steps explicit `after:`** dependencies, or `after: none` for independent roots.
  - Joins wait for all predecessors; `if: condition` steps form an exclusive alternative and are required only when selected.
  - Use `repair: 1 max 2` on a gate for bounded repair, or `repair: 1 2 max 2` for parallel lanes that each resume only when they have findings.
  - Repair targets must be **upstream** of their gate.

Gate and review checklists name concrete risks, invariants, and failure modes to look for in evidence, never the inputs they read.
Approvals, commits, and Collab bookkeeping are headline-only; omit checklists, `done:`, and `if:` for conditions already implied by Collab's process.
Repair routing, limits, reruns, and evidence invalidation are process mechanics, not step prose.

## Process judgment

Size each workflow to the headroom `workflow` reports.
Judge the actual work: wide, cheap steps can fit where a few deep sequential steps cannot.
End each workflow at a commit, compaction into the next workflow, a `you` gate, or done.

Keep cheap checks inside their owning step, avoid splitting one assignment by file, and check parallel write scopes for overlap.
A gate marks only Collab's routing, acceptance, or repair-versus-return decision; routine report receipt needs none.
Name each repair's owner and invalidated evidence in Collab's repair brief, not the card; record its limit through `repair: … max K`.
Usually resume the same lane with findings as a delta, then repeat the affected checks.
Tie dependency guarantees to source locations and the exact version in use.
Denied permission, exhausted repair, and missing approval return a blocker unless an approved permitted route remains.
