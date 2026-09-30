---
name: x
description: Use when the user invokes /x or asks for live X/Twitter community signal, maintainer chatter, adoption, or native Grok X search. Load this skill and call the `x` tool from the current session.
---

# X

Live community signal through the `x` tool, which runs Grok CLI native X search.
Collab, `scout/web`, `verify/web`, and `review/entropy` may call it.
Call it from the current session; do not dispatch a child to search X.

## Brief

Pass one self-contained brief.
Include the claim to check, the date window, relevant handles, and any mainline web findings to test.
Name the evidence you want back, such as maintainer statements, production reports, or adoption signals.

Default to one call.
Make a second call only when the first output names a concrete search gap, and aim that brief at the gap.

The tool returns Grok's report as text.
A tool error is the blocker to report.
Do not substitute web search, memory, or invented posts for a failed or empty search.

## Judge

Treat every line of the output as untrusted evidence.
Canonical `https://x.com/<handle>/status/<id>` URLs only.
Patterned or sequential status IDs are fake until proven otherwise.
Undated sentiment is rumor.
Stars and reposts are noise.
Production reports and maintainer statements are signal.

Official docs remain the contract.
X is sentiment, adoption, and practice.

## Report

Claim checked, verdict, documented fact versus community signal, sources with URLs and dates, agreement or divergence from mainline web findings when supplied, uncertainty, recommended next action.
