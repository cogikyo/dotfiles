---
name: docs
description: Use with prose to write or shorten READMEs, guides, usage notes, repository documentation, agent prompts, and skills.
---

# Repository documentation

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
Load `diagram` for construction only after deciding the representation earns its place.

Leave worked examples out of agent prompts and skills; agents copy their surface, and every load pays for them in context.
When a rule seems to need an example, rewrite the rule until it stands alone.
Notation is the exception: a diagram, graph, or format whose shape words cannot carry may show one sample.
This rule is a trial; `retro` proposes removing examples where they caused problems.

Instruction files state the current rule as fact: `AGENTS.md`, `COLLAB.md`, `ROUTING.md`, agent prompts, and skills.
Leave out dates, scores, test results, and how a rule was learned; the commit message holds those.

## Structure and emphasis

Use _italics_ for a short reason only when it covers cases the rule doesn't name.

Use `INFO` callouts for principles or a file's defining rule that the reader must internalize.
Use `IMPORTANT` for a rule whose omission could cause a wrong decision, unsafe action, or broken result.
Use only types supported by the renderer.

Put the one-line thesis on the marker line as the title, followed by a blank quoted line and at most a few short lines or bullets.
Quote every body and blank line in the callout.
Put the reasoning in a short plain paragraph immediately after the callout, outside it.
Give peer callouts their own headings, as the virtues in `AGENTS.md` do.

Keep one callout per concept; do not wrap ordinary content or whole sections.
Do not flatten or remove existing callouts unless the brief asks you to.
