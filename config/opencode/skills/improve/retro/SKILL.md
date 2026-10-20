---
name: retro
description: Use when the user invokes /retro or asks for an end-of-session retro. Collab runs it attended; it also runs non-interactively on one past session as an /improve leaf.
---

# Retro

## Authority

Collab owns the live conversation and any approved changes.
A `scout/session` leaf may run past mode for `/improve`.
Past mode is read-only: do not edit files or continue the session's work.

## Scope

Treat `$ARGUMENTS` as an optional focus or session ID.
Live mode defaults to the current session; read its ID from the shell environment variable `$OPENCODE_SESSION_ID`.
A supplied session ID or an `/improve` leaf brief selects past mode.
Keep all transcript searches inside the target session and its children.

Use what Collab remembers from the live conversation first.
Use the `sessions` tool for compacted early turns, lane transcripts, and tool failures.
Quote minimally and omit secrets.

## Gather

Load `papercuts` for its verdicts and `epistemology` for correction reading and owner selection.
Use their relevant methods within this skill's session scope and confirmation bar.

- Classify failed shell calls and subagent errors with the papercuts verdicts, preserving any supplied triage diagnosis.
- Read user corrections, stated preferences, redone work, and explanations Collab had to reconstruct.
- Look for missing or awkward tools, permission holes, and slow routes.

## Qualify

A candidate qualifies when at least one condition holds:

- The user explicitly corrected Collab or stated a preference.
- The same miss happened twice in the session.
- A failure cost real turns.
- Tool friction recurred.

One-off task detail does not qualify as reusable guidance.
In live mode, user confirmation can stand in for the cross-session recurrence that epistemology requires.
In past mode, mark every finding **unconfirmed** and include the question the user must answer to confirm it.

## Converse

In live mode, first show a short list of candidate issues with minimal evidence.
Ask one or two focused questions at a time about which corrections are general, what the user wants next time, and tool pain Collab missed; hold the rest.
Keep the exchange short and let the user steer or add issues.
Confirm the reusable lesson before proposing a change.
Past mode returns findings and open questions to the parent without this exchange.

## Propose

Pick the smallest owner from epistemology's owner list that prevents the repeat.
Also consider `opencode.json` permissions and plugins or tools when they own the friction.
Prefer editing or deleting existing guidance over adding more.
When the owner carries a worked example, propose removing it and tightening the rule it shows; keep notation samples such as diagrams.
This follows the trial rule in `docs`; note in the report whether the example-free version held.
Keep papercuts' Proper deny and Recovered protections; an agent miss alone does not justify wider permissions.

## Apply

Only live Collab may apply changes, after the user approves specific proposals.
Global-config edits land in `~/dotfiles`; when running from another project, say that this is a separate repository before approval.
Keep edits inside the approved targets and inspect the resulting changes.

## Report

Return a short finding list rather than a transcript dump.
Report as epistemology does, plus:

- Confirmation status, with a confirmation question for every past-mode finding.

Distinguish proposals from approved changes actually applied.
