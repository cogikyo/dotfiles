# Models

Human-editable source of truth for adaptive model routing: what to use, why, and how well each model has been earning its seat.
Primaries read this before routing leaves; Markdown is never auto-included.
Defaults live here; adapt to task shape, usage headroom, provider limits, cost pressure, and observed model performance.

Doctrine: intelligence at the membrane, throughput in the leaves.
Stronger models earn seats when judgment, ambiguity, multi-concern coordination, or risk requires them.

## Call contract

The `task` tool accepts `model` as `provider/model-id` and `effort` per call.
Name both on every routed leaf unless the parent intentionally lets the session default apply.
Sol and Terra priority routing uses provider service-tier config and their ordinary IDs.

## Usage and cost awareness

Before expensive or fanout routing, check usage and limit state when available.
Prefer cheaper or less-constrained providers when the preferred provider is near or at its limit.
Use Terra first for routine independent tool work, including utility and scout leaves.
When Sol priority or cost is constrained, use Terra for bounded GPT seats, including harder utility work.
If GPT access is constrained, let `xai/grok-4.5` or `opencode-go/glm-5.2` take only bounded work that fits their failure modes.
Cost and limit state are routing inputs, but quality gates still apply on final synthesis, acceptance, and risky edits.

## Routing defaults

| Work                                                                                 | Model                       | Effort |
| ------------------------------------------------------------------------------------ | --------------------------- | ------ |
| Primary orchestration and synthesis                                                  | `openai/gpt-5.6-sol`        | high   |
| Secondary (oftend limited) orchestration and synthesis                               | `anthropic fable`           | high   |
| Concise spec writing, tight brief only                                               | `anthropic` fable           | medium |
| Mechanical relays, classification, and other small utility seats                     | `openai/gpt-5.6-terra`      | medium |
| Routine `scout/*` passes and clear bounded work requiring tool judgment              | `openai/gpt-5.6-terra`      | medium |
| Moderate multi-concern coordination and bounded implementation                       | `openai/gpt-5.6-terra`      | high   |
| Deep review (debug, security, critic) and acceptance verification                    | `openai/gpt-5.6-terra`      | xhigh  |
| Deliberate quick patches and clear mechanical builds after a concrete brief          | `xai/grok-4.5`              | medium |
| Tightly specified, bounded mechanical build slice from an approved objective or spec | `xai/grok-4.5`              | medium |
| Standard intial debug review, often finds issue fast and is correct                  | `xai/grok-4.5`              | high   |
| Ambiguous, high-stakes, multi-file, ambitious implementation                         | `openai/gpt-5.6-sol`        | high   |
| Escalation after other agents fails or underperforms                                 | `openai/gpt-5.6-sol`        | high   |
| HTML/CSS, visual design decisions, and UX/UI client surface                          | `anthropic/claude-opus-4-8` | high   |
| Often good at planning, inventing, scheming. Good default .spec/ editing             | `anthropic/claude-opus-4-8` | high   |
| Dissent probes and council copies                                                    | `anthropic/claude-opus-4-8` | high   |
| Dissent probes and council copies                                                    | `opencode-go/glm-5.2`       | high   |
| Dissent probes and council copies                                                    | `xai/grok-4.5`              | high   |

### Rules of Thumb:

Grok is a fast opt-in seat for clear mechanical implementation, especially when a human can steer the result.
Grok may provide advisory adversarial dissent, council copies, `verify/x` evidence, and an advisory `review/architect` dissent.
Grok usage limist are lower than others at this time.
Terra owns utility, scout, routine tool-judgment, simple GPT, and moderate bounded seats.
Luna receives no routing seats until ChatGPT Codex OAuth availability works on this account/backend.
Escalate on multi-concern risk, acceptance stakes, or observed failure modes.

## Model ledger

Rate and judge here as evidence lands; verdicts are provisional and should say how they could be wrong.

| Model                       | Verdict                                                                                                    | Strengths                                                                      | Failure modes                                                                                                                  | Last judged |
| --------------------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| `openai/gpt-5.6-sol`        | Primary orchestration and synthesis seat, plus high-stakes implementation and deep acceptance.             | Multi-concern reasoning, synthesis, and sustained implementation judgment.     | Demote if high-effort outputs repeatedly lose acceptance or fail to justify their cost and latency against Terra.              | 2026-07-09  |
| `openai/gpt-5.6-terra`      | Moderate bounded GPT workhorse plus utility, scout, and simple GPT seats.                                  | Balanced bounded implementation and multi-concern coordination.                | Displace if Grok meets its acceptance rate or Sol's extra quality justifies its overhead.                                      | 2026-07-10  |
| `openai/gpt-5.6-luna`       | Valid model ID with no routing seats while unavailable through this account's ChatGPT Codex OAuth backend. | Candidate for low-cost mechanical work and medium-effort tool judgment.        | Inference returns `Model not found gpt-5.6-luna`; retest when OAuth catalog availability changes.                              | 2026-07-10  |
| `anthropic` fable           | Concise spec and Markdown specialist; it no longer owns primary orchestration.                             | Restrained Markdown and tight briefs; other models can over-write prose.       | Expensive; wasted on relay leaves; availability drama.                                                                         | 2026-07-08  |
| `anthropic/claude-opus-4-8` | HTML/CSS, visual design decisions, and UX/UI client surface.                                               | Visual, UX, and product-shape reasoning.                                       | Anecdotes of plan regressions vs fable; scarce usage can bottleneck.                                                           | 2026-07-08  |
| `opencode-go/glm-5.2`       | Independent provider lens and cheap implementer candidate.                                                 | Different failure modes; useful when GPT or Opus usage is constrained.         | Not a selector; agreement without independent evidence is noise.                                                               | 2026-07-08  |
| `xai/grok-4.5`              | Intentional fast mechanical implementation seat after a concrete brief, plus dissent and live-X evidence.  | Speed; tool-use potential; independent provider lens; live X/community signal. | Provisional: watch tool-judgment misses, incomplete patches, and overconfidence when scope or implementation shape is unclear. | 2026-07-10  |

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
Use `none` only when the model and task make reasoning unnecessary.
Use `low` only for mechanical work when the routed model's evidence supports the opt-down.
Use `medium` for Terra's utility, scout, simple, and clear bounded work, and for deliberately routed Grok mechanical builds.
Use `high` for Sol orchestration and high-stakes implementation, Terra's harder bounded builds, and Grok evidence or dissent seats.
Use `xhigh` for deep review or tough orchestration; where acceptance where a missed flaw is costly.
Use `max` pretty much useless, never use.

## Failure handling

Effort names are model-specific; an invalid effort returns an error listing valid efforts, so re-pick from that list.
Provider allowlist errors mean the requested provider is missing from `delegate.json`; re-pick an allowed provider or report the missing policy.
`task_id` resume can hard-fail on evicted child sessions; recover by re-briefing a fresh child from the durable brief.
