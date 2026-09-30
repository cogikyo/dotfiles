---
routes:
  build/owner: claude/claude-opus-5-5 high
  build/general: claude/claude-opus-5-5 high
  build/git: claude/claude-opus-5-5 high
  build/patch: claude/claude-sonnet-5-5 high
  build/scribe: openai/gpt-6.1-sol high
  scout/*: claude/claude-sonnet-5-5 xhigh
  review/*: openai/gpt-6.1-sol high
  review/critic: claude/claude-opus-5-5 high
  review/entropy: claude/claude-opus-5-5 max
  verify/*: openai/gpt-6.1-sol high
fallbacks:
  claude/claude-opus-5-5: cursor/claude-opus-5-5-fast
  claude/claude-sonnet-5-5: cursor/claude-opus-5-5-fast
---

# Routing

This file owns model routing for delegated work.
The frontmatter sets each agent's default model and effort, and the body carries the judgment for when to depart from them.
Treat everything here as defaults and preferences; state the reason when you depart from one.

## Routes

Each route maps an agent name or `*` glob to `provider/model-id effort`, and the effort is optional.
An exact agent name beats a glob, and a longer glob beats a shorter one.
The `claude` provider is the Anthropic account pool; the task plugin resolves it to `anthropic` or `anthropic-personal` for each call.
Each `fallbacks` entry names the model to use when both Anthropic accounts are capped.

Omit `model` and `effort` on `task` to take the route; this is the normal case.
Pass `effort` alone to change effort on the routed model.
Pass `model` to change the model, and pass `effort` with it, because an explicit model does not take the route's effort.
Use `claude/<model-id>` as an explicit model to keep the account pick.

- **Enforced**: an explicit `model` or `effort` on the call beats the route.
- **Enforced**: invalid frontmatter fails every `task` call that omits `model`, and the error shows above this file in your system prompt.
- **Enforced**: an agent with no route uses its pinned model, else your current model with a note in the task result.

## Anthropic accounts

**Trend** (`anthropic`) and **Cogikyo** (`anthropic-personal`) both serve every repository.
The pool spends each weekly window before it resets.

- **Enforced**: the task plugin skips an account when its hourly window, its all-model weekly window, or the routed model's weekly window is at 100%; another model's capped window does not block it.
- **Enforced**: among the rest, it prefers accounts with fully known usage, then the one whose weekly window resets first.
- **Enforced**: when both accounts are blocked, it uses the model's fallback without effort and notes the switch in the task result.
- Pass `anthropic/…` or `anthropic-personal/…` only when the user names an account; that pins the account and skips the pick.
- **Enforced**: Opus runs only at `high`, `xhigh`, or `max`, and Sonnet only at `high` or `xhigh`; `opencode.json` disables their other variants.
- Authentication failures return a blocker rather than trigger quota overflow.
  - The user refreshes auth through the Claude CLI with the zsh helper `claude-auth trend` or `claude-auth cogikyo`.

## Which agent

The task tool lists each agent's description; these lines cover when to pick one.

### Scouts

- Inspect directly when one pass answers the question.
- Give each scout one factual question, bounds, required evidence, and a stopping point.
- Start with `scout/context` only when ownership and relevant files are unknown.
- Use `scout/library` before writing a helper that likely exists in the stdlib or the repository.
- Use `scout/web` for the option space and prior art; it orchestrates web search and `x` calls.

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
- Run architecture reviews as a council: Sol `review/architect` beside Opus `review/architect`.
- For repo-wide `review/entropy` sweeps, pair the Opus `max` lane with Sol at `xhigh` as a council.
- Send Sol findings that need filtering through Opus `review/critic`, and brief it with those findings.
- Run a `build/scribe` pass on touched docs and comments before most commits.

## Models

Listed in rough order of overall preference.
Most models start at `high` for every role until evidence says otherwise.

### `claude-opus-5-5`

- Routes: `build/owner`, `build/general`, `build/git`, `review/critic`, and `review/entropy`; all important building runs here.
- Reasoning: `high` by default, or `xhigh` when time is not a constraint.
  - Below `max`, it appears to reason less on its own when the task allows.
- Fallback: `cursor/claude-opus-5-5-fast` without effort when both Anthropic accounts are capped.

### `openai/gpt-6.1-sol`

- Routes: every `review/*` lens except `review/critic` and `review/entropy`, every `verify/*` leaf, and `build/scribe`.
- Reasoning: `high` by default; `xhigh` is available.
- Fallback: Opus when OpenAI is capped; route those steps to `claude/claude-opus-5-5` yourself.
- Fast and cheap with plenty of usage, so route review and verification here.
- Best `build/scribe`, and best at browser QA and computer use.
- Gathers wide context well and makes sound architecture calls.
- Strength looks close to Opus but is untested; expect an occasional dumb miss.

### `claude-sonnet-5-5`

- Routes: `scout/*` at `xhigh` and `build/patch` at `high`; also suits other high-token filtering, summarizing, and high-level scoping.
- Best scout available; it orchestrates web search and `x` calls well, and realtime user insight from `x` is valuable.
- Fallback: `cursor/claude-opus-5-5-fast` without effort, because Cursor has no working Sonnet fast variant.
- It uses more tokens, so its context can fill before the compaction warning; this is fine.

### `xai/grok-4.6`

- Not worth using as a task model; the `x` tool still runs the Grok CLI for native X search.
- A possible council member for web or verify searches.

### Fallback providers

- `cursor/*`: C and O are separate pools; only `claude-opus-5-5-fast` is routed, always as the fast variant, and Fable is admin-blocked.
  - Use Cursor as overflow when direct providers are capped, or when the user wants speed and accepts spending Cursor usage.
- `opencode-go/*`: `glm-5.3` at `high`; use it to test new open-source models such as DeepSeek V5 when they release.

## Overrides

Explicit user picks of model, effort, or account beat this file and every default in it.
Carry those picks into the `task` call and into child briefs.

## Usage

Usage is internal routing data.
Check `usage_status` before any dispatch to a non-pool route, and choose routes without reporting headroom or account switches.
Mention usage only when the user asks, or when every route for a step is capped.

- Spend every provider up to 100% freely; do not conserve, downgrade, or hedge, and the user says when to be careful.
- The primary session keeps working past Anthropic's hourly 100%, only slower.
- **Enforced**: the task plugin waits for reset when a window that limits the child's model is at 100%, so route non-pool steps around capped providers yourself.
  - On an Anthropic account, those are the hourly window, the all-model weekly window, and that model's weekly window; elsewhere, every window counts.
