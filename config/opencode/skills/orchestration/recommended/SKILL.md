---
name: recommended
description: Use when the user invokes /recommended, or before asking for an approval that bundles several decisions or follows compaction, repair rounds, or a scope shift the user hasn't seen restated; Collab restates thesis, findings, decisions, and uncertainties, then asks independent questions with recommendations in the same reply unless answering the user's own question.
---

# Recommended

Give the user common knowledge of what Collab is deciding and why, so they can steer before more work lands.
Collab restates first and asks at the end of the same reply, _so the user need not send another message to reach the form_.
A focus argument narrows Findings and Open to that topic; Thesis still covers the whole session.

## Rebuild

Rebuild the thesis from the user's own words, not lane conclusions or earlier restatements.
After compaction, always send one `scout/session` leaf over the session history since the newest finished workflow listed in the digest.
The leaf reports the user's asks and decisions, questions still open, and uncommitted work with owners.
Rebuild the thesis from that report plus the digest's user messages and question answers.
Check every approval Collab is acting on against what the user actually said.
Answer any question or "why" the user left pending before restating anything.
Do not repeat a question the newest reply already asked; if nothing changed, say so and briefly restate Thesis and Open only.

## Restate

Open with one short sentence tying the work to the ask, then use these sections:

- **Thesis**: at most three one-line bullets, never a paragraph: the ask in the user's words, Collab's current understanding, and whether the work still serves it.
  - Nest each scope shift and who made it; an unasked shift is a finding, not a settled fact.
- **Findings**: only what changed a decision; tag only inferred or lane-reported facts in children.
  - An untagged finding means it was measured or read in source.
- **Decided**: outcomes the user would notice or might object to; nest each reason.
- **Open**: user decisions, headed by Collab's recommendation; nest when another option wins.
- **Uncertainties**: what could still be wrong or remains unverified; nest what would show it.

**Use one row per item.**
Give each item a bold headline and a short clause within about 85 display cells, including its marker and indent.
Nest reasons, evidence, alternatives, and checks in child bullets, _so each row scans alone_.

When a workflow is **stored**, call `workflow` and restate from its status; write a new fence only when the plan changes.
Without a stored workflow, show pending work in a `workflow` fence before the TL;DR, following the `workflow` skill.
In a new or changed card, put open decisions that gate work in `you` steps, and use `if:` for work that depends on one answer.
Omit a new card when nothing but the user's decisions is pending.
“Write for a reader returning cold” governs prose; in the card, lane names appear as pills by design, and each headline says what its step does.

Close the restatement with `## TL;DR`: the thesis and top recommendation in one sentence fitting one row.
When something is open, say the questions follow, unless the user's own question defers them to the next reply.

**Own the mechanism.**
Inside approved work, implementation choices belong to Collab and go under Decided.
Leave under Open only the user's workflow and preferences, scope, anything that replaces or discards their work, approval boundaries, and the repository's ask-first items.

**Write for a reader returning cold.**
Replace lane names, round and step numbers, file names, and internal labels with what they do, or define each on first use.

**Split bundles.**
Each open decision stands alone, so answering one never changes another.
Break an approval that covers several decisions into its parts, and move the parts Collab owns to Decided.
When one decision matters only under one answer to another, keep the first under Open and hold the second until it is answered.

## Ask

Ask open decisions with the `question` tool after the TL;DR, at the end of the same reply.
When the user asks a question, including in the message that invoked this skill, answer it in text and ask on the next reply instead.
The synthetic compaction follow-up is not the user's message and does not trigger this delay.
On a reply to the form, apply any steering first and drop the open decisions it settled.
Restate any steering, including a typed custom answer, in a short paragraph before acting.

Precede the call with one sentence naming what it asks; the restatement already carried the context.
Ask every remaining open decision in that one call, most important first.
Each question restates the context it needs in one line, _because the form hides the restatement_.
Put the recommended option first, and let each option's description say what happens next.
More than four open decisions usually means Collab is asking what it should decide.

Answers approve only the steps they name; an unanswered decision stays open and its step waits.
If the user closes the form without answering, keep the decisions open as listed in the restatement and their steps waiting; do not ask again.
When nothing is open, skip the call and continue with the stated decisions inside approved work.
If the handoff brief or summary marks a driven run active, no final report has ended it, and the user's messages since then have not stopped it or moved to another task, load `drive` first and follow its **Driven runs** rules: decide open in-scope items, record them under Decided, skip routine approval, and continue in the same turn.
