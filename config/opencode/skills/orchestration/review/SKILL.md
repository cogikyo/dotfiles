---
name: review
description: Use to judge code changes, plans, specs, incident hypotheses, or systems; load for cross-cutting risk investigation, disputed evidence, or independent/council synthesis without implementing fixes.
---

# Review

Inspect the target for consequential errors and report criticisms that survive evidence gathering.
This procedure supports focused explanation, ordinary review, broad investigation, and council judgment.
A specialist lens is optional; general review covers cross-cutting problems that do not fit one leaf.
Follow the current owner's approved scope and routing rules.
Collab can review in place or delegate an independent context.
An Orchestrator review uses `authority: "read-only"` throughout its descendants; loading the skill neither changes permissions nor authorizes checks.

## Frame and inspect

Identify the target, baseline, governing claims, exclusions, and likely failure consequences.
Follow actual risk rather than a fixed checklist.
Look for unsupported assumptions, invalid state transitions, hidden coupling, unsafe boundaries, partial failures, and unverifiable acceptance claims.
Separate observation, inference, and conjecture, then trace reachability and consequence before assigning severity.
Inspect source or request one discriminating piece of evidence when it could change the verdict.
Do not manufacture findings or broaden scope because another lens exists.

> [!IMPORTANT] Judgment stays read-only
>
> No patches, implementation, commits, generated artifacts, or delegated writes.
> Read-only API and shell inspection remain subject to the permission envelope; builds, suites, generators, benchmarks, and other expensive checks need explicit approval.

Recommend repairs without applying them.

## Use independent contexts deliberately

Review directly when one coherent pass can settle the important claims.
When approved, delegate orthogonal risks, independent criticism, or evidence gathering that would crowd synthesis.
Choose lenses from the risk map, not the file list:

- `review/debug`: correctness, state, concurrency, parsing, and root cause.
- `review/security`: credible adversarial paths and trust boundaries.
- `review/architect`: ownership, coupling, system shape, and conceptual truth.
- `review/critic`: assumptions, alternatives, plans, and acceptance criteria.
- `review/simplify` and `review/modernize`: accidental complexity or obsolete mechanisms.
- `review/profile` and `review/test`: evidenced performance risk or test-system maintenance cost.
- `review/design`: product intent, visual language, UX, and interaction design.

Give each leaf the same relevant baseline and constraints, then one bounded concern and falsifying check.
Verifiers settle source, published, or browser evidence; they do not cast votes.
For explicitly requested live X/Twitter signal, load `x` and run Grok CLI in the current owner.
A fresh Astra context can provide independence without model diversity.
Choose another model when different failure patterns or lower cost justify it, without adding a manager that only repeats reports.

## Synthesize

One owner reconciles disagreement by inspecting the disputed mechanism or commissioning an approved discriminating check.
Deduplicate findings by cause and consequence, reject unsupported failure paths, and preserve material dissent that evidence cannot settle.
Agreement matters only to the extent that its evidence is independent.
Criticize a provisional verdict when a causal leap, missing alternative, or weak acceptance argument could change confidence.
That criticism can be the owner's direct work; it does not require a critic child or a second synthesis stage.

For a council, follow Collab's frozen brief and baseline.
Participants stay isolated until all results return; a judge receives every candidate, checks, decisions, and dissent before choosing a base.
Select the strongest candidate, identify compatible superior mechanisms from others, or reject every candidate.
Do not count votes, edit the winner, or delegate integration from a read-only judgment boundary.

## Return

Lead with the answer or verdict and list actionable findings by severity with source location, evidence, consequence, and uncertainty.
Name the smallest credible repair or next owner and a falsifying check when useful.
For broad review, order remediation by dependency and consequence.
Report inspected coverage, blocked checks, material disagreement, and residual risk.
If no actionable finding survives, say so and identify what remains unverified.
An Orchestrator returns missing decisions as `Questions for Collab`; review completion never grants implementation permission.

## Extended example: duplicate jobs and stalled shutdown

Assume Collab approves a wide review after duplicate jobs, stalled shutdown, and inconsistent API errors are reported.
The packet fixes the repository, worktree, branch, comparison commits, spec, pinned dependency, and sanitized incident logs.
Scope includes `internal/queue/**`, `internal/worker/**`, `internal/api/**`, and `docs/queue.md`, excluding implementation, production access, and incident response actions.
Symptoms are questions to explain, not established causes.

### Contract and routes

One `[high • openai/gpt-5.6-sol-fast] orchestrator` owns broad judgment with `review`, using `authority: "read-only"` and `unattended: true`.
Every child receives both fields in its task and brief, with at most three concurrent leaves and no fallback routes.
Only supplied local artifacts, the available pinned source cache, and read-only source comparison are approved.
No tests, builds, benchmarks, generators, acquisition, service access, or artifact writes are included.
Return a desired reproduction command as a proposed check rather than running it.

The example uses focused `[xhigh • openai/gpt-5.6-luna-fast]` scouts, `[high • xai/grok-4.6]` verifiers and `scout/context` for the bounded chronology, `[medium • anthropic/claude-opus-5] review/debug`, and `[high • xai/grok-4.6] review/security`.
These routes follow the owner's model contract rather than defining a second routing policy.
No premium substitution is included; the high-level council variation below requires a separate explicit request.

### Evidence: follow mechanisms across files

Owner: Orchestrator.

```text
        ┌─→ 2 ─→ 5 ─┐
(1) ────┼─→ 3 ──────┼─→ (6)
        └─→ 4 ──────┘
```

