---
name: improve
description: /improve current/general/permissions/agents/skills/scripts workflow audit. Use ONLY when the human explicitly invokes /improve or asks to improve/codify agent workflows from session or local history evidence.
---

# improve

Audit agent-workflow evidence for durable improvements.
Use ONLY when the human explicitly invokes `/improve` or asks to improve or codify agent workflows from session or local history evidence.
Do not run for routine orchestration, ordinary code review findings, or speculative prompt cleanup.

## Modes and arguments

Treat command arguments as mode and filter intent, not as permission to dump history.
Default to approval packets, not direct edits.

- `/improve` or `/improve current`: audit only the current visible session.
- `/improve general`: run a bounded recent local history scan for cross-cutting workflow improvements.
- `/improve permissions`: inspect permission prompts, denials, overbroad rules, and missing narrow allowances.
- `/improve agents`: inspect agent, manager, and Drive prompt conflicts, repeated manual instructions, handoff failures, and subagent misuse.
- `/improve skills`: inspect recurring workflows that should become reusable skills.
- `/improve scripts`: inspect repeated manual shell or procedure patterns that should become scripts or Go commands.

Optional selectors such as `--since`, `--limit`, `--repo`, and `--agent` are filter intent when available.
If a selector cannot be applied exactly, state the nearest bounded interpretation before scanning.

## Current-session workflow

Use current-session mode for `/improve` and `/improve current`.
If running in-session, use the transcript, tool calls, child reports, manager choices, and visible files available to you.
State visibility limits instead of pretending you saw a full transcript.

Audit current session evidence for:

- User corrections, reversals, approvals, or complaints.
- Drive or manager choices that reduced or increased error correction.
- Subagent calls that were redundant, wasted, missing, or valuable.
- Missing tools, missing permissions, or overbroad permissions.
- Unclear prompts, stale instructions, contradictory docs, or missing report fields.
- Verification gaps, blocked commands, or weak falsification strategy.
- Child-agent disagreements, stale assumptions, or handoff failures.
- Good patterns worth codifying because they predictably reduced future error.

## Local history scan workflow

Use history-scan workflow for `general`, `permissions`, `agents`, `skills`, and `scripts` modes.
Prefer a fresh Drive-style session when available, especially after a long run or when the current context is noisy.
Keep scans bounded by explicit selectors or conservative defaults such as recent sessions, a specific repo, or a small count limit.

Prefer metadata and structure before raw transcript reading:

- Inspect session metadata, names, timestamps, repos, agents, and diff summaries first.
- Inspect `~/.local/share/opencode/storage/session_diff/*.json` before raw transcripts when it answers the question.
- Use readonly SQLite access only when inspecting `~/.local/share/opencode/opencode.db`, such as `sqlite3 'file:/home/cullyn/.local/share/opencode/opencode.db?mode=ro'`.
- Never mutate, vacuum, migrate, checkpoint, or repair the OpenCode database.
- Exclude secrets and auth material, especially `~/.local/share/opencode/auth.json`.
- Avoid bulk-reading `tool-output/` by default because raw tool outputs can be huge or sensitive.
- Read raw transcripts or tool outputs only when metadata cannot support or falsify a specific candidate, and then read the smallest relevant slice.

Summarize or redact evidence.
Do not dump raw transcripts, raw tool outputs, secrets, tokens, auth blobs, or unrelated user content.
Report scan limits every time: paths inspected, session count or time window, filters applied, and sources intentionally left unscanned.

Mode focus:

- `general`: cross-cutting workflow friction, repeated corrections, recurring verification gaps, and durable wins.
- `permissions`: permission prompts, denials, repeated asks, risky broad allows, and missing narrow command patterns, or workarounds due to lack of permisisons.
- `agents`: prompt conflicts, repeated manual instruction overrides, manager or Drive routing errors, handoff failures, and subagent misuse.
- `skills`: repeated workflows where a reusable skill would reduce future manual prompting.
- `scripts`: repeated shell procedures or command sequences that should become a script or Go command.

## Evidence discipline

Separate evidence from conjecture.
Quote or summarize only the minimal evidence needed to support a finding.
Do not dump transcripts, raw child logs, or long tool outputs.
Treat a single incident as conjecture unless it exposes a durable workflow gap likely to repeat.
Avoid overfitting to one session; prefer repeated signals or one high-signal failure mode with a clear recurrence path.

## Default output

Produce approval packets by default, not direct edits.
Each packet must include:

```markdown
Finding:
Evidence:
Root-cause conjecture:
Proposed change:
Source-of-truth targets:
Risk/overfit concern:
Verification:
Recommendation:
```

Keep packets small, deduplicated, and ordered by expected future error reduction.
Say when no durable improvement is justified by the available evidence.

## Editing after approval

If the human approves an exact edit scope, proceed through the normal Build/edit workflow.
Read the relevant `AGENTS.md`, target files, and source-of-truth docs first.
Apply the smallest approved change and run targeted verification.
Report changed files, verification, residual risk, and restart needs for opencode config edits.

Do not broaden destructive filesystem operations, secret reads, force git operations, pushes, package installs, network writes, production-impacting commands, or Docker destructive commands.
Do not weaken security or privacy guardrails without explicit human approval for that exact change.
