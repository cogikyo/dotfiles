---
name: scheme
description: Use for substantive design questions, competing ownership models, migration dependencies, or authoring and revising governing specs; develops evidence-backed alternatives and a bounded design contract without implementing it.
---

# Scheme

Turn provisional intent into a bounded design that can survive criticism and guide implementation.
Collab uses this procedure in the attended conversation; Orchestrator uses it for an approved planning objective.
The current owner retains planning judgment and synthesis.
Follow its approval, routing, permission, and continuity rules rather than starting another approval layer.

## Establish the design question

Identify the desired outcome, current behavior, governing inputs, fixed decisions, exclusions, and consequential unknowns.
Separate missing evidence from decisions that require the user and inspect only within the planning boundary.
Existing code or an old spec does not prove that its architecture remains correct.
Explore credible alternatives when they change ownership, behavior, cost, or risk.
Prefer the smallest truthful design, including deletion when a concept has no remaining purpose.
Keep reversible implementation mechanics open unless a governing constraint depends on them.

## Investigate and criticize

Inspect directly when the question fits the current context.
When delegation is approved, assign bounded scouts or verifiers to consequential evidence gaps.
Use specialist criticism for a real uncertainty or independent challenge, not a mandatory planning stage.
Do not manufacture a multi-stage workflow for a settled design.
One owner reconciles evidence, assumptions, alternatives, and material dissent into a recommendation.
For each load-bearing conjecture, identify what would falsify it.
Continue until uncertainty is harmless implementation freedom or a genuine decision for Collab.
Planning alone never authorizes implementation or repair of discovered problems.

## Plans and specs

An ephemeral plan is the default when no artifact was requested.
Return enough intent, constraints, dependencies, evidence, and open decisions for the next owner to act without reconstructing the investigation.
Keep execution decomposition separate from the durable design contract.

A spec governs one coherent concern in its nearest owning `.spec/` directory.
It states intended behavior, ownership, boundaries, invariants, consequential decisions, and settled trade-offs, leaving capable builders room for local mechanics.
Collab authors approved specs in the attended session.

> [!IMPORTANT] Artifact authority is explicit
>
> Orchestrator returns spec-ready text by default and writes planning artifacts only when its brief grants write authority for their exact paths.
> Neither owner uses planning permission to edit production code or unrelated documentation.

### Artifact shape

Start with what exists when the concern is implemented and why.
Name sections after the domain's real parts, nesting only where the domain needs it.
Use declarative present-tense intent, common words, consistent terms, and one main claim per sentence.
Put one sentence on each source line and keep related lines together as a paragraph.

- Include acceptance conditions that define behavior, protect material risk, or constrain design.
- Omit file inventories, function shapes, commands, check matrices, and edit sequences unless they are part of the contract.
- Avoid facts a builder can cheaply discover from the live tree.
- Use examples, tables, or diagrams only when they clarify a necessary relationship.

Keep each spec self-contained and cheap to rewrite.
Split when ownership or design concerns separate, not because of length.
Do not add progress logs, completed-slice lists, handoffs, check transcripts, or Git state.
Git owns history; todos and the tree own execution state.

### Lifecycle

1. Establish one concern and its governing design.
2. Harden it through evidence and criticism.
3. Have Collab name it as input to an approved implementation boundary.
4. Delete it after its contract passes and the approved scope includes removal.

Genuine remaining design belongs in a successor spec, not a journal attached to the spent contract.
Only Collab calls `spec_title`, after a real governing packet is active, with its path and exactly four ALL-CAPS words totaling at most 28 characters.
Spec commits return to Collab and require separate Git approval.

## Return

Lead with the recommendation and surviving trade-offs.
State evidence, material dissent, unresolved decisions, and what could falsify the design.
Report authorized artifact changes and the next decision or implementation boundary.
Do not begin implementation merely because planning is complete.

## Extended example: recoverable queue lease ownership

Assume Collab approves planning for duplicate execution after expiry and unclear ownership during shutdown.
The input fixes the repository, worktree, branch, baseline, current spec, sanitized traces, pinned dependency, and affected queue/worker/API boundaries.
Tenant isolation and the public request shape must remain unchanged; changing either requires Collab's decision.
The design question is whether renewal, fencing, and recovery can remain with the queue owner or need a separate coordinator.
Candidate contracts are transient reasoning products, never competing implementations.

### Planning envelope

One `[high • openai/gpt-5.6-sol-fast] orchestrator` owns synthesis with `authority: "read-only"` and `unattended: true` by default.
Every leaf receives both fields in task and brief, with at most three concurrent leaves and no fallback routes.
Evidence is limited to the named source, spec, traces, and already-cached pinned dependency.
No builds, tests, benchmarks, installs, acquisition, production repairs, or Git mutations are permitted.
Proposed experiments require separate execution approval.

The only artifact variation is explicit upfront write authority for `.spec/queue.md` on this Sol Fast owner and step 11's scribe.
All evidence and criticism leaves stay read-only.
Without that path-specific authority, step 11 is skipped and the owner returns spec-ready text.

### Deep scouting and candidate contracts

Owner: Orchestrator, using `scheme` for synthesis and `review` when assessing criticism.

