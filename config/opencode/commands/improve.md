---
name: improve
kind: command
description: Audit current or bounded local history agent workflow evidence and produce approval packets for durable prompt, permission, skill, script, or documentation improvements.
invocation: user
---

# improve

Treat `$ARGUMENTS` as mode and filter input for this invocation, not as permission to dump history or raw tool outputs.

Examples:

- `/improve` or `/improve current` audits the current visible session only.
- `/improve general --since 7d --limit 20` scans bounded recent local history for cross-cutting workflow improvements.
- `/improve permissions --repo dotfiles` looks for repeated permission prompts, denials, overbroad rules, or missing narrow permissions.
- `/improve agents --agent build.fast` looks for prompt conflicts, repeated manual instructions, handoff failures, or subagent misuse.
- `/improve skills` looks for recurring workflows that should become reusable skills.
- `/improve scripts` looks for repeated shell or procedure patterns that should become scripts or Go commands.

Use the skill tool to load `improve`.
