---
routes:
  build/owner: claude/claude-opus-5-5 high
  build/general: claude/claude-opus-5-5 high
  build/git: claude/claude-opus-5-5 high
  build/patch: claude/claude-sonnet-5-5 high
  build/scribe: openai/gpt-6.1-sol high
  scout/*: claude/claude-opus-5-5 high
  scout/context: claude/claude-sonnet-5-5 high
  scout/dirty: claude/claude-sonnet-5-5 high
  scout/session: openai/gpt-6-astra xhigh
  scout/web: opencode-go/grok-4.7 high
  review/*: openai/gpt-6.1-sol high
  review/critic: claude/claude-opus-5-5 high
  review/entropy: claude/claude-opus-5-5 max
  verify/*: openai/gpt-6.1-sol high
fallbacks:
  claude/claude-opus-5-5: opencode-go/grok-4.7
  claude/claude-sonnet-5-5: opencode-go/grok-4.7
---

# Routing

This file owns model routing for delegated work.
The frontmatter sets each agent's default model and effort, and the body carries the judgment for when to depart from them.
Treat everything here as defaults and preferences; state the reason when you depart from one.
Recommend changes to this file and confirm each decision with the user before editing it.
When drive mode hides `question`, leave routing changes for the user unless already approved.

## Routes

Frontmatter maps an agent name or `*` glob to `provider/model [variant]`, with an optional space-separated variant.
Explicit `subagent` model values use `provider/model#variant` instead.
An exact agent name beats a glob, and a longer glob beats a shorter one.
The `claude` provider is the Anthropic account pool; the lanes plugin resolves it to `anthropic` or `anthropic-personal` for each call.
Each `fallbacks` entry names the model to use when both Anthropic accounts are capped.

Omit `model` on `subagent` to take the route; this is the normal case.
To change reasoning effort, pass the model and its variant together, such as `claude/claude-opus-5-5#xhigh`.
An explicit model does not take the route's variant.
Use `claude/<model>#<variant>` to keep account selection, or omit `#<variant>` to use the model's default.

- **Enforced**: an explicit `model` beats the route, including its variant.
- **Enforced**: unusable frontmatter fails calls that omit `model` or use the `claude` pool; a concrete explicit provider bypasses this file.
- An agent with no route keeps native model selection; the lanes plugin adds no routing note for that case.

## Anthropic accounts

**Trend** (`anthropic`) and **Cogikyo** (`anthropic-personal`) both serve every repository.
The pool spends each weekly window before it resets.

- **Enforced**: the pool skips an account when its hourly `H`, all-model weekly `W`, or routed model's scoped window is at 100%; another model's cap does not block it.
- **Enforced**: a resumed child keeps its account while that account is open; otherwise the pool prefers fully known usage, then the earliest weekly reset.
- **Enforced**: cached usage older than ten minutes can still influence the pick, and the result notes its age.
- **Enforced**: when both accounts are capped, the pool uses the model's fallback without a variant and notes the switch in the result.
  - Without a fallback, it picks the account whose limiting caps reset first; it does not wait for reset.
- Pass `anthropic/…` or `anthropic-personal/…` only when the user names an account; that pins the account and skips the pick.
- Use Opus at `high`, `xhigh`, or `max`, and Sonnet at `high` or `xhigh`.
  - The Anthropic plugin filters low variants from the canonical provider; this is guidance, not a lanes validation rule.
- Authentication failures return a blocker rather than trigger quota overflow.
  - The user refreshes auth through the Claude CLI with the zsh helper `claude-auth trend` or `claude-auth cogikyo`.

## Which agent

Use the `subagent` agent descriptions and these guidelines to pick a child.
Only Collab dispatches children; every child is a leaf, including reusable lanes.

### Scouts

- Inspect directly when one pass answers the question.
- Give each scout one factual question, bounds, required evidence, and a stopping point.
- Start with `scout/context` only when ownership and relevant files are unknown.
- Use `scout/library` before writing a helper that likely exists in the stdlib or the repository.
- Use `scout/web` for the option space and prior art; it routes to Grok and calls `x` by default.

### Reviewers

- Pick lenses from the risk, not the file list; a small diff often needs one.
- Send frontend changes that add or change UI text through `review/copy` before commit.
- Run `review/entropy` on request, on package or app sweeps, or as a retro after a feature lands; on a small diff it produces rewrite noise.
- A lane that built the change is a poor judge of it; use a fresh reviewer for the final verdict.
- Synthesize findings in-session with `review`.

### Builders

Pick a builder by how much the brief settles.

- `build/owner` suits a large, open objective whose context the builder gathers itself.
- `build/general` suits a bounded outcome with clear constraints.
- `build/patch` suits settled mechanical edits to named files.
- `build/scribe` suits documentation, comments, banners, and agent or skill prompts.
  - Route new or rewritten prose there even inside a larger build; builders own the code.
- `build/git` suits one approved Git workflow too large for this context.
- Builders own formatting, lint, and other cheap checks in their scope.
- Builders add no comments; `build/scribe` owns comments.
- Name a lane when follow-ups are likely, and send repairs back to the lane that built the change.

### Verifiers

- Use a verifier when a claim decides the design or verdict.
- `verify/source` checks a claim against source, and `verify/web` checks it against current docs or live APIs.
- `verify/test` runs the commands its brief names.
- `verify/browser` covers layout, interactions, console, and network.

## When to deviate

- Run a hard `build/owner` or `build/general` at Opus `xhigh` when time is not a constraint.
- Send trivial edits to `build/patch` on Sonnet `high`, which is slightly faster than Opus.
  - Keep a `{scope}-patch` lane open for repeated patches in the same scope.
- Use Opus `max` when the user asks for it.
- Add Sol or Astra leaves eagerly beside Opus work; Collab picks between them.
  - Run a parallel OpenAI scout on the same brief when a second independent read could change the answer.
  - Send any claim the answer or design rests on to `verify/*`.
  - Prefer `verify/web` or an OpenAI `scout/web` over answering external facts from memory.
- Pass `openai/gpt-6-astra#high` or `openai/gpt-6.1-sol#high` when Collab picks one explicitly.
- Seat a fresh Opus reviewer beside Sol or Astra in each review council.
  - Pass `claude/claude-opus-5-5#high` for the Opus seat.
  - For repo-wide `review/entropy` sweeps, run the Opus `max` lane beside Astra at `high`.
- Send OpenAI findings that need filtering through Opus `review/critic`, and brief it with those findings.

## Models

Listed in rough order of overall preference.
Most models start at `high` for every role until evidence says otherwise.

### `claude-opus-5-5`

- Routes: `build/owner`, `build/general`, `build/git`, `review/critic`, `review/entropy`, and the `scout/*` default; all important building runs here.
- Reasoning: `high` by default, or `xhigh` when time is not a constraint.
  - Below `max`, it appears to reason less on its own when the task allows.
- Fallback: `opencode-go/grok-4.7` without effort when both Anthropic accounts are capped.

### `openai/gpt-6.1-sol`

- Routes: every `review/*` lens except `review/critic` and `review/entropy`, every `verify/*` leaf, and `build/scribe`.
- Reasoning: `high` by default; `xhigh` is available.
- Fallback: Opus when OpenAI is capped; route those steps to `claude/claude-opus-5-5#high` yourself.
- Fast and cheap with plenty of usage, so route review and verification here.
- Best `build/scribe`, and best at browser QA and computer use.
- Gathers wide context well and makes sound architecture calls.
- Strength looks close to Opus but is untested; expect an occasional dumb miss.

### `openai/gpt-6-astra`

- Routes: `scout/session` at `xhigh`; otherwise Collab adds it as a council seat or a parallel leaf.
- OpenAI positions it as its most capable model, and Sol as near-Astra at one fifth of the API price.
- Only model here with a published long-context score: 96.3% on MRCR v2 with eight needles at 512K–1M tokens.
- Reasoning: `high`, `xhigh`, and `max` are available.
- Fallback: Opus when OpenAI is capped, because Astra and Sol share one OpenAI window.
- Strength relative to Sol and Opus on this work is untested; record evidence as councils compare it.

### `claude-sonnet-5-5`

- Routes: `scout/context`, `scout/dirty`, and `build/patch`, all at `high`.
- Orchestrates web search and `x` calls well.
- Fallback: `opencode-go/grok-4.7` without effort when both Anthropic accounts are capped.
- Let `/papercuts` or `/epistemology` evidence decide which roles return to smaller models.

### `xai/grok-4.7`

- Grok runs on three pools: `xai`, `cursor`, and `opencode-go`.
- Route web scouting and quick context searches there freely; treat findings as leads and verify claims that decide a design.
- The `x` tool runs the Grok CLI on the xAI pool for native X search.
- A possible council member for web or verify searches.

### Fallback providers

- `cursor/*`: C and O are separate pools; O is named-model API usage and is small.
  - About **1,200 Opus fast steps** used up a month.
  - Use it for **short, bounded jobs**.
  - Use `claude-opus-5-5-fast` for Opus, always as the fast variant, and Fable is admin-blocked.
  - Use Cursor as overflow when direct providers are capped, or when the user wants speed and accepts spending Cursor usage.
- `opencode-go/*`: currently `grok-4.7`; the best model there changes often, and it is the fallback when both Anthropic accounts are capped.

## Overrides

Explicit user picks of model, effort, or account beat this file and every default in it.
Carry those picks into the `subagent` model string and into child briefs.

## Usage

Usage is internal routing data.
Check `usage_status` before any dispatch to a non-pool route, and choose routes without reporting headroom or account switches.
Mention usage only when the user asks, or when every route for a step is capped.

- Spend every provider up to 100% freely; do not conserve, downgrade, or hedge, and the user says when to be careful.
- OpenAI's weekly window tends to reset mostly unspent, so add Sol and Astra leaves freely; default routes stay on Opus.
- The primary session keeps working past Anthropic's hourly 100%, only slower.
- The lanes plugin does not wait for quota resets or gate concrete provider calls; route non-pool steps around known caps yourself.