```text
        ┌─→ 2 ─→ 5 ─┐
(1) ────┼─→ 3 ──────┼─→ (6) ─→ <6>
        └─→ 4 ──────┘
```

1. `self`: Define the consequential choice and fixed constraints.
   - Distinguish observed failures, disputed assumptions, and decisions that must return to Collab.
2. `[xhigh • openai/gpt-5.6-luna-fast] scout/context`: Map authoritative lease state.
   - Trace all acquire, renew, fence, and recovery writers across queue and worker boundaries, identifying the dependency primitive.
3. `[xhigh • openai/gpt-5.6-luna-fast] scout/context`: Trace lifetime and failure transitions.
   - Establish where cancellation, shutdown, lost renewal, or crash can leave work alive after ownership expires.
4. `[xhigh • openai/gpt-5.6-luna-fast] scout/context`: Trace public and tenant constraints.
   - Identify source-backed retry, identity, and error behavior without treating undocumented behavior as fixed intent.
5. `[high • xai/grok-4.7] verify/source`: Verify the available primitive.
   - Establish pinned atomicity and fencing guarantees from cached source, with exact refs and version limits.
6. `self`: Form and compare candidate contracts.
   - Use all evidence to compare an in-place queue owner, a separate coordinator, and minimal correction by invariants, failure behavior, migration cost, and falsifiers.

Step 5 follows step 2 while the other evidence branches can continue, keeping the maximum at three leaves.
Gate 6 returns a missing fixed decision before designing around it.
Otherwise it selects the next phase's disputed-evidence and criticism routes from the actual candidate risks.

### Resolve the claims that control the choice

This phase consumes gate 6's candidate packet; its two optional branches are independent.
The upper branch verifies a disputed external claim, while the lower challenges a consequential design assumption.
Each branch bypasses its optional leaf when that need is absent; step 9 waits for both selected routes, never for a skipped leaf.

```text
      ┌─┬─→ 7 ─┬─┐
<6> ──┤ └──────┘ ├─→ (9) ─→ <9>
      └─┬─→ 8 ─┬─┘
        └──────┘
```

7. If a disputed primitive controls the choice, `[high • xai/grok-4.7] verify/source`: Settle that guarantee.
   - Answer one discriminating source question without acquiring source or changing dependency versions.
8. If recovery ownership has a material unresolved risk, `[medium • anthropic/claude-opus-5] review/architect`: Challenge the candidates.
   - Assess authoritative state, coupling, tenant-boundary assumptions, and concrete expiry/shutdown counterexamples against the frozen candidate packet, without writing a replacement plan.
9. `self`: Revise and judge the selected design.
   - Integrate the selected evidence and criticism, retain superior mechanisms from rejected candidates, and settle migration order, rollback constraints, and behavior-level acceptance or reject all candidates.

One substantive critic can cover the coupled ownership question across files; this does not require a panel or a mandatory spec-readiness critic.
If there is no material uncertainty, both leaves are skipped and the owner evaluates the candidates directly.
For the illustrated failure case, step 8 is justified by the risk that moving renewal leaves two apparent authorities during recovery.

Gate 9 permits one revision pass through a fresh step 8 child only when its objection is repairable within the same constraints and available evidence.
This loop detail shows the same steps after initial synthesis:

```text
8 ─→ (9) ─→ <9>
↑            │
└────────────┘
```

The back edge targets step 8, not the branch junction: it does not rerun unrelated source verification or wait for the original fan-out again.
Step 9 supplies the revised candidate and disputed invariant before that child starts.
Retain step 7's source evidence only while the revised candidate uses the same verified guarantee; changed or missing evidence returns to Collab because this loop authorizes renewed criticism only.
After the single pass, unresolved criticism remains a blocker or named uncertainty, never an implicit pass.
New evidence needs, changed ownership beyond the brief, rejected candidates, exhausted revision, and denied operations return to Collab instead of expanding the loop.

### Final contract or transient result

This phase starts only after gate 9 accepts a coherent design.
Blocked or rejected designs return directly from that gate without an artifact write.

```text
<9> ─→ (10) ─┬─→ 11 ─┐
             │       ├─→ (12) ─→ RETURN Collab
             └───────┘
```

10. `self`: State the settled contract in spec-ready form.
    - Explain authoritative ownership, fencing invariants, cancellation, and the recovery trade-off, keeping execution commands and check matrices in a separate proposal.
11. Only with approved artifact authority, `[high • xai/grok-4.7] build/scribe`: Write `.spec/queue.md` with `prose` and `prose-docs`.
    - Preserve the settled design and dissent, using source comparison, Markdown inspection, and `git diff --check` without production edits or an execution journal.
12. `self`: Validate fidelity and return the planning result.
    - Return the recommendation, evidence, rejected alternatives, falsifiers, unresolved decisions, and authorized artifact delta without starting implementation.

The merge before step 12 takes the artifact route or the transient-text route, never both.
Artifact drift has no repair loop in this contract and returns a precise correction request to Collab.
A rejected coordinator can still contribute a useful recovery invariant; unresolved clock behavior must remain explicit.
Completion authorizes neither implementation, a spec commit, nor deletion of a governing artifact.
