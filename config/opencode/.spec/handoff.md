# Mode handoff

One persistent session carries a concern across primary modes through an explicit handoff protocol: the seated mode yields with a request, an orchestrator session validates it and reseats the same session under the target mode, and accumulated session context survives the transition.
This replaces child-session fanout for phase transitions of one continuous concern, and it is the substrate for orchestrating many persistent sessions.
It builds only on V2's native per-prompt agent surface, lands after cutover, and is never a migration gate.

## Handoff request

- A seated primary mode requests a transition through one tool call declaring the target mode, the objective for the next seat, the reason for yielding, and an observable done-when boundary.
- The request ends the worker's claim to the seat; the tool returns nothing actionable and the worker never prompts its own session or mutates orchestrator state.
- Target is always a primary mode; leaves are dispatched, never seated.
- Any primary mode is a valid target, Drive included; an orchestrated session may run unattended legs under a Drive seat.
- A request without a genuine phase change is a smell; the seated mode keeps working instead of ceremonially bouncing.

## Orchestrator

- The orchestrator is itself an OpenCode session, human-driven, whose work product is the seats and objectives of the sessions it manages.
- It owns seat state for every managed session and is the only automated writer of mode transitions.
- On a handoff it validates the transition against an allowed-transition set and a per-session loop budget, then prompts the same session under the target mode with the declared objective.
- The next turn runs with the target mode's full prompt and permission envelope; nothing about the previous seat's permissions leaks forward.
- Transition history per session is observable, so pathological mode bouncing is a visible fact rather than a silent loop.
- It schedules across sessions like a process scheduler: workers yield, it decides what runs next and with what priority.
- Managed sessions span a spectrum of attention: orchestrator-driven, human-steered mid-flight, or fully independent once their objective and transition set are stable.

## Seat semantics

- A session holds exactly one seated mode at a time; the seat defines authorship boundaries, so a Scheme seat writes specs and a Collab or Drive seat writes code, within the session's lifetime.
- Session context is the asset the protocol exists to preserve; a transition never compacts, forks, or re-briefs where the native seat change suffices.
- Seated modes still dispatch leaves and mode children under the delegation contract; handoff and delegation compose rather than compete.
- Handoff is for sequential phases of one concern; delegation is for bounded disjoint concerns; choosing handoff where delegation fits inflates one session past usefulness.

## Human preemption

- Human attention always outranks the orchestrator: a human prompt or manual mode switch on a managed session parks automation for that session.
- A parked session resumes under orchestration only by explicit human release.
- The human's manual seat choice and the orchestrator's seat record never diverge silently; the orchestrator observes the actual seat before every transition it issues.

## Failure behavior

- An invalid target or disallowed transition returns an ordinary error result to the seated worker, which continues in its current seat.
- An exhausted loop budget parks the session with its transition history for human review rather than granting one more hop.
- Orchestrator death leaves managed sessions parked and fully usable through normal attended tabs; no session is held hostage by its scheduler.
- A handoff request that the orchestrator never picks up is visible as a parked session, never a silent stall retried by the worker.

## Acceptance

- One session moves Scheme to Collab to Review on the pinned V2 revision with context intact, each seat operating under its own permissions.
- A disallowed transition and an exhausted loop budget both park observably instead of looping.
- A human mid-run prompt on a managed session parks automation and wins the seat.

## Next actions (residue only)

- The handoff transport is undecided: how a yield reaches the orchestrator session and how its reseat prompt is issued get hammered out against the pinned V2 surface when this spec is revisited.
