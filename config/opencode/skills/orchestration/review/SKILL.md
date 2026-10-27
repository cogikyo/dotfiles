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
Pick lenses from the risk map; a small diff often needs only one, and each review agent's description says what it covers.
Use scouts when ownership, governing instructions, or relevant churn would require bulk reading before choosing lenses.

Give each leaf the same baseline and constraints, one bounded concern, and a falsifying check.
Verifiers settle source, published, or browser evidence; they do not vote.
A lane that built the change is a poor judge of it; use a fresh leaf for a final verdict.

## Synthesize

Reconcile disagreement by inspecting the disputed mechanism or running a discriminating check.
Deduplicate findings by cause and consequence, reject unsupported failure paths, and keep material dissent that evidence cannot settle.
Agreement counts only as far as its evidence is independent.
Criticize your own provisional verdict when a causal leap, missing alternative, or weak acceptance argument could change confidence.

A council is parallel review leaves on one frozen brief, baseline, and acceptance question, synthesized by Collab.
Council seats do not see each other's output until all results return.
Select the strongest candidate, take compatible better mechanisms from the others, or reject every candidate.
Weigh evidence over vote counts.

## Return

Lead with the verdict, then list actionable findings by severity with location, evidence, consequence, and uncertainty.
Name the smallest credible repair and its owner, often the lane that built the change, and a falsifying check when useful.
For broad review, order remediation by dependency and consequence.
Report coverage, blocked checks, material disagreement, and residual risk.
If no actionable finding survives, say so and name what remains unverified.
