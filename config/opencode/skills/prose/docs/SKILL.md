---
name: prose-docs
description: Use with prose to write or shorten READMEs, guides, usage notes, and repository documentation; admits task-relevant information and removes cross-section duplication before sentence polishing.
---

# Repository documentation

Read `prose` for audience and source-fidelity procedure.
Let the intended reader's task determine the document's scope and sections; there is no mandatory README outline.

## Admit information

Keep a block only if it enables a reader task or decision, prevents a likely mistake, or supplies a mental model needed for what follows.
A true statement can still be unnecessary here.
Introduce purpose, architecture, or an operating model only when the reader needs that context; do not add those sections by habit.
Organize around reader needs rather than mirroring source layout unless the layout itself answers their question.

Compare claims across the whole document before polishing sentences.
Delete redundant sections and move any unique qualification to the best owner of the claim.
Keep one authoritative explanation where practical, but do not replace usable local instructions with a maze of links.
An independently entered procedure still needs its prerequisites and hazards beside the relevant action.

## Make the task usable

Put prerequisites and destructive consequences before the action that depends on them.
Order commands as the reader must run them, with a useful expected result or verification step when completion would otherwise be unclear.
Use exact source terms and define unfamiliar concepts where the reader first needs them.
Preserve exceptions and operational limits that affect the task.

Let examples, commands, tables, and diagrams carry meaning without narrating everything they already show.
Add explanation only for useful interpretation, a non-obvious consequence, or a necessary constraint.
Choose a tree for load-bearing hierarchy and a diagram for load-bearing relationships; inspect source for every path, node, label, and edge.
Load the diagram skill `docs` for construction only after deciding the representation earns its place.

Use callouts sparingly for a real hazard, constraint, or surprising fact, and only with types supported by the renderer.
Use `INFO` for useful separated context and reserve `IMPORTANT` for omissions that could cause a wrong decision, unsafe action, or broken result.
Put a concise subject on the marker line, such as `> [!IMPORTANT] Data removal`, followed by a blank quoted line and a short paragraph or brief list.
Quote every body and blank line; use a normal section when several paragraphs or unrelated ideas are needed.

## Deletion judgment

If a README introduction says "Run `serve` to start the local server" and its usage block already shows that command, remove the sentence unless it contributes a prerequisite or meaningful result.
Do not replace it with a shorter slogan.

If both an architecture section and a deployment section explain worker ownership, keep the explanation where readers need to understand ownership.
Move a deployment-only exception beside the deployment action before deleting the duplicate section.

If a reset guide is an independent entry point, keep "This deletes local data" immediately before its reset command even when a reference page documents the same effect.
The local warning prevents a mistake; a link alone would not do that job.

Finish with a cold-reader pass against the inspected source and the complete document.
Check that deletion preserved necessary conditions and qualifications, examples remain valid, and every surviving section earns its space.
