# Fleet expansion: learn mode, alt-provider leaves, council doctrine

Mid-drive scope expansion approved by the user's message of 2026-07-04; user AFK 8+ hours, this doc is the durable brief if the session dies.
Goal: a fourth primary mode `learn`, dedicated alt-provider leaves, council/supplement doctrine in the primaries, and surviving invented leaves.
End state: artifacts committed, doctrine landed, this doc deleted.

## Approved scope (decode of the user's message)

1. New fourth primary mode `learn` alongside scheme/collab/drive at `agents/learn.md`.
   Green colored; leans heavily on web research and the `verify/*` and `scout/*` leaves.
   Purely for understanding, clarifying, and verifying how things work; independent study mode, no code edits.
   Socratic: asks the user questions to prove they roughly know the answers; teaching through questioning.
   Source inspiration: Matt Pocock's skills repo, specifically his teach-style skill (exact name under research); synthesize into an agent mode rather than copying.
2. After the opencode-go and xai usage integration completes (`.spec/delegate.md` runbook, nearly done): dedicated verify and scout leaves pinned to alternative providers, GLM 5.2 and xAI/X web-API search.
   Doctrine: these run in conjunction with mainline web searches as supplements and alternative opinions, accepted as likely slightly less smart.
   Council pattern: GLM 5.2 and grok can run as duplicate copies of other review briefs, democratic-council subagent runs; the parent synthesizes across the copies.
   This council/supplement doctrine gets baked into the primaries' delegation/model-routing guidance.
3. Open invitation: invent additional core primitive leaves inside build/review/verify/scribe where clear workflow value exists; propose, critique, land the survivors.
4. Model budget this week: lots of Anthropic tokens (claude-fable subagents liberally) and lots of gpt-5.5; be liberal with reasoning effort; lots of critiques in core places and rounds of review/simplify.

## Decisions log

- Harness/agent-file edits for this scope were approved by the user's 2026-07-04 message; `scribe/agents` is not currently spawnable, so drive authors harness artifacts directly under that approval, with critic/simplify review rounds before commit.
- Learn is a synthesis of the teach-style skill, never a copy.
- Alt-provider leaves are supplements to mainline search, never replacements.

## Phases

### A. Research + synthesize learn mode --- in progress

Owns: research notes only, no repo files.
Find the exact Matt Pocock teach-style skill; extract the Socratic questioning workflow; synthesize a mode design.

### B. Author learn.md + wire config --- pending

Owns: `agents/learn.md`, `opencode.json`.
Author the mode, run critique + simplify rounds, commit.
Blocked on: A; mode-cycle position open question below.

### C. Alt-provider availability probe + dedicated leaves --- pending

Owns: new `verify/*` and `scout/*` leaf files pinned to GLM 5.2 and xAI/X web-API search.
Probe availability first (GLM 5.2, X search via xai), then author leaves.
Blocked on: `.spec/delegate.md` runbook completion.

### D. Council doctrine into primaries --- pending

Owns: delegation/model-routing guidance in `agents/{scheme,collab,drive}.md`; assumption: `learn.md` gets the same section once it exists.
Bake in the supplement + democratic-council pattern with parent synthesis.
Blocked on: C.

### E. Invented-leaf proposals --- pending

Owns: new leaf files under `agents/{build,review,verify,scribe}/` for survivors only.
Propose, critique, land what survives.

### F. Ongoing critique/simplify rounds --- pending

Owns: no new files; review passes over core places per the budget note in scope item 4.

## Next steps

1. Finish phase A research; identify the exact teach-style skill name.
2. Author and wire `learn.md` (phase B) with critique + simplify rounds; commit.
3. On delegate runbook completion, probe GLM 5.2 and X search availability, then author the alt-provider leaves (phase C).
4. Land council doctrine in the primaries (phase D), then run E and F.

## Queued for user (record only, do not do)

- Carried forward from `.spec/delegate.md`: collab-mode guidance on using xai and opencode-go models well, pending real usage signals; xai burn percent still lacks the inference SSE rate_limits tap, deliberately unbuilt.
- Shared-doctrine triplication across the primaries (~90 lines × 3) and the 24× leaf-contract duplication want a sync ritual through `scribe/agents`.

## Questions for user

- Exact position of learn in the mode cycle keybinds/config, if any preference exists beyond color green.
