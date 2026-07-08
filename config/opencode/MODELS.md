# Models

Human-editable source of truth for model routing: what to use, why, and how well each model has been earning its seat.
Primaries read this before routing leaves; Markdown is never auto-included.
Defaults live here; adapt only when the task carries clear evidence that a different allowed model is better.

Doctrine: intelligence at the membrane, throughput in the leaves.
Orchestration and deep review deserve the strongest models; leaves get the cheapest model that still preserves tool judgment.

## Call contract

The `task` tool accepts `model` as `provider/model-id` and `effort` per call.
Name both for unpinned leaves; let pinned leaves use their frontmatter model.
Pass `effort` only for models with effort variants; xai models have none.

## Routing defaults

| Work                                                              | Model                                       | Effort |
| ----------------------------------------------------------------- | ------------------------------------------- | ------ |
| Primary orchestration and synthesis                               | `anthropic` fable                           | high   |
| Relays, summaries, `scout/*` passes, simple commits               | `openai/gpt-5.5`                            | low    |
| Fast fanout where latency dominates                               | `openai/gpt-5.5-fast`                       | low    |
| Multi-patch commit detangling, moderate coordination              | `openai/gpt-5.5`                            | medium |
| Build slices, verify runs, evidence gathering                     | `openai/gpt-5.5`                            | high   |
| Deep review (debug, security, critic) and acceptance verification | `openai/gpt-5.5`                            | xhigh  |
| Frontend and UI builds                                            | `anthropic/claude-opus-4-8`                 | high   |
| Concise spec writing, tight brief only                            | `anthropic` fable                           | medium |
| Dissent probes                                                    | `opencode-go/glm-5.2`, `xai/grok-4.5`       | pinned |
| Council copies                                                    | `opencode-go/glm-5.2`, `xai/grok-build-0.1` | none   |

## Model ledger

Rate and judge here as evidence lands; verdicts are provisional and should say how they could be wrong.

| Model                       | Verdict                                                                  | Strengths                                                                                        | Failure modes                                                       | Last judged |
| --------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- | ----------- |
| `anthropic` fable           | Orchestration seat, almost exclusively; maybe concise spec/md writing.   | Planning, judgment, synthesis, restrained Markdown; other models over-write prose.               | Expensive; wasted on relay leaves; availability drama.              | 2026-07-08  |
| `anthropic/claude-opus-4-8` | Frontend/UI builds; fallback orchestrator.                               | Visual, UX, and product-shape reasoning.                                                         | Anecdotes of plan regressions vs fable.                             | 2026-07-08  |
| `openai/gpt-5.5`            | Workhorse leaf model across the whole effort ladder.                     | Low effort is efficient enough for tool-heavy relays; high/xhigh strong for build and review.    | Higher efforts can overthink messy tool access.                     | 2026-07-08  |
| `openai/gpt-5.5-fast`       | Latency seat for fanout leaves.                                          | Same weights on priority tier; good when wall-clock matters.                                     | Premium $/token; keep off final review and acceptance.              | 2026-07-08  |
| `openai/gpt-5.4-mini-fast`  | Rarely needed; `gpt-5.5` low covers its seats with better tool judgment. | Cheap bulk fanout when cost truly dominates.                                                     | Weaker judgment shows up exactly when a leaf must decide something. | 2026-07-08  |
| `opencode-go/glm-5.2`       | Pinned `scout/web`; council dissent seat.                                | Independent provider lens with different failure modes; cheap implementer in community pairings. | Not a selector; agreement without independent evidence is noise.    | 2026-07-08  |
| `xai/grok-4.5`              | Pinned `verify/x`.                                                       | Live X/community signal; cheap second seat.                                                      | Vendor-bench hype; no effort variants.                              | 2026-07-08  |
| `xai/grok-build-0.1`        | Council dissent copy.                                                    | Cheap independent dissent for contested judgments.                                               | Code-tier model; keep off prose and synthesis.                      | 2026-07-08  |

## Pinned leaves

`scout/web` is pinned in frontmatter to `opencode-go/glm-5.2`.
`verify/x` is pinned in frontmatter to `xai/grok-4.5`.
Treat both as dissent probes with different failure modes.
Run them alongside mainline evidence passes, never instead of them.

## Second opinions and council

Use `scout/web` or `verify/x` when a second-opinion probe can expose outside evidence, ecosystem drift, or live community signal.
For contested or high-stakes judgments, form a democratic council by rerunning the same `review/*`, `verify/web`, or `verify/source` brief as parallel copies on `opencode-go/glm-5.2` and `xai/grok-build-0.1`.
Synthesize the council on the primary session.
Agreement counts only when the copies cite independent evidence.
Disagreement is a finding and should be preserved.

## Effort guidance

Start at the default in the routing table; escalate only when uncertainty changes the outcome.
Low covers tool-heavy work where results carry most of the information.
Medium buys separation of related changes and moderate ambiguity.
High buys sustained reasoning for implementation and evidence gathering.
Xhigh is reserved for review and acceptance where a missed flaw is costly.
Bias toward the stronger model or higher effort when unsure; never run whole fleets at xhigh.

## Failure handling

Effort names are model-specific; an invalid effort returns an error listing valid efforts, so re-pick from that list.
Provider allowlist errors mean the requested provider is missing from `delegate.json`; re-pick an allowed provider or report the missing policy.
`task_id` resume can hard-fail on evicted child sessions; recover by re-briefing a fresh child from the durable brief.
