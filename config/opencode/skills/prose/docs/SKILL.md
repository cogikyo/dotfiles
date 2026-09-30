---
name: prose-docs
description: Use with prose to write or shorten READMEs, guides, usage notes, repository documentation, agent prompts, and skills; admits task-relevant information and removes cross-section duplication before sentence polishing.
---

# Repository documentation

Read `prose` for audience and source-fidelity procedure.
This skill covers repository documentation, agent prompts, and skills read by agents and humans.
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

## Structure and emphasis

Apply `AGENTS.md`'s paragraphs-and-lists rule: paragraphs explain connected reasoning, flat lists enumerate peers, and nesting shows dependence.

- Use **paragraphs** for connected explanation, causal reasoning, framing, or synthesis; do not turn connected prose into bullets only to make it look scannable.
- Use **flat lists** for peer items that answer the same question and can be scanned independently.
- Use **nested bullets** when a child explains, qualifies, exemplifies, or operationalizes its parent.
  - The parent states the main idea, and the child depends on it for useful context.
- Use **numbered lists** only when sequence, priority, or dependency matters.
- Keep each **list item** to one sentence and preferably one rendered line.
  - Move sustained reasoning into a nearby paragraph instead of expanding the bullet.
- Give a **conceptual section** a paragraph when readers need to understand why its ideas connect.
- Let a self-explanatory **catalog, checklist, or rule family** remain list-only.
- Use **bold** for a rule's key term so a reader scanning the file lands on it; never bold whole sentences.
- Use *italics* for the short human reason so agents and humans can see why the rule matters to the reader.

Use `INFO` callouts for principles or a file's defining rule that the reader must internalize.
Use `IMPORTANT` for a rule whose omission could cause a wrong decision, unsafe action, or broken result.
Use only types supported by the renderer.

Put a concise title on the marker line, such as `> [!INFO] Humility`, followed by a blank quoted line, a one-line thesis, and at most a few short lines or bullets.
Quote every body and blank line in the callout.
Put the reasoning in a short plain paragraph immediately after the callout, outside it.
Use `---` to separate a series of peer callouts when useful, as in `AGENTS.md`'s Core Principles.

Keep one callout per concept; do not wrap ordinary content or whole sections.
Do not flatten or remove existing callouts unless the brief asks you to.

## Deletion judgment

If a README introduction says "Run `serve` to start the local server" and its usage block already shows that command, remove the sentence unless it contributes a prerequisite or meaningful result.
Do not replace it with a shorter slogan.

If both an architecture section and a deployment section explain worker ownership, keep the explanation where readers need to understand ownership.
Move a deployment-only exception beside the deployment action before deleting the duplicate section.

If a reset guide is an independent entry point, keep "This deletes local data" immediately before its reset command even when a reference page documents the same effect.
The local warning prevents a mistake; a link alone would not do that job.

Finish with a cold-reader pass against the inspected source and the complete document.
Check that deletion preserved necessary conditions and qualifications, examples remain valid, and every surviving section earns its space.
