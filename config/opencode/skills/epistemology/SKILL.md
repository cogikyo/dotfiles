---
name: epistemology
description: Use when the user invokes /epistemology or asks to mine OpenCode sessions for evidence-backed improvements to skills, agent instructions, command wrappers, or AGENTS.md. Collab owns it. Returns proposals without applying them.
---

# Epistemology

## Authority

Collab owns this attended procedure.
Do not dispatch it.
Stay read-only and return proposals only unless the user separately asks to apply a resulting proposal.

## Scope

Mine live OpenCode session traces for reusable workflows and missing or unclear knowledge.
Targets are skills, agent instructions, command wrappers, or `AGENTS.md`.
Papercuts owns command, permission, and agent-use failures; do not re-diagnose those here.

Treat `$ARGUMENTS` as optional focus: a topic, session ID, directory, time range, or a papercuts handoff.
Ask one focused question before querying only when those arguments are materially ambiguous.
With no arguments, scan recent top-level sessions for the current directory without asking.

Accept a papercuts handoff in this form and use its session IDs, time range, command family, and diagnosis as the starting scope:

`/epistemology sessions <ids>; time <range>; focus on <command family>; papercuts diagnosis: <diagnosis>`

Default bound: the 12 most recent top-level sessions (`parent_id IS NULL`) whose `session.directory` matches the current directory, with `time_updated` in the last 14 days.
If that match is empty, retry against the repository root.
Report the actual sampled count and the oldest-to-newest `time_updated` span.
Never claim complete coverage when the cap or window truncated the store.

## Store

Read `/home/cullyn/.local/share/opencode/opencode.db` through SQLite's live-WAL-safe URI:

```bash
sqlite3 'file:/home/cullyn/.local/share/opencode/opencode.db?mode=ro' "PRAGMA query_only=ON; PRAGMA busy_timeout=200; <sql>"
```

Generate only `SELECT` and those two `PRAGMA`s.
Run one short invocation per query; do not hold an idle interactive read transaction.
`message` and `part` are the transcript.
`session_message` and `event` are not the current path.

Never place raw `$ARGUMENTS` or other untrusted text in SQL.
Quote each generated value as a SQL string literal: wrap in single quotes and double every embedded `'`.

## Query

`session.directory` is the authority for current-directory selection.
`project.worktree` is optional context only; many sessions use `project_id='global'` and have no project row for their real directory.
A scan of the session table is cheap.
Do not join `project` or build a project-resolution layer.

Default to top-level rows so user-role text is human input.
Pull child sessions only after selecting a relevant parent, or when delegation behavior is the focus.
Treat a parent plus its children as one occurrence unless they independently evidence the finding.
Include `session.agent` so a finding can name the owning instruction file.

Never query many sessions' parts with one `IN (...)` and a flat `LIMIT`.
Query one session at a time.
Keep text and output excerpts to 800 characters and keep `LIMIT` at 20 or below.
Retrieve tool outputs only after a specific part matters.

List candidate sessions:

```sql
SELECT id, directory, agent, title, time_updated
FROM session
WHERE parent_id IS NULL
  AND directory = '<directory>'
  AND time_updated >= unixepoch('now', '-14 days') * 1000
ORDER BY time_updated DESC
LIMIT 12;
```

After a parent is selected, its children:

```sql
SELECT id, agent, title, time_updated
FROM session
WHERE parent_id = '<parent-id>'
ORDER BY time_updated DESC
LIMIT 20;
```

Text excerpts for one session:

```sql
SELECT p.id,
       json_extract(m.data, '$.role') AS role,
       substr(json_extract(p.data, '$.text'), 1, 800) AS excerpt
FROM part AS p
JOIN message AS m ON m.id = p.message_id
WHERE p.session_id = '<session-id>'
  AND json_extract(p.data, '$.type') = 'text'
ORDER BY p.time_created, p.id
LIMIT 20;
```

Tool inventory without outputs:

```sql
SELECT id,
       json_extract(data, '$.tool') AS tool,
       json_extract(data, '$.state.status') AS status
FROM part
WHERE session_id = '<session-id>'
  AND json_extract(data, '$.type') = 'tool'
ORDER BY time_created, id
LIMIT 20;
```

One tool part after it matters:

```sql
SELECT json_extract(data, '$.state.input') AS input,
       substr(json_extract(data, '$.state.output'), 1, 800) AS output_excerpt
FROM part
WHERE id = '<part-id>'
  AND session_id = '<session-id>'
LIMIT 1;
```

Inspect reasoning only after narrowing to a session that still needs it.
Skip a reasoning part when its text is missing, encrypted, or redacted.
Never use reasoning alone as evidence.

For skill loads, `$.tool` is `skill` and the name is `$.state.input.name`.

## Find

Look for repeated user corrections, reconstructed explanations, and stable tool or workflow sequences across independent sessions.
Read a proposed target's current content before claiming it lacks the knowledge.
Do not promote one-off task detail, parent-child duplicates, weak similarity, or contradictory traces into a rule.
Quote minimally and omit secrets.

Prefer the smallest owner that can prevent rediscovery:

- A focused `SKILL.md` for a reusable attended workflow.
- An existing skill or agent instruction for a local clarification.
- A command wrapper for invocation or argument guidance.
- `AGENTS.md` only for genuinely broad, stable guidance.

Mark primary agent prompts and `AGENTS.md` as higher-blast-radius targets.

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
