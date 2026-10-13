---
name: clear-lanes
description: Use when the user invokes /clear-lanes or asks to close stale delegate lanes; Collab judges which named lanes in the current session are finished and closes them with lanes_close so only active lanes remain.
---

# Clear lanes

Collab owns this procedure and runs it without asking for each lane.
Closing a lane hides it from the sidebar and frees its name: the next `subagent` call with that name starts a fresh child.
A closed lane can't be resumed, including by its `sessionID`, and every permission on the closed child is denied.
Its history stays, so a wrong close costs a re-brief.

## Read

1. Run `lanes` for the current session.
2. Collect each named lane with its status, agent, last activity, and session ID.
3. Recall from this conversation and the todo list which lanes still own open work.

Skip one-shot children without a lane name, lanes already `closed`, and lanes that are `running`.
The `lanes` tool still lists closed children with their old names.

## Judge

Close a lane when any of these holds:

- Its work was accepted, committed, or abandoned, and no open todo or pending decision names it.
- A newer lane took over its scope, such as a re-brief under a new name.
- It exists only for a finished demo, probe, or smoke check.

Keep a lane when it has an open repair round, awaits user feedback, or a planned next step names it.
When the evidence is mixed, keep the lane and list it as a candidate.
User arguments override this judgment: close exactly the lanes they name, or keep the ones they protect.

## Close

Call `lanes_close` once with a `lanes` array of every name to close.
It never closes a running child and returns one line per name: `closed <name>` or `skipped <name>: <reason>` for running, missing, or already closed lanes.
Report skipped lanes with the reason the tool returned.

## Report

Lead with the counts: closed, kept, and skipped.
List closed lanes with one short reason each, then kept lanes with the work that keeps them open.
