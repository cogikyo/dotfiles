---
routes:
  build/owner: claude/claude-opus-5-5 high
  build/general: claude/claude-opus-5-5 high
  build/git: claude/claude-opus-5-5 high
  build/patch: claude/claude-opus-5-5 medium
  build/scribe: openai/gpt-6.1-sol high
  scout/*: claude/claude-opus-5-5 high
  scout/context: cursor/grok-4.7
  scout/dirty: cursor/grok-4.7
  scout/web: cursor/grok-4.7
  review/*: openai/gpt-6.1-sol high
  review/entropy: claude/claude-opus-5-5 xhigh
  verify/*: openai/gpt-6.1-sol high
fallbacks:
  claude/claude-opus-5-5: cursor/claude-opus-5-5
  claude/claude-sonnet-5-5: cursor/claude-sonnet-5-5
  cursor/grok-4.7: opencode-go/grok-4.7
  cursor/claude-opus-5-5: opencode-go/grok-4.7
  cursor/claude-sonnet-5-5: opencode-go/grok-4.7
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
The `claude` provider is the Anthropic account pool; the lanes plugin resolves it to `anthropic`, `anthropic-personal`, or `anthropic-cullyn` for each call.
Each `fallbacks` entry names the next model to try when the source model's pool is capped; entries form a chain.

Main sessions belong to the user and are not routed here.
`ctrl+m` and main-session failover pick only Cogikyo or Cullyn; default and manual picks otherwise keep their account.

Omit `model` on `subagent` to take the route; this is the normal case.
To change reasoning effort, pass the model and its variant together, such as `claude/claude-opus-5-5#xhigh`.
An explicit model does not take the route's variant.
Use `claude/<model>#<variant>` to keep account selection, or omit `#<variant>` to use the model's default.

## Effort ladder

Pick effort for each workflow step from the task; route variants are defaults.

- **`medium`**: mechanical or fully specified work, such as patches, lookups, first-pass maps, data pulls, and builds whose brief settles every choice.
- **`high`**: bounded judgment, such as builders with a clear brief, focused scouts, and routine reviews.
- **`xhigh`**: hard or ambiguous work, such as tricky debugging, cross-cutting builds, and the final review of a risky change.
- **`max`**: only when the user asks for it, or for a sweep the user requested.

Picks of `medium` or `high` along this ladder are within guidance.
Picks of `xhigh` or `max` count as departures and carry a `why:` on the workflow card, even when the route defaults to that effort.
Collab runs at `high`; the user switches it to `xhigh` in the TUI picker when a session needs it.

## Anthropic accounts

**Trend** (`anthropic`), **Cogikyo** (`anthropic-personal`), and **Cullyn** (`anthropic-cullyn`) all serve every repository.
Lane picks and failover share this order among uncapped accounts:

1. Keep a lane's follow-ups on its current account.
2. Prefer accounts with fresh usage data over unknown ones.
3. Prefer Trend.
4. Then prefer the soonest weekly reset.

Trend leads because its 5-hour window limits how fast it can spend weekly usage, so it cannot be drained in a burst later.
Cogikyo and Cullyn have no 5-hour window, so soonest-reset-first uses their weekly capacity before it expires; `ctrl+m` and main-session failover exclude Trend and use that order.

- **Enforced**: when every account is capped for Opus or Sonnet, lanes try the same model on Cursor's O pool, then `opencode-go/grok-4.7` if O is capped, without a variant and with a note for each switch.
- When an account caps mid-request, the Anthropic plugin resends on the next open account; later requests in that session stay there until the cap resets.
- Pass `anthropic/…`, `anthropic-personal/…`, or `anthropic-cullyn/…` only when the user names an account; that pins the account and skips the pick.
- Opus and Sonnet both take `medium` through `max`; pick by the effort ladder.
- Authentication failures return a blocker rather than trigger quota overflow.

## Which agent

Use the `subagent` agent descriptions and these guidelines to pick a child.

### Scouts

- Use `scout/context` and `scout/dirty` on Cursor Grok for first passes that give Collab a rough overview of whether focused scouts or verifiers are needed and which ones.
- Start with `scout/context` only when ownership and relevant files are unknown; skip the first pass when the brief names the files or the question is already bounded.
- Send focused questions to `scout/code` for local code behavior, readers and consumers, end-to-end traces, and live or legacy paths.
- Use `scout/library` before writing a helper that likely exists in the stdlib or the repository.
- Keep reads that need the Docker socket or `.pier/secrets.yaml`, such as `docker save` or `pier profile show`, in Collab's shell; cage blocks both for read-only children.
- Use `scout/web` for the option space and prior art; it routes to Cursor Grok and calls `x` by default, and other scouts and source/web verifiers can also call `x`.
- Run web research as a council by default, because web sources are noisy and seat diversity earns its cost.
  - Seat `scout/web` on its Cursor Grok route beside a DeepSeek seat, `opencode-go/deepseek-v4.1-flash#high`.
  - Add an Opus seat, `claude/claude-opus-5-5#high`, when the answer decides a design.
  - Check `usage_status` before seating the council and drop a known-capped seat before lanes can substitute a fallback; skip `x` when its separate xAI pool is capped.

### Reviewers

- Sol carries review volume; keep routine reviews on its routes.
- After implementation, pair **`review/debug`** for correctness with **`review/simplify`** for idiom, dead code, and cleanliness.
  - Keep `review/debug` alone as the default for small fixes.
- Add `review/security`, `review/profile`, `review/architect`, and `review/copy` as the change's risks warrant.
- Keep `review/security` and other adversarial briefs on OpenAI, because Anthropic's cyber filter blocks Opus on them.
  - Adversarial means bypass hunts, exploit paths, or attacks on keys, sandboxes, or guards.
  - Opus can still take a routine security pass, such as secrets or auth in an app diff.
- Send frontend changes that add or change UI text through `review/copy` before commit.
- Run `review/entropy` on request, on package or app sweeps, or as a retro after a feature lands; on a small diff it produces rewrite noise.

### Builders

Pick a builder by how much the brief settles.

- Route new or rewritten prose to `build/scribe` even inside a larger build; builders own the code.
  - Pass `claude/claude-opus-5-5#high` for READMEs and large docs where every word matters; Sol stays the route for quick patches, edits, and comment passes.
- `build/git` suits one approved Git workflow too large for this context.
- Builders add no comments; `build/scribe` owns comments.

### Verifiers

- Use a verifier when a claim decides the design or verdict.

## When to deviate

- Run `build/owner` or `build/general` at Opus `xhigh` only for hard or ambiguous steps, with a `why:`.
- Send trivial edits to `build/patch` on Opus `medium`.
  - Keep a `{scope}-patch` lane open for repeated patches in the same scope.
- Use Opus `max` when the user asks for it.
- Add Sol or Astra leaves eagerly beside Opus work; Collab picks between them.
  - Run a parallel OpenAI scout on the same brief when a second independent read could change the answer.
  - Use `verify/*` to settle an open claim, to double-check a claim the answer or design rests on, or as a council seat that guards against shared assumptions.
  - Prefer `verify/web` or an OpenAI `scout/web` over answering external facts from memory.
- Pass `openai/gpt-6-astra#high` or `openai/gpt-6.1-sol#high` when Collab picks one explicitly.
- Convene a council for harness changes, production-impact findings, and design verdicts, with a **fresh Opus reviewer** beside Sol or Astra.
  - Reserve the Opus seat for those cases; other councils run Sol and Astra seats.
  - Give Opus a different lens when risks span lenses, but not an adversarial one; keep seats on the same lens only for high-stakes changes where model diversity is the point.
  - Pass `claude/claude-opus-5-5#high` for the Opus seat.
  - For repo-wide `review/entropy` sweeps, run the Opus `xhigh` lane beside Astra at `high`.
- `review/critic` routes to Sol; pass `claude/claude-opus-5-5#high` only when it filters OpenAI findings, and brief it with those findings.

## Models

Listed in rough order of overall preference.
Use the effort ladder for each step.

### `claude-opus-5-5`

- Reasoning: `high` by default, `medium` for fully specified work, and `xhigh` for hard or ambiguous steps.

### `openai/gpt-6.1-sol`

- Reasoning: `high` by default; `xhigh` is available.
- Fallback: Opus when OpenAI is capped; route those steps to `claude/claude-opus-5-5#high` yourself.
- Fast and cheap with plenty of usage, so use Sol alone for small checks and keep the Sol `review/*` and `verify/*` routes for routine work.
- Best `build/scribe` for quick patches, edits, and comment passes, and best at browser QA and computer use.
- Gathers wide context well and makes sound architecture calls.
- Strength looks close to Opus but is untested; expect an occasional dumb miss.

### `openai/gpt-6-astra`

- Use only as a **council seat** or **parallel leaf**, never as the primary.
- Reasoning: `high`, `xhigh`, and `max` are available.
- Fallback: Opus when OpenAI is capped, because Astra and Sol share one OpenAI window.
- Strength relative to Sol and Opus on this work is untested; record evidence as councils compare it.

### `claude-sonnet-5-5`

- Available as an explicit pick when speed matters; no route defaults to Sonnet.
- Orchestrates web search and `x` calls well.

### `xai/grok-4.7`

- Grok is available through `xai`, `cursor`, and `opencode-go`; scout routes use Cursor's C pool.
- Route web scouting and quick context searches there freely; treat findings as leads and verify claims that decide a design.
- The `x` tool runs the Grok CLI on the xAI pool for native X search; Collab keeps it for `/x` and routes other X research to scouts or verifiers.
- A standing seat in web councils; see Scouts.

### `opencode-go/deepseek-v4.1-flash`

- A standing seat in web councils; see Scouts.
- Costs cents per scout against an $80 monthly Go cap.
- On one Go-tools brief in October 2026 it nearly matched Opus and found facts the others missed, at about 6 minutes per run.

### Fallback providers

- `cursor/*`: C and O are separate monthly pools on a legacy Cursor plan.
  - C covers Cursor models: Grok, Composer, and Auto; Grok checks only C and falls back to Go when C is capped, leaving O for Claude overflow.
  - O covers third-party models such as Opus and Sonnet and is small.
  - About **1,200 Opus fast steps** used up a month.
  - Use it for **short, bounded jobs**.
  - Claude fallbacks use `claude-opus-5-5` or `claude-sonnet-5-5`; Fable is admin-blocked.
  - In one run each on the same known-answer code trace, Cursor Opus scored **5/5 in 2 minutes** and Grok 4.7 scored **4/5 in 3 minutes**.
  - Composer 2.5 scored **4/5** but drifted for **10 minutes and 195 tool calls** on its one run, so it is not used.
  - Lanes skip a Cursor route when its fresh pool window reads 100% and has not reset.
    Missing, stale, or unusable usage counts as open and adds a note.
- `opencode-go/*`: currently `grok-4.7` and `deepseek-v4.1-flash`; the best models there change often, and Grok is the second fallback hop for Claude, after Cursor, or the direct fallback for Cursor Grok.
  - Each Go model has its own monthly dollar cap, with 20% per 5 hours and 50% per week; Grok's is $15, so one Grok scout spends about 15% of a week.

## Usage

Usage is internal routing data.
Check `usage_status` before any dispatch outside `claude` or `cursor`, route those steps around known caps yourself, and choose routes without reporting headroom or account switches.
Mention usage only when the user asks, or when every route for a step is capped.

- With three accounts, Claude headroom is the norm, so every Claude route defaults to Opus.
- OpenAI's weekly window tends to reset mostly unspent, so Sol and Astra leaves stay cheap to add.
