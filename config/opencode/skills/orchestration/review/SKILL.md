---
name: review
description: Use to judge code changes, plans, incident hypotheses, or systems; Collab reviews in-session or synthesizes independent review leaves and councils without implementing fixes.
---

# Review

Inspect the target for consequential errors and report criticisms that survive evidence gathering.
Collab runs this procedure in-session for focused review, broad investigation, and council synthesis.
A specialist lens is optional; general review covers cross-cutting problems that do not fit one leaf.

## Frame and inspect

Identify the target, baseline, governing claims, exclusions, and likely failure consequences.
Follow actual risk instead of a fixed checklist.
Look for unsupported assumptions, invalid state transitions, hidden coupling, unsafe boundaries, partial failures, and acceptance claims that nobody can check.
Separate observation, inference, and conjecture, then trace reachability and consequence before you assign severity.
Inspect source or request one discriminating piece of evidence when it could change the verdict.
Report only findings the evidence supports, and keep scope where the target sets it.
Recommend repairs without applying them during the review.

## Use independent contexts

Review directly when one coherent pass can settle the important claims.
Send review leaves for orthogonal risks, independence from the implementation context, or evidence that would crowd synthesis.
Choose lenses from the risk map, not the file list:

- `review/debug`: correctness, state, concurrency, parsing, and root cause.
- `review/security`: credible adversarial paths and trust boundaries.
- `review/architect`: ownership, coupling, system shape, and conceptual truth.
- `review/critic`: assumptions, alternatives, plans, and acceptance criteria.
- `review/simplify`: accidental complexity, dead code, and obsolete mechanisms.
- `review/profile`: evidenced performance risk.
- `review/design`: product intent, visual language, UX, and interaction design.

Give each leaf the same baseline and constraints, one bounded concern, and a falsifying check.
Verifiers settle source, published, or browser evidence; they do not vote.
A lane that built the change is a poor judge of it; use a fresh leaf for a final verdict.

## Synthesize

Reconcile disagreement by inspecting the disputed mechanism or running an approved discriminating check.
Deduplicate findings by cause and consequence, reject unsupported failure paths, and keep material dissent that evidence cannot settle.
Agreement counts only as far as its evidence is independent.
Criticize your own provisional verdict when a causal leap, missing alternative, or weak acceptance argument could change confidence.

A council is parallel review leaves on one frozen brief and baseline, synthesized by Collab.
Participants stay isolated until all results return.
Select the strongest candidate, take compatible better mechanisms from the others, or reject every candidate.
Weigh evidence over vote counts.

## Return

Lead with the verdict, then list actionable findings by severity with location, evidence, consequence, and uncertainty.
Name the smallest credible repair and its owner, often the lane that built the change, and a falsifying check when useful.
For broad review, order remediation by dependency and consequence.
Report coverage, blocked checks, material disagreement, and residual risk.
If no actionable finding survives, say so and name what remains unverified.

## Examples

These are shapes, not approved work; `workflow` defines the notation and default routes.
Review widens by adding independent lenses, not by adding stages.

### Change review with parallel lenses

```text
        ┌─→ 2 ─┐
(1) ────┼─→ 3 ─┼─→ (5) ─→ RETURN user
        └─→ 4 ─┘
```

1. `self`: Frame the diff, baseline, and the claims the change makes.
2. `[medium • anthropic/claude-opus-5-5] review/debug`: Check state transitions and partial failures.
3. `[high • openai/gpt-6-sol] review/security`: Check the new input boundary for a credible exploit path.
4. `[high • openai/gpt-6-sol] review/simplify`: Check for accidental complexity and dead paths.
5. `self`: Deduplicate by cause, inspect any disputed mechanism, and return the verdict.

Pick lenses from the risk map; a small diff often needs only one.

### System audit from a risk map

```text
        ┌─→ 2 ─┐           ┌─→ 5 ─┐
(1) ────┤      ├─→ (4) ────┤      ├─→ (7) ─→ RETURN user
        └─→ 3 ─┘           └─→ 6 ─┘
```

1. `self`: Name the subsystem, the failure you care about, and exclusions.
2. `scout/context`: Map ownership, entry points, and governing instructions.
3. `scout/dirty`: Report recent churn and WIP that touch the subsystem.
4. `self`: Build the risk map and choose lenses from it.
5. `[medium • anthropic/claude-opus-5-5] review/architect`: Judge ownership and hidden coupling.
6. `[high • openai/gpt-6-sol] review/debug`: Trace the highest-risk failure path end to end.
7. `self`: Synthesize and order remediation by dependency and consequence.

The scouts keep bulk reading out of the session, so step 4 reasons over short reports.

### Council with a disputed claim

```text
        ┌─→ 2 ─┐
(1) ────┼─→ 3 ─┼─→ (5) ─→ <5> ─┬─→ 6 ─┐
        └─→ 4 ─┘               │      ├─→ (7) ─→ RETURN user
                               └──────┘
```

1. `self`: Freeze one brief, baseline, and acceptance question.
2. `[medium • anthropic/claude-opus-5-5] review/critic`: Critique the plan.
3. `[high • openai/gpt-6-sol] review/critic`: Critique the same plan.
4. `[high • openai/gpt-6-astra] review/critic`: Critique the same plan.
5. `self`: Compare candidates; gate 5 selects step 6 only when a disputed claim decides the verdict.
6. `verify/source`: Settle that one claim against source.
7. `self`: Keep the strongest critique, merge compatible points, and preserve material dissent.

The merge is exclusive: step 7 never waits for a skipped step 6.
Different models make this a comparison of teams, and agreement counts only as far as the evidence is independent.
