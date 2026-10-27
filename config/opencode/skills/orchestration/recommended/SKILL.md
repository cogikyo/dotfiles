---
name: recommended
description: Use when the user invokes /recommended, or before asking for an approval that bundles several decisions or follows compaction, repair rounds, or a scope shift the user hasn't seen restated; Collab restates thesis, findings, decisions, and uncertainties, then asks independent questions with recommendations in the same reply unless answering the user's own question.
---

# Recommended

Give the user common knowledge of what Collab is deciding and why, so they can steer before more work lands.
Collab restates first and asks at the end of the same reply, _so the user need not send another message to reach the form_.
A focus argument narrows Findings and Open to that topic; Thesis still covers the whole session.

## Rebuild

Rebuild the thesis from the user's own messages, not from compaction notes, lane reports, or earlier restatements.
After compaction, read this session's user messages with `sessions` (`text`, `role: user`) first, _because the summary paraphrases all but the latest one_.
Check every approval Collab is acting on against what the user actually said.
Answer any question or "why" the user left pending before restating anything.

## Restate

Open with one sentence placing the work against the original ask, then write these sections:

- **Thesis**: the problem in the user's words, Collab's current understanding, and whether the current work still serves it.
  - Name each scope shift since the ask and who made it; an unasked shift is a finding, not a settled fact.
- **Findings**: only what changed a decision, each with how Collab knows it: measured, read in source, reported by a lane, or inferred.
- **Decided**: decisions the user would notice or might object to, each with its reason, stated as the outcome they will see.
- **Open**: each decision that needs the user, with Collab's recommendation and when another option wins.
- **Uncertainties**: what could still be wrong, what would show it, and what remains unverified.

When work follows the restatement, whether approved, proposed, or waiting on an answer, show it in a `workflow` fence in a section before the TL;DR, following the `workflow` skill.
Put open decisions that gate work in the card as `you` steps.
Work that runs only under one answer carries `if:` with that answer.
Skip the card only when nothing but the user's decisions is pending.
“Write for a reader returning cold” governs prose; in the card, lane names appear as pills by design, and each headline says what its step does.

Close the restatement with `## TL;DR`: the thesis and top recommendation in one or two sentences.
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
When drive mode hides the question tool, list the open decisions last in text, numbered, and suggest turning drive off to get the form.
