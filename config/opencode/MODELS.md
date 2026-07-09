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
Supported GPT-5.6 efforts are `none`, `low`, `medium`, `high`, `xhigh`, and `max`.
Pass `effort` only for models with effort variants; `xai/grok-4.5` defaults to high reasoning, with `medium` and `low` as opt-down overrides.
For Grok leaves, prefer medium by default and high when reasoning load rises; do not route Grok at `low` as a default.
Fast GPT routing uses the priority service tier for Sol and Terra, never a `-fast` model ID.

## Usage and cost awareness

Before expensive or fanout routing, check usage and limit state when available.
Prefer cheaper or less-constrained providers when the preferred provider is near or at its limit.
For simple independent leaf work, shift first to `xai/grok-4.5`.
When Sol priority or cost is constrained, use Terra for its moderate or harder bounded GPT seats and Luna for utility leaves.
If GPT access is constrained, let `xai/grok-4.5` or `opencode-go/glm-5.2` take work that fits their failure modes.
Cost and limit state are routing inputs, but quality gates still apply on final synthesis, acceptance, and risky edits.

## Routing defaults

| Work                                                                                                     | Model                       | Effort |
| -------------------------------------------------------------------------------------------------------- | --------------------------- | ------ |
| Primary orchestration and synthesis                                                                      | `openai/gpt-5.6-sol`        | high   |
| Secondary (oftend limited) orchestration and synthesis                                                   | `anthropic fable`           | high   |
| Concise spec writing, tight brief only                                                                   | `anthropic` fable           | medium |
| Mechanical relays, classification, and other small utility seats                                         | `xai/grok-4.5`              | medium |
| Independent throughput leaves, quick patches, and clear builds                                           | `xai/grok-4.5`              | high   |
| Moderate multi-concern coordination and bounded implementation                                           | `openai/gpt-5.6-terra`      | high   |
| Relays, summaries, `scout/*` passes, simple commits, and clear bounded work requiring tool judgment      | `openai/gpt-5.6-terra`      | medium |
| Harder bounded builds                                                                                    | `openai/gpt-5.6-sol`        | medium |
| Ambiguous, high-stakes, multi-file, TypeScript, business logic, integration, or logistics implementation | `openai/gpt-5.6-sol`        | high   |
| Unclear builds, or escalation after Grok fails or underperforms                                          | `openai/gpt-5.6-sol`        | high   |
| Deep review (debug, security, critic) and acceptance verification                                        | `openai/gpt-5.6-sol`        | xhigh  |
| Exceptional quality-first work with a measured reason                                                    | `openai/gpt-5.6-sol`        | max    |
| HTML/CSS, visual design decisions, and UX/UI client surface                                              | `anthropic/claude-opus-4-8` | high   |
| Dissent probes and council copies                                                                        | `opencode-go/glm-5.2`       | high   |
| Dissent probes and council copies                                                                        | `xai/grok-4.5`              | high   |

Grok remains the default independent throughput leaf for clear work and deliberate quick patches; it's very fast and accurate.
Luna owns utility seats and clear bounded GPT work when its medium-effort tool judgment is useful.
Terra's workhorse seat is provisional because day-zero evidence suggests Luna or Sol may dominate it.
Escalate on multi-concern risk, acceptance stakes, or observed failure modes.

## Model ledger

Rate and judge here as evidence lands; verdicts are provisional and should say how they could be wrong.

| Model                       | Verdict                                                                                         | Strengths                                                                      | Failure modes                                                                                                               | Last judged |
| --------------------------- | ----------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `openai/gpt-5.6-sol`        | Primary orchestration and synthesis seat, plus high-stakes implementation and deep acceptance.  | Multi-concern reasoning, synthesis, and sustained implementation judgment.     | Demote if high-effort outputs repeatedly lose acceptance or fail to justify their cost and latency against Luna or Terra.   | 2026-07-09  |
| `openai/gpt-5.6-terra`      | Provisional moderate-complexity GPT workhorse and Sol fallback under priority or cost pressure. | Balanced bounded implementation and multi-concern coordination.                | Displace if Luna meets its acceptance rate on comparable tool-dependent work or Sol's extra quality justifies its overhead. | 2026-07-09  |
| `openai/gpt-5.6-luna`       | Utility seat for relays, scouts, simple commits, and clear bounded GPT work.                    | Low-cost mechanical work and medium-effort tool judgment.                      | Escalate its seats if medium-effort leaves repeatedly miss tool choices, scope, or acceptance criteria.                     | 2026-07-09  |
| `anthropic` fable           | Concise spec and Markdown specialist; it no longer owns primary orchestration.                  | Restrained Markdown and tight briefs; other models can over-write prose.       | Expensive; wasted on relay leaves; availability drama.                                                                      | 2026-07-08  |
| `anthropic/claude-opus-4-8` | HTML/CSS, visual design decisions, and UX/UI client surface.                                    | Visual, UX, and product-shape reasoning.                                       | Anecdotes of plan regressions vs fable; scarce usage can bottleneck.                                                        | 2026-07-08  |
| `opencode-go/glm-5.2`       | Independent provider lens and cheap implementer candidate.                                      | Different failure modes; useful when GPT or Opus usage is constrained.         | Not a selector; agreement without independent evidence is noise.                                                            | 2026-07-08  |
| `xai/grok-4.5`              | Default independent throughput leaf for quick delegation, patches, and clear builds.            | Speed; tool-use potential; independent provider lens; live X/community signal. | Provisional: watch tool-judgment misses, incomplete patches, and overconfidence on ambiguous builds.                        | 2026-07-08  |

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
Use `low` for Luna's mechanical relays, classification, and similar utility work.
Use `medium` for Luna tool-judgment work, Terra's moderate bounded work, and the Grok floor for independent leaves.
Use `high` for Sol orchestration and high-stakes implementation, Terra's harder bounded builds, and Grok evidence or dissent seats.
Use `xhigh` for Sol review and acceptance where a missed flaw is costly.
Use `max` only for exceptional Sol tasks with a measured quality-first reason.
Bias toward the stronger model or higher effort when unsure; never run whole fleets at xhigh.
Default independent leaf speed bias is Grok at medium or high; do not starve Grok of trial builds while trust is forming.

## Failure handling

Effort names are model-specific; an invalid effort returns an error listing valid efforts, so re-pick from that list.
Provider allowlist errors mean the requested provider is missing from `delegate.json`; re-pick an allowed provider or report the missing policy.
`task_id` resume can hard-fail on evicted child sessions; recover by re-briefing a fresh child from the durable brief.
