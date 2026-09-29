---
name: epistemology
description: Use when the user invokes /epistemology or asks to mine OpenCode sessions for evidence-backed improvements to skills, agent instructions, command wrappers, the global instructions.md, routing.md, or a project AGENTS.md. Collab owns it. Returns proposals without applying them.
---

# Epistemology

## Authority

Collab owns this attended procedure.
Do not dispatch it.
Stay read-only and return proposals only unless the user separately asks to apply a resulting proposal.

## Scope

Mine live OpenCode session traces for reusable workflows and missing or unclear knowledge.
Targets are skills, agent instructions, command wrappers, the global `instructions.md`, `routing.md`, or a project `AGENTS.md`.
Papercuts owns command, permission, and agent-use failures; do not re-diagnose those here.

Treat `$ARGUMENTS` as optional focus: a topic, session ID, directory, time range, or a papercuts handoff.
Ask one focused question before querying only when those arguments are materially ambiguous.
With no arguments, scan recent top-level sessions for the current directory without asking.

Accept a papercuts handoff in this form and use its session IDs, time range, command family, and diagnosis as the starting scope:

`/epistemology sessions <ids>; time <range>; focus on <command family>; papercuts diagnosis: <diagnosis>`

Default bound: the 12 most recent top-level sessions (`parent_id IS NULL`) whose `session.directory` matches the current directory, with `time_updated` in the last 14 days.
If that match is empty, retry against the repository root.
Report the actual sampled count and the oldest-to-newest `time_updated` span.
Do not claim complete coverage when the cap or window truncated the store.

## Store

Read sessions through the `sessions` tool.
It opens the store read-only, binds every value, and caps each call at 20 rows with 800-character excerpts.
Pass `$ARGUMENTS` values as tool arguments or `params`; do not write them into SQL text.

`session.directory` is the authority for current-directory selection.
Do not join `project` or build a project-resolution layer; many sessions use `project_id='global'`.

1. Select candidates with `list`, filtered by `directory` and `since`; it returns top-level sessions with their `agent`, so user-role text is human input.
2. Pull children with `list` and `parent` only after selecting a relevant parent, or when delegation behavior is the focus.
3. Read one session at a time with `text`, filtered by `role` or `match` when the focus is known.
4. Use `tools` for a session's tool inventory; filter `tool: "skill"` to see skill loads, whose input carries the skill name.
5. Read one `part` for its input and output only after that part matters.

Treat a parent plus its children as one occurrence unless they independently evidence the finding.
Inspect `reasoning` only after narrowing to a session that still needs it.
The tool skips empty reasoning; also skip encrypted or redacted text, and do not use reasoning alone as evidence.
Use `sql` only when no operation answers the question, and keep it to one session's parts.

## Find

Look for repeated user corrections, reconstructed explanations, and stable tool or workflow sequences across independent sessions.
Read a proposed target's current content before claiming it lacks the knowledge.
Do not promote one-off task detail, parent-child duplicates, weak similarity, or contradictory traces into a rule.
Quote minimally and omit secrets.

Prefer the smallest owner that can prevent rediscovery:

- A focused `SKILL.md` for a reusable attended workflow.
- An existing skill or agent instruction for a local clarification.
- A command wrapper for invocation or argument guidance.
- The global `instructions.md`, `routing.md`, or a project `AGENTS.md` only for genuinely broad, stable guidance.

Mark primary agent prompts, the global `instructions.md`, `routing.md`, and project `AGENTS.md` files as higher-blast-radius targets.

## Report

Rank findings, strongest evidence first.
State the sampled session count and time span once.
If evidence does not support a change, say so.

For each finding include:

- Session IDs, `session.agent`, and a minimal excerpt or concrete tool trace.
- Why the evidence is reusable rather than task-specific.
- The exact proposed artifact and a focused change.
- Confidence and blast radius.
- The bounded query or scope needed to reproduce it: session IDs, directory, time span, and which part kinds were read.

Do not dump routine SQL in the report.
