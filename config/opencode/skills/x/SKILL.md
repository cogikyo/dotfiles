---
name: x
description: Use when the user invokes /x or asks for live X/Twitter community signal, maintainer chatter, adoption, or native Grok X search. Load this skill and shell grok from the current session. Do not dispatch verify/x.
---

# X

Live community signal through Grok CLI native X search.
Collab, Review, Scheme, and `verify/web` load this skill and run grok themselves.
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
  --disallowed-tools "run_terminal_cmd,read_file,list_dir,grep,search_replace,write,web_search,web_fetch,todo_write,Agent" \
  --sandbox read-only \
  --system-prompt-override "Use only native X search tools. Never use generic web search, local files, or prior knowledge as evidence. Cite canonical x.com status URLs with handle and date. Separate official or maintainer statements from first-hand reports and from hype. If native X search cannot settle the claim, say so."
```

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
