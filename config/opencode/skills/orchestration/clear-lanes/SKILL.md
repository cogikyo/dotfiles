---
name: clear-lanes
description: Use when the user invokes /clear-lanes or asks to close stale delegate lanes; Collab closes finished named lanes with lanes_close and reports interrupted or stalled one-shots in the current session.
---

# Clear lanes

Collab owns this procedure and runs it without asking for each lane.
Its history stays, so a wrong close costs a re-brief.

## Read

1. Run `lanes` for the current session.
2. Collect each child, including unnamed one-shots, with its status, agent, last activity, and session ID.
3. Recall from this conversation which lanes still own open work.

Skip successfully completed one-shots and lanes already `closed`; keep running children open, but report suspected stalls.
The `lanes` tool still lists closed children with their old names.

## Judge

Close a lane when any of these holds:

- Its work was accepted, committed, or abandoned, and no remaining work or pending decision depends on it.
- A newer lane took over its scope, such as a re-brief under a new name.
- It exists only for a finished demo, probe, or smoke check.

Keep a lane when it has an open repair round, awaits user feedback, or a planned next step names it.
When the evidence is mixed, keep the lane and list it as a candidate.
User arguments override this judgment: close exactly the lanes they name, or keep the ones they protect.

## Close

Call `lanes_close` once with a `lanes` array of every name to close.
Report skipped lanes with the reason the tool returned.
It accepts only named lanes; do not claim it ended an unnamed one-shot.

## Report

Lead with the counts: closed, kept, and skipped.
List closed lanes with one short reason each, then kept lanes with the work that keeps them open.
Also list interrupted or stalled one-shots by agent and session ID so the user can judge lost work; do not claim nothing is running while one is still running.
