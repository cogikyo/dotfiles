---
name: clear-lanes
description: Use when the user invokes /clear-lanes or asks to close stale delegate lanes; Collab judges which named lanes in the current session are finished and closes them with task_close so only active lanes remain.
---

# Clear lanes

Collab owns this procedure and runs it without asking for each lane.
Closing a lane hides it from the sidebar and frees its name: the next `task` call with that name starts a fresh child.
A closed child keeps its history, so a wrong close costs a re-brief, not lost work.

## Read

1. Run `task_status` for the current session.
2. Collect each named lane with its status, agent, title, last update, and any context limit.
3. Recall from this conversation and the todo list which lanes still own open work.

Skip one-shot children without a lane name, lanes already `closed`, and lanes that are `busy` or `retry`.

## Judge

Close a lane when any of these holds:

- Its work was accepted, committed, or abandoned, and no open todo or pending decision names it.
- It hit a context limit, so the next call starts fresh anyway.
- A newer lane took over its scope, such as a re-brief under a new name.
- It can no longer resume, such as after a permission envelope or execution contract mismatch.
- It exists only for a finished demo, probe, or smoke check.

Keep a lane when it has an open repair round, awaits user feedback, or a planned next step names it.
When the evidence is mixed, keep the lane and list it as a candidate.
User arguments override this judgment: close exactly the lanes they name, or keep the ones they protect.

## Close

Call `task_close` once with every lane to close.
Report skipped or failed lanes with the reason the tool returned.

## Report

Lead with the counts: closed, kept, and skipped.
List closed lanes with one short reason each, then kept lanes with the work that keeps them open.
