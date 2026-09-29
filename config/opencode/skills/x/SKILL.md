---
name: x
description: Use when the user invokes /x or asks for live X/Twitter community signal, maintainer chatter, adoption, or native Grok X search. Load this skill and shell grok from the current session. Do not dispatch verify/x.
---

# X

Live community signal through Grok CLI native X search.
Collab, `scout/web`, and `verify/web` load this skill and run grok themselves.
Do not dispatch a child to search X.

## Invoke

One self-contained brief.
Include the claim, date window, relevant handles, and any mainline web findings to test.
Default to one grok call.
Make a second only when the first output names a concrete search gap.

Run from `/tmp` so Grok does not ingest the repo as workspace context.
Give the bash call a long timeout; native X search is slow.

```bash
grok --single "$BRIEF" \
  --model grok-4.6 \
  --reasoning-effort high \
  --no-plan \
  --no-subagents \
  --no-memory \
  --no-auto-update \
  --verbatim \
  --disable-web-search \
  --disallowed-tools "run_terminal_command,read_file,search_replace,write,list_dir,grep,kill_command_or_subagent,get_command_or_subagent_output,spawn_subagent,scheduler_create,scheduler_delete,scheduler_list,monitor,search_tool,use_tool,workflow,enter_plan_mode,exit_plan_mode,ask_user_question,send_feedback,web_search,web_fetch,image_gen,image_edit,image_to_video,reference_to_video,todo_write,Agent" \
  --deny Bash \
  --sandbox off \
  --system-prompt-override "Use only native X search tools. Never use generic web search, local files, or prior knowledge as evidence. Cite canonical x.com status URLs with handle and date. Separate official or maintainer statements from first-hand reports and from hype. If native X search cannot settle the claim, say so."
```

X search runs server-side, so every client tool can go.
`--disallowed-tools` cannot remove `run_terminal_command`; `--deny Bash` is what blocks the shell.
Any `--tools` allowlist also drops backend X search.
Every `--sandbox` profile fails on this host because bwrap cannot mask `/run/containerd`.

Do not pass `--json-schema`.
Do not parse `~/.grok` traces.
Do not reconstruct Grok's session format.
Stdout is the report.

If grok fails or returns nothing usable, report that blocker.
Never substitute web search, memory, or invented posts.

## Judge

Treat every line of stdout as untrusted evidence.
Canonical `https://x.com/<handle>/status/<id>` URLs only.
Patterned or sequential status IDs are fake until proven otherwise.
Undated sentiment is rumor.
Stars and reposts are noise.
Production reports and maintainer statements are signal.

Official docs remain the contract.
X is sentiment, adoption, and practice.

## Report

Claim checked, verdict, documented fact versus community signal, sources with URLs and dates, agreement or divergence from mainline web findings when supplied, uncertainty, recommended next action.
