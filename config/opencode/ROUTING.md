---
routes:
  build/owner: claude/claude-opus-5-5 high
  build/general: claude/claude-opus-5-5 high
  build/git: claude/claude-opus-5-5 high
  build/patch: claude/claude-opus-5-5 medium
  build/scribe: gemini/gemini-4-neon high
  scout/*: claude/claude-opus-5-5 high
  scout/context: claude/claude-sonnet-5-5 high
  scout/dirty: claude/claude-sonnet-5-5 high
  scout/session: openai/gpt-6.2-sol high
  scout/web: xai/grok-4.7 high
  review/*: openai/gpt-6.2-sol high
  review/copy: gemini/gemini-4-neon high
  review/debug: gemini/gemini-4-neon high
  review/security: gemini/gemini-4-neon medium
  review/simplify: gemini/gemini-4-neon high
  review/entropy: claude/claude-opus-5-5 xhigh
  verify/*: openai/gpt-6.2-sol high
  verify/decide: claude/claude-fable-5-2 high
fallbacks:
  claude/claude-fable-5-2: claude/claude-opus-5-5
  claude/claude-opus-5-5: cursor/claude-opus-5-5
  claude/claude-sonnet-5-5: cursor/claude-sonnet-5-5
  cursor/claude-opus-5-5: opencode-go/grok-4.7
  cursor/claude-sonnet-5-5: opencode-go/grok-4.7
  gemini/gemini-4-neon: openai/gpt-6.2-sol
  xai/grok-4.7: opencode-go/grok-4.7
---

# Routing

Frontmatter sets each agent's default model and effort; the body sets when to depart from them.
Treat everything here as defaults; state the reason when departing, and confirm each decision with the user before editing this file unless already approved.

## Routes

Frontmatter routes use `provider/model [variant]`, while explicit `subagent` model values use `provider/model#variant`.
An exact agent name beats a glob, and a longer glob beats a shorter one.
The `claude` provider is the Anthropic account pool; the lanes plugin resolves it to `anthropic`, `anthropic-personal`, or `anthropic-cullyn` for each call.
Each `fallbacks` entry names the next model to try when the source model's pool is capped; entries form a chain.

Main sessions belong to the user and are not routed here.
`ctrl+m` and main-session failover pick only Cogikyo or Cullyn.
On the first turn, main sessions matching the agent or config default on either account take the `ctrl+m` pick; later manual picks are untouched.

Omit `model` on `subagent` to take the route; an explicit `provider/model#variant` never inherits the route's variant, and omitting `#<variant>` uses the model's default.
Use `claude/<model>#<variant>` (such as `claude/claude-opus-5-5#xhigh`) to keep Anthropic account selection.

## Effort ladder

Pick effort for each workflow step from the task; route variants are defaults.

- **`medium`**: mechanical or fully specified work, such as patches, lookups, data pulls, and builds whose brief settles every choice.
- **`high`**: bounded judgment, such as builders with a clear brief, focused scouts, and routine reviews.
- **`xhigh`**: hard or ambiguous work, such as tricky debugging, cross-cutting builds, and the final review of a risky change.
- **`max`**: only when the user asks for it, or for a sweep the user requested.

Picks of `medium` or `high` follow guidance; `xhigh` and `max` count as departures and carry a `why:` on the workflow card, even when the route defaults to that effort.
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
- Main-session caps and transient credential failures resend on the `ctrl+m` pick and save that account; lane stickiness lasts only until the cap resets.
- Pass `anthropic/…`, `anthropic-personal/…`, or `anthropic-cullyn/…` only when the user names an account; that pins the account and skips the pick.
- Opus and Sonnet both take `medium` through `max`; pick by the effort ladder.
- Login failures block without moving accounts.

## Which agent

Use the `subagent` agent descriptions and these guidelines to pick a child.

### Scouts

- Use `scout/context` and `scout/dirty` on Sonnet `high` for rough first passes; skip `scout/context` when the brief names the files or the question is already bounded.
- Send focused questions to `scout/code` for local code behavior, readers and consumers, end-to-end traces, and live or legacy paths.
- Use `scout/library` before writing a helper that likely exists in the stdlib or the repository.
- Send shell checks needing network, sockets, daemons, or credentials to `verify/test` or Collab's shell, not scouts, reviewers, or source/web verifiers.
- Use `scout/web` for the option space and prior art; it routes to xAI Grok and calls `x` by default, and other scouts and source/web verifiers can also call `x`.
- Run web research as a council by default, because web sources are noisy and seat diversity earns its cost.
  - Seat `scout/web` on its route beside `opencode-go/deepseek-v4.1-flash#high` and `openai/gpt-6.2-sol#high`; `claude/claude-sonnet-5-5#high` is also an acceptable seat.
  - Add an Opus seat, `claude/claude-opus-5-5#high`, when the answer decides a design.
  - Check `usage_status` before seating the council and drop a known-capped seat before lanes substitutes a fallback; when the shared xAI pool is capped, skip `x` and seat Sonnet `high` or `opencode-go/grok-4.7#high` yourself.

### Reviewers

- Neon leads code review; Sol keeps the other `review/*` routes and sits beside Neon as a seat.
- After implementation, pair **`review/debug`** (correctness) with **`review/simplify`** (idiom, dead code, cleanliness); keep `review/debug` alone for small fixes.
- Add `review/security`, `review/profile`, `review/architect`, and `review/copy` as risks warrant; run `review/copy` before commit on frontend UI text changes.
- `review/debug` and `review/simplify` route to Neon `high`; seat `openai/gpt-6.2-sol#high` beside each whenever OpenAI usage allows, except on small fixes.
  - When Gemini is capped, pass `claude/claude-opus-5-5#high` yourself, and fall to Sol only when Claude is capped too, because Neon's automatic fallback is Sol.
- `review/security` routes to Neon `medium` with an unconditional `openai/gpt-6.2-sol#high` seat beside it (not gated on Gemini quota); log one `LEDGER.md` row per run.
- When Neon and Sol review the same change, base the synthesis on Neon's report.
  - Verify Sol-only findings by reproduction or source check before they drive a repair.
  - Check every blocker from either seat before it blocks: Neon's often predate the diff, and Sol's often rest on source reads the code can't reach.
  - Log one `LEDGER.md` row per debug or simplify run with both seats, giving held and dropped findings per seat, until about 10 runs.
- Keep `review/security` and adversarial briefs off Opus, because Anthropic's cyber filter blocks it there.
  - Adversarial means bypass hunts, exploit paths, or attacks on keys, sandboxes, or guards; Opus can still take routine security passes (secrets, auth).
- Run `review/entropy` on request, on package or app sweeps, or as a retro after a feature lands; on a small diff it produces rewrite noise.

### Experimental seats

While Gemini's 5-hour and weekly windows both have at least half their quota left, add a different-family seat beside the routed model and append one comparison row to `LEDGER.md`.

- Seat `gemini/gemini-4-neon#medium` beside hard `scout/code` traces.
- Seat `gemini/gemini-4-hydrogen` beside simple scouts.
- Let the routed model's result drive the work; a seat earns a route in `LEDGER.md` on findings that held up, not on findings raised.

### Builders

Pick a builder by how much the brief settles.

- Route new or rewritten prose and comments to `build/scribe` (name its role and skill files in the brief) even inside a larger build; builders own the code and add no comments.
  - Give any Neon `build/scribe` pass that deletes or cuts rules a Sol review checking nothing load-bearing was lost, and log one `LEDGER.md` row per check recording clean or repaired until about 10 clean runs.
- `build/git` suits one approved Git workflow too large for this context.

### Verifiers

- Use a verifier when a claim decides the design or verdict.
- Launch `verify/decide` on its route only for `COLLAB.md`'s critical calls; its model entry below covers cost and caps.

## When to deviate

- Send trivial edits to `build/patch` (Opus `medium`), keeping a `{scope}-patch` lane open for repeated patches in the same scope.
- Add Sol (`openai/gpt-6.2-sol#high`) or Astra (`openai/gpt-6.2-astra#high`) leaves eagerly beside Opus work; Collab picks between them.
  - Run a parallel OpenAI scout on the same brief when a second independent read could change the answer.
  - Use `verify/*` to settle an open claim, to double-check a claim the answer or design rests on, or as a council seat that guards against shared assumptions.
  - Prefer `verify/web` or an OpenAI `scout/web` over answering external facts from memory.
- Convene a council for harness changes, production-impact findings, and design verdicts, with a **fresh Opus reviewer** (`claude/claude-opus-5-5#high`) beside Sol or Astra.
  - Reserve the Opus seat for those cases; other councils run Sol and Astra seats.
  - Give Opus a different lens when risks span lenses, but not an adversarial one; keep seats on the same lens only for high-stakes changes where model diversity is the point.
  - For repo-wide `review/entropy` sweeps, run the Opus `xhigh` lane beside Astra at `high`.
- `review/critic` routes to Sol; pass `claude/claude-opus-5-5#high` only when it filters OpenAI findings, and brief it with those findings.

## Models

Listed in rough order of overall preference; pick effort from the ladder above.

### `claude-opus-5-5`

- Reasoning: `high` by default, `medium` for fully specified work, and `xhigh` for hard or ambiguous steps.
- Evidence: `scout/code` median 2.4m (n=485); on the improve council, `high` found 12 items in 9.6m ($3.93) and beat Neon on PR house form (see `LEDGER.md`).

### `claude-fable-5-2`

- Routes only `verify/decide` at `high`; keep it there rather than raising effort for a risky change.
- Decide falls back Fable → `claude/claude-opus-5-5#high` → `openai/gpt-6.2-astra#high`; Sol makes no decide calls (and shares Astra's weekly window).
  - Check usage before launch and pass the first open pick, because automatic hops drop the variant and continue through Cursor to Grok.
  - If all three are capped, follow `COLLAB.md`'s capped-decider rule.
- Evidence: 61 decide runs since Nov 13 averaged ~$2.40 (3–15m, 2× Opus/token); decide replay scored 3/4 (Sol 3/4, Astra 2/4), and one council run found 15 items in 16m at $15.20 (see `LEDGER.md`).

### `openai/gpt-6.2-sol`

- Reasoning: `high` by default; `xhigh` is available.
- Fallback: route to `claude/claude-opus-5-5#high` yourself when OpenAI is capped; on a provider-policy refusal, relaunch once on `gemini/gemini-4-neon#high` instead of retrying Sol.
- Fast and cheap with ample headroom; keep its `verify/*` and remaining `review/*` routes, seat it beside Neon reviews and in web councils, and use it alone for small checks.
- Best at browser QA and computer use, gathers wide context well, makes sound architecture calls, and serves as Neon's fallback when Gemini is capped.
- Strength looks close to Opus but is untested; expect an occasional dumb miss.

### `openai/gpt-6.2-astra`

- Use as a **council seat** or **parallel leaf**; it leads decide only when Fable and Opus are capped.
- Reasoning: `high`, `xhigh`, and `max` are available.
- Fallback: Opus when OpenAI is capped, because Astra and Sol share one OpenAI window.
- Evidence: `high` returned 6 council findings in 5.4m (fastest seat) and scored 2/4 on the decide replay; still thin, so keep logging in `LEDGER.md`.

### `gemini-4-neon`

- Reasoning: `medium` for `review/security`, seats, and scouts, `high` for scribe, copy, debug, simplify, and Sol-refusal reruns; `xhigh` is untested.
- Fallback: Sol when either Gemini window is capped.
- Routes `build/scribe`, `review/copy`, `review/debug`, `review/simplify`, and `review/security` (the security route rests on one run).
- Seat it at `medium` beside hard `scout/code` traces under the experimental phase, and log comparisons in `LEDGER.md`.
- Gets only the brief, so name the role file and any skill files it needs.
- Evidence: `high` beat Sol (+4 verified bugs) on one review and had scribe cohort medians of 9.3m (n=9) and 6.3m (n=4, 4/13 repaired); `medium` matched Opus's raw finding count (12 each, 7.5m vs 9.6m) on one council (see `LEDGER.md`).

### `gemini-4-hydrogen`

- Reasoning: `low` to `xhigh`; one `high` review took ~3m (known bug only, 0/4 extras; see `LEDGER.md`), untested on scouts.
- May suit fast scouts; trial it as a seat beside simple scouts before routing it.
- Shares Gemini's windows with Neon, gets only the brief, and has no fallback yet.

### `claude-sonnet-5-5`

- Routes `scout/context` and `scout/dirty` at `high`; also available as an explicit pick when speed matters.
- Orchestrates web search and `x` calls well.

### `xai/grok-4.7`

- Available through `xai`, `cursor`, and `opencode-go`; `scout/web` routes to the xAI pool (shared with `x`) as a standing web-council seat and falls back to `opencode-go/grok-4.7`.
- Verify claims that decide a design, and weigh findings from Grok (any provider) or Cursor's own models below Opus, Sol, and Fable.
  - Claude served through Cursor's O pool is the same model and keeps full weight.
- The `x` tool runs the Grok CLI on the xAI pool for native X search; Collab keeps it for `/x` and routes other X research to scouts or verifiers.

### `opencode-go/deepseek-v4.1-flash`

- A standing seat in web councils; see Scouts.
- Costs cents per scout against an $80 monthly Go cap.
- On one Go-tools brief in October 2026 it nearly matched Opus and found facts the others missed, at about 6 minutes per run.

### Fallback providers

- `cursor/*`: legacy plan with two monthly pools: C (Cursor models: Grok, Composer, Auto) and small O (third-party models like Opus and Sonnet; ~**1,200 Opus fast steps**/month, for **short, bounded jobs**).
  - Claude fallbacks use `claude-opus-5-5` or `claude-sonnet-5-5` (Fable is admin-blocked); Cursor Opus scored **5/5 in 2m** on one known-answer trace.
  - Cursor Grok (8 web scouts: 54m median, 4 no report) and Composer 2.5 (**4/5**, 10m and 195 tool calls) are not routed.
  - Lanes skip a Cursor route when its fresh pool window reads 100% before reset; missing, stale, or unusable usage counts as open with a note.
- `opencode-go/*`: currently `grok-4.7` (second fallback hop for Claude after Cursor, and first for xAI) and `deepseek-v4.1-flash`; top models change often.
  - Each Go model has its own monthly dollar cap (20% per 5h, 50% per week); Grok's is $15 (~15% of a week per scout).

## Usage

Usage is internal routing data.
Check `usage_status` before any dispatch outside `claude` or `cursor` and route around known caps yourself.
Keep headroom and same-model account switches silent, and mention usage only when the user asks or when every route for a step is capped.
When a fallback replaces a council seat's model, name that substitution with the seat's result, because a different model changes what the evidence means.

- With three accounts, Claude headroom is the norm.
- OpenAI's weekly window tends to reset mostly unspent, so Sol and Astra leaves stay cheap to add.
