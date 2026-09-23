---
name: scheme
description: Use for substantive design questions, competing ownership models, or migration dependencies; Collab develops evidence-backed alternatives and a bounded plan in the conversation without implementing it.
---

# Scheme

Turn provisional intent into a bounded design that can survive criticism and guide implementation.
Collab runs this procedure in the conversation and keeps planning judgment and synthesis.
Keep the working plan in the conversation.

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
Skip the multi-stage workflow when the design is already settled.
For each load-bearing conjecture, name what would falsify it.
Continue until the remaining uncertainty is harmless implementation freedom or a real decision for the user.

## Return

Lead with the recommendation and the trade-offs that survive.
State the evidence, material dissent, open decisions, and what could falsify the design.
Give the next owner enough intent, constraints, and dependencies to act without repeating the investigation.
Keep the execution steps separate from the design, and propose them as a workflow when the user wants to proceed.
Planning alone does not start implementation.

## Examples

These are shapes, not approved work; `workflow` defines the notation and default routes.
Scheme goes deeper by letting evidence and criticism change the candidates before anything is built.

### Evidence before candidates

```text
        ┌─→ 2 ─→ 3 ─┐
(1) ────┤           ├─→ (5) ─→ RETURN user
        └─→ 4 ──────┘
```

1. `self`: State the design question, fixed decisions, and what would change the answer.
2. `scout/context`: Trace every writer of the lease state across queue and worker.
3. `verify/source`: Verify the pinned primitive's atomicity from step 2's locations.
4. `scout/web`: Map prior art for lease recovery, with cited URLs.
5. `self`: Compare two or three candidates by ownership, failure behavior, and cost, and name each falsifier.

Step 3 needs step 2's locations; step 4 runs in parallel with both.

### Candidates under challenge

```text
(1) ─→ 2 ─→ (3) ─→ <3> ─→ RETURN user
       ↑            │
       └────────────┘ revise ≤1
```

1. `self`: Draft the candidates from evidence already in the conversation.
2. `[medium • anthropic/claude-opus-5-5] review/architect`, lane `critic`: Challenge ownership, coupling, and failure cases.
3. `self`: Revise; gate 3 accepts, sends one revision back to lane `critic`, or returns an open decision to the user.

Resuming lane `critic` lets it check whether its own objections were answered.
Use a fresh leaf instead when the final verdict needs independence from earlier rounds.

### Prototype when the user must feel it

```text
(1) ─→ <1> ─┬─→ 2 ─→ you: 3 ─┐
            │                ├─→ (4) ─→ RETURN user
            └────────────────┘
```

1. `self`: Narrow to the leading design; gate 1 selects step 2 only when a throwaway prototype would settle a question that argument cannot.
2. `build/general`, lane `spike`: Build the prototype under `/tmp/opencode`, outside the repository.
3. You try it and say what feels wrong.
4. `self`: Fold the result into the plan and name what it falsified.

The prototype is evidence, not implementation, and step 2 needs its own approval.
The merge is exclusive: without a prototype, step 4 works from the argument alone.
