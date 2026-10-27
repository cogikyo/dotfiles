---
name: scheme
description: Use for substantive design questions, competing ownership models, or migration dependencies; Collab develops evidence-backed alternatives and a bounded plan in the conversation without implementing it.
---

# Scheme

Turn provisional intent into a bounded design that can survive criticism and guide implementation.
Collab runs this procedure in the conversation and keeps the working plan, planning judgment, and synthesis there.

Apply `redesign` when a requirement changes the existing design; apply `representation` before comparing domain models or boundaries.

## Establish the design question

Identify the desired outcome, current behavior, governing inputs, fixed decisions, exclusions, and consequential unknowns.
Separate missing evidence from decisions that need the user.
Existing code does not prove that its architecture is still correct.
Explore credible alternatives when they change ownership, behavior, cost, or risk.
Prefer the smallest truthful design, including deletion when a concept has no remaining purpose.
Leave reversible implementation mechanics open unless a constraint depends on them.

## Investigate and criticize

Inspect directly when the question fits the current context.
Send a scout or verifier for a consequential evidence gap that would crowd the conversation.
Use a `review/*` leaf for a real uncertainty or an independent challenge, usually `review/critic` or `review/architect`.
Resume the critic lane to check whether Collab's revisions answered its objections; use a fresh leaf when the verdict needs independence from earlier rounds.
Skip the multi-stage workflow when the design is already settled.
For each load-bearing conjecture, name what would falsify it.
Revise candidates when evidence or criticism changes their premise.
Build a throwaway prototype under `/tmp/opencode`, outside the repository, only when argument cannot settle the question.
Get separate approval for the prototype; missing approval returns a blocker.
Treat the user's prototype trial as evidence, not implementation, and name what it falsified.
Continue until the remaining uncertainty is harmless implementation freedom or a real decision for the user.

## Return

Lead with the recommendation and the trade-offs that survive.
State the evidence, material dissent, open decisions, and what could falsify the design.
Give the next owner enough intent, constraints, and dependencies to act without repeating the investigation.
Keep the execution steps separate from the design, and propose them as a workflow when the user wants to proceed.
