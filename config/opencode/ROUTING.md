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
Recommend changes to this file and confirm each decision with the user before editing it unless already approved.

## Routes

Frontmatter routes use `provider/model [variant]`, with a space before the optional variant.
Explicit `subagent` model values use `provider/model#variant`.
An exact agent name beats a glob, and a longer glob beats a shorter one.
The `claude` provider is the Anthropic account pool; the lanes plugin resolves it to `anthropic` or `anthropic-personal` for each call.
Each `fallbacks` entry names the model to use when both Anthropic accounts are capped.

Omit `model` on `subagent` to take the route; this is the normal case.
To change reasoning effort, pass the model and its variant together, such as `claude/claude-opus-5-5#xhigh`.
An explicit model does not take the route's variant.
Use `claude/<model>#<variant>` to keep account selection, or omit `#<variant>` to use the model's default.

## Anthropic accounts

**Trend** (`anthropic`) and **Cogikyo** (`anthropic-personal`) both serve every repository.
The pool spends each weekly window before it resets.

- **Enforced**: when both accounts are capped, the pool uses the model's fallback without a variant and notes the switch in the result.
- Pass `anthropic/…` or `anthropic-personal/…` only when the user names an account; that pins the account and skips the pick.
- Use Opus at `high`, `xhigh`, or `max`, and Sonnet at `high` or `xhigh`.
- Authentication failures return a blocker rather than trigger quota overflow.

## Which agent

Use the `subagent` agent descriptions and these guidelines to pick a child.

### Scouts

- Start with `scout/context` only when ownership and relevant files are unknown.
- Use `scout/library` before writing a helper that likely exists in the stdlib or the repository.
- Use `scout/web` for the option space and prior art; it routes to Grok and calls `x` by default.

### Reviewers

- Pick lenses from the risk, not the file list; a small diff often needs one.
- Send frontend changes that add or change UI text through `review/copy` before commit.
- Run `review/entropy` on request, on package or app sweeps, or as a retro after a feature lands; on a small diff it produces rewrite noise.

### Builders

Pick a builder by how much the brief settles.

- Route new or rewritten prose to `build/scribe` even inside a larger build; builders own the code.
- `build/git` suits one approved Git workflow too large for this context.
- Builders add no comments; `build/scribe` owns comments.

### Verifiers

- Use a verifier when a claim decides the design or verdict.

## When to deviate

- Run a hard `build/owner` or `build/general` at Opus `xhigh` when time is not a constraint.
- Send trivial edits to `build/patch` on Sonnet `high`, which is slightly faster than Opus.
  - Keep a `{scope}-patch` lane open for repeated patches in the same scope.
- Use Opus `max` when the user asks for it.
- Add Sol or Astra leaves eagerly beside Opus work; Collab picks between them.
  - Run a parallel OpenAI scout on the same brief when a second independent read could change the answer.
  - Use `verify/*` to settle an open claim, to double-check a claim the answer or design rests on, or as a council seat that guards against shared assumptions.
  - Prefer `verify/web` or an OpenAI `scout/web` over answering external facts from memory.
- Pass `openai/gpt-6-astra#high` or `openai/gpt-6.1-sol#high` when Collab picks one explicitly.
- Seat a **fresh Opus reviewer** beside Sol or Astra in every review council, and convene one for harness changes, production-impact findings, and design verdicts.
  - Pass `claude/claude-opus-5-5#high` for the Opus seat.
  - For repo-wide `review/entropy` sweeps, run the Opus `max` lane beside Astra at `high`.
- Send OpenAI findings that need filtering through Opus `review/critic`, and brief it with those findings.

## Models

Listed in rough order of overall preference.
Most models start at `high` for every role until evidence says otherwise.

### `claude-opus-5-5`

- Reasoning: `high` by default, or `xhigh` when time is not a constraint.
  - Below `max`, it appears to reason less on its own when the task allows.

### `openai/gpt-6.1-sol`

- Reasoning: `high` by default; `xhigh` is available.
- Fallback: Opus when OpenAI is capped; route those steps to `claude/claude-opus-5-5#high` yourself.
- Fast and cheap with plenty of usage, so use Sol alone for small checks and keep the Sol `review/*` and `verify/*` routes for routine work.
- Best `build/scribe`, and best at browser QA and computer use.
- Gathers wide context well and makes sound architecture calls.
- Strength looks close to Opus but is untested; expect an occasional dumb miss.

### `openai/gpt-6-astra`

- Use only as a **council seat** or **parallel leaf**, never as the primary.
- Reasoning: `high`, `xhigh`, and `max` are available.
- Fallback: Opus when OpenAI is capped, because Astra and Sol share one OpenAI window.
- Strength relative to Sol and Opus on this work is untested; record evidence as councils compare it.

### `claude-sonnet-5-5`

- Orchestrates web search and `x` calls well.
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

## Usage

Usage is internal routing data.
Check `usage_status` before any dispatch to a non-pool route, route those steps around known caps yourself, and choose routes without reporting headroom or account switches.
Mention usage only when the user asks, or when every route for a step is capped.

- Spend every provider up to 100% freely; do not conserve, downgrade, or hedge, and the user says when to be careful.
- OpenAI's weekly window tends to reset mostly unspent, so add Sol and Astra leaves freely; default routes stay on Opus.
- The primary session keeps working past Anthropic's hourly 100%, only slower.