1. `self`: Frame competing explanations and their falsifiers.
   - Separate renewal failure, stale ownership, retry amplification, and unrelated shutdown delay.
2. Luna `scout/context`: Trace the lease lifetime.
   - Follow acquire, renew, expire, and fence paths across queue and worker callers, identifying the exact upstream primitive.
3. Luna `scout/context`: Trace retry and cancellation reachability.
   - Follow API errors through server retry and worker shutdown, separating reachable paths from assumed client behavior.
4. Grok `scout/context`: Reconstruct the supplied incident chronology.
   - Tie observed events to timestamps and job identities, explicitly recording gaps without inferring causality.
5. Grok `verify/source`: Verify the pinned renewal guarantee.
   - Inspect atomicity, clock use, and cancellation semantics in cached source, retaining exact ref and limitations.
6. `self`: Form the cross-cutting risk map.
   - Reconcile all required reports into reachable mechanisms, separating source facts, observed events, and unresolved causal claims.

Step 5 depends on step 2 and can overlap steps 3 and 4 without exceeding three leaves.
The join waits for all evidence branches; an unavailable claim stays unresolved rather than being treated as proof.

### Independent concerns, then synthesis

This phase consumes step 6's frozen evidence and risk map.
For this incident, both stale-ownership interleavings and tenant replay are credible enough to justify independent judgment.
A simpler review with no such independent risks stays with the owner.

```text
      ┌─→ 7 ─┐
(6) ──┤      ├─→ (9) ─→ <9> ─┬─→ 10 ─┐
      └─→ 8 ─┘               │       ├─→ (11) ─→ RETURN Collab
                             └───────┘
```

7. Opus `review/debug`: Challenge stale ownership and shutdown causality.
   - Give concrete interleavings or falsification across the affected boundaries, not one review per directory.
8. Grok `review/security`: Challenge tenant identity and replay assumptions.
   - Report credible cross-tenant or replay paths with prerequisites supported by the frozen packet.
9. `self`: Synthesize findings and criticize the provisional verdict.
   - Gate 9 requires both independent reports, deduplicates causes, reinspects disputed local paths, and selects step 10 only for a material upstream disagreement.
10. Grok `verify/source`: Resolve the one disputed guarantee.
    - Return a discriminating answer from the same pinned version without acquisition or version substitution.
11. `self`: Return the evidence-backed judgment.
    - Incorporate the selected evidence, downgrade unsupported severity, and state findings, remediation dependencies, dissent, coverage, and smallest proposed falsifiers.

Steps 7 and 8 cannot see each other's reports before returning.
Gate 9's lower route skips step 10 when no material upstream dispute remains; the merge before 11 takes only the selected route.
The additional source investigation is bounded to one request, not a recursive evidence hunt.
The owner directly challenges its inference at step 9 because confusing chronology with causality would change the verdict.
There is no mandatory `review/critic` after that work.
If step 10 cannot settle the claim, step 11 returns qualified judgment or an evidence blocker rather than commissioning more agents.
Denied permissions and out-of-scope evidence needs return immediately; a blocked check can prevent a verdict but cannot authorize implementation.

### Council variation

For explicitly requested whole-review comparisons, Collab may dispatch sibling read-only Orchestrators from one frozen brief.
An ordinary approved council uses `[high • openai/gpt-5.6-sol-fast]`, `[medium • anthropic/claude-opus-5]`, and `[high • xai/grok-4.6]`, each with the same permitted Luna evidence routes and no further judgment leaves.
They are Collab's children, never children of the investigation owner, and return before seeing sibling output.

Owner: Collab; all three participants have `authority: "read-only"` and `unattended: true`.

```text
        ┌─→ {2} ─┐
(1) ────┼─→ {3} ─┼─→ (5) ─→ RETURN user
        └─→ {4} ─┘
```

1. `self`: Freeze the whole-review question, baseline, and evidence-only child routes.
2. Sol Fast `orchestrator`: Explain the cross-boundary failure mechanisms with `review` and return supported findings.
3. Opus `orchestrator`: Independently review the same complete contract and return transient judgment only.
4. Grok `orchestrator`: Independently review the same complete contract and return supported findings.
5. `self`: Reconcile all three reports against disputed evidence, retaining supported minority findings.

Collab then uses `review` to inspect disputed evidence and reconcile every candidate.
Different internal teams must be reported as team comparisons rather than isolated model performance.
The council adds neither implementation authority nor an automatic extra judge, and cannot erase a supported minority finding.

### Explicit high-level council

For a user-requested high-level comparison of recovery architectures, replace the ordinary council with this pair.
Owner: Collab; both Orchestrators and their permitted Luna evidence leaves remain read-only and unattended, with the same frozen brief, evidence limits, and no writes or expensive checks.
No Opus participates anywhere in this workflow, and Fable never owns durable writing.

```text
        ┌─→ {2} ─┐
(1) ────┤        ├─→ (4) ─→ RETURN user
        └─→ {3} ─┘
```

1. `self`: Freeze the coupled recovery-architecture question and record the explicit Fable request.
2. `[high • openai/gpt-6-astra] orchestrator`: Judge the whole recovery architecture, reserving high effort for its interacting failure invariants.
3. `[high • anthropic/claude-fable-5-1] orchestrator`: Independently judge that same architecture and return transient synthesis only.
4. `self`: Inspect both returned arguments and reconcile the strongest supported mechanisms without votes or another judge.

Participants cannot inspect sibling output before returning; a missing prerequisite returns a qualified judgment or blocker rather than expanding either team.
