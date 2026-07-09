# Models

Human-editable source of truth for adaptive model routing: what to use, why, and how well each model has been earning its seat.
Primaries read this before routing leaves; Markdown is never auto-included.
Defaults live here; adapt to task shape, usage headroom, provider limits, cost pressure, and observed model performance.

Doctrine: intelligence at the membrane, throughput in the leaves.
`xai/grok-4.5` is the default throughput leaf (fastest seat for quick delegation and clear builds).
Stronger models earn seats when judgment, ambiguity, multi-concern coordination, or risk requires them.

## Call contract

The `task` tool accepts `model` as `provider/model-id` and `effort` per call.
Name both on every routed leaf unless the parent intentionally lets the session default apply.
Pass `effort` only for models with effort variants; `xai/grok-4.5` defaults to high reasoning, with `medium` and `low` as opt-down overrides.
For Grok leaves, prefer medium by default and high when reasoning load rises; do not route Grok at `low` as a default.

## Usage and cost awareness

Before expensive or fanout routing, check usage and limit state when available.
Prefer cheaper or less-constrained providers when the preferred provider is near or at its limit.
For simple or independent leaf work, shift first to `xai/grok-4.5`; keep `openai/gpt-5.5-fast` for heavier leaf shape or after Grok fails.
If Opus is near limit, let `xai/grok-4.5`, `openai/gpt-5.5-fast`, or `opencode-go/glm-5.2` take more work.
If GPT priority usage is constrained, shift remaining GPT-shaped work to `openai/gpt-5.5`, `xai/grok-4.5`, or `opencode-go/glm-5.2` when they fit.
Cost and limit state are routing inputs, but quality gates still apply on final synthesis, acceptance, and risky edits.

## Routing defaults

| Work                                                                      | Model                                        | Effort   |
| ------------------------------------------------------------------------- | -------------------------------------------- | -------- |
| Primary orchestration and synthesis                                       | `anthropic` fable                            | high     |
| Relays, summaries, `scout/*` passes, simple commits                       | `xai/grok-4.5`                               | medium   |
| Quick patches and small bounded edit slices                               | `xai/grok-4.5`                               | medium   |
| Clear / very-bounded builds and verify runs (trust-earning default)       | `xai/grok-4.5`                               | medium   |
| Ambiguous, multi-file, TypeScript, business logic, integration, logistics | `openai/gpt-5.5-fast`                        | medium   |
| Multi-patch commit detangling and moderate multi-concern coordination     | `openai/gpt-5.5-fast`, then `openai/gpt-5.5` | medium   |
| Unclear builds, or escalate after Grok fails / underperforms              | `openai/gpt-5.5-fast`                        | medium   |
| Deep review (debug, security, critic) and acceptance verification         | `openai/gpt-5.5-fast`                        | xhigh    |
| HTML/CSS, visual design decisions, and UX/UI client surface               | `anthropic/claude-opus-4-8`                  | high     |
| Concise spec writing, tight brief only                                    | `anthropic` fable                            | medium   |
| Dissent probes and council copies                                         | `opencode-go/glm-5.2`                        | none     |
| Dissent probes and council copies                                         | `xai/grok-4.5`                               | high     |
| GPT reserve when fast or priority usage is constrained                    | `openai/gpt-5.5`                             | low-high |

When a build is clear and bounded, try Grok first and give deliberate trial seats while trust is forming.
When ambiguity is moderate, occasional Grok trial builds are fine; escalate on multi-concern risk, acceptance stakes, or observed failure modes.

## Model ledger

Rate and judge here as evidence lands; verdicts are provisional and should say how they could be wrong.

| Model                       | Verdict                                                                                                          | Strengths                                                                                     | Failure modes                                                                                    | Last judged |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ----------- |
| `anthropic` fable           | Orchestration seat, almost exclusively; maybe concise spec/md writing.                                           | Planning, judgment, synthesis, restrained Markdown; other models over-write prose.            | Expensive; wasted on relay leaves; availability drama.                                           | 2026-07-08  |
| `anthropic/claude-opus-4-8` | HTML/CSS, visual design decisions, and UX/UI client surface.                                                     | Visual, UX, and product-shape reasoning.                                                      | Anecdotes of plan regressions vs fable; scarce usage can bottleneck.                             | 2026-07-08  |
| `openai/gpt-5.5-fast`       | Escalation / complex leaf workhorse when Grok is insufficient or the task shape is heavier.                      | Ambiguous multi-file work, TS/business logic, integration, multi-patch detangle, deep review. | Premium $/token; keep off final review and acceptance when usage is tight.                       | 2026-07-08  |
| `openai/gpt-5.5`            | Reserve GPT fallback when fast or priority usage is constrained.                                                 | Same family fallback when usage limits or cost state argue against the fast lane.             | Slower; do not use just because it feels more serious.                                           | 2026-07-08  |
| `openai/gpt-5.4-mini-fast`  | Rarely needed; `gpt-5.5` low covers its seats with better tool judgment.                                         | Cheap bulk fanout when cost truly dominates.                                                  | Weaker judgment shows up exactly when a leaf must decide something.                              | 2026-07-08  |
| `opencode-go/glm-5.2`       | Independent provider lens and cheap implementer candidate.                                                       | Different failure modes; useful when GPT or Opus usage is constrained.                        | Not a selector; agreement without independent evidence is noise.                                 | 2026-07-08  |
| `xai/grok-4.5`              | Default fast leaf for quick delegation, patches, scouts, and clear builds; trust still earning on harder builds. | Speed; tool-use potential; independent provider lens; live X/community signal.                | Provisional: watch tool-judgment misses, incomplete patches, overconfidence on ambiguous builds. | 2026-07-08  |

## Second opinions and council

Use `scout/web` or `verify/x` when a second-opinion probe can expose outside evidence, ecosystem drift, or live community signal.
For contested or high-stakes judgments, form a democratic council by rerunning the same `review/*`, `verify/web`, or `verify/source` brief as parallel copies on `opencode-go/glm-5.2`, `xai/grok-4.5`, or another model with distinct failure modes.
When usage headroom is plentiful, it is fine to dispatch secondary agents with the same or adjacent task for second opinions, extra critiques, plan reviews, or review disagreement.
Synthesize the council on the primary session.
Agreement counts only when the copies cite independent evidence.
Disagreement is a finding and should be preserved.
Notice if a model is repeatedly strong or repeatedly making mistakes, then mention that to the user during review or reporting and suggest updating this file.

## Effort guidance

Start at the default in the routing table; escalate only when uncertainty changes the outcome.
Medium is the Grok floor for relays, scouts, patches, and clear builds.
High buys sustained reasoning for implementation, evidence gathering, and dissent seats.
Xhigh is reserved for review and acceptance where a missed flaw is costly.
Bias toward the stronger model or higher effort when unsure; never run whole fleets at xhigh.
Default leaf speed bias is Grok at medium or high; do not starve Grok of trial builds while trust is forming.

## Failure handling

Effort names are model-specific; an invalid effort returns an error listing valid efforts, so re-pick from that list.
Provider allowlist errors mean the requested provider is missing from `delegate.json`; re-pick an allowed provider or report the missing policy.
`task_id` resume can hard-fail on evicted child sessions; recover by re-briefing a fresh child from the durable brief.
