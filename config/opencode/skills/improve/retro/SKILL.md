---
name: retro
description: Use when the user invokes /retro or asks for an end-of-session retro. Collab runs it attended; it also runs non-interactively on one past session as an /improve leaf.
---

# Retro

## Authority

Collab owns the live conversation and any approved changes.
A `scout/session` leaf may run past mode for `/improve`.
Past mode is read-only: do not ask the user questions, edit files, or continue the session's work.
Do not mutate Git in either mode.

## Scope

Treat `$ARGUMENTS` as an optional focus or session ID.
Live mode defaults to the current session; read its ID from the shell environment variable `$OPENCODE_SESSION_ID`.
A supplied session ID or an `/improve` leaf brief selects past mode.
Keep all transcript searches inside the target session and its children.

Use what Collab remembers from the live conversation first.
Use the `sessions` tool for compacted early turns, lane transcripts, and tool failures.
Pull children with `list` and `parent`.
Page bounded results when needed; name missing or truncated evidence.
Read `text` for relevant turns, `tools` for failures, and `part` for a specific message or tool trace.
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
Treat the target and its children as one occurrence.

## Converse

In live mode, first show a short list of candidate issues with minimal evidence.
Batch a few focused questions about which corrections are general, what the user wants next time, and tool pain Collab missed.
Keep the exchange short and let the user steer or add issues.
Confirm the reusable lesson before proposing a change.
Past mode returns findings and open questions to the parent without this exchange.

## Propose

Pick the smallest owner from epistemology's owner list that prevents the repeat.
Also consider `opencode.json` permissions and plugins or tools when they own the friction.
Read each target's current content before claiming a gap.
Prefer editing or deleting existing guidance over adding more.
Mark primary agent prompts, global `AGENTS.md`, `ROUTING.md`, and project `AGENTS.md` as high blast radius.
Keep papercuts' Proper deny and Recovered protections; an agent miss alone does not justify wider permissions.
“Nothing worth changing” is a valid result.

## Apply

Only live Collab may apply changes, after the user approves specific proposals.
Global-config edits land in `~/dotfiles`; when running from another project, say that this is a separate repository before approval.
Keep edits inside the approved targets and inspect the resulting changes.

## Report

Return a short finding list rather than a transcript dump.
For each finding include:

- Evidence: session and message IDs or a tool trace, with a minimal excerpt.
- Target file and the exact proposed change.
- Confidence and blast radius.
- Confirmation status, with a confirmation question for every past-mode finding.

Distinguish proposals from approved changes actually applied.
