# Delegation

V2's native subagent mechanism owns child lifecycle end to end; model and effort routing is expressed through explicit agent profiles, and a thin boundary adapter guards only the policy that native V2 cannot express.

## Native ownership

- The native subagent tool owns child creation, foreground and background execution, live inspection of child text, reasoning, tool calls, and permission prompts, per-child interrupt, and completion delivery to the parent.
- No local code recreates session spawning, polling, waiting, or result synthesis; the V1 delegate machinery is gone.
- Background children remain inspectable live from the parent tab and interruptible per child, through the runtime's native inspector on the pinned revision.
- Nesting depth is configured deliberately through the runtime's setting, one level unless a workflow justifies more.

## Routed profiles

- Every model and effort route the fleet uses is an explicit agent profile pinning that route; a dispatch selects a profile, and the child runs natively under it with full native inspection.
- Free-form per-call model and effort arguments are not carried over; a route that matters gets a profile instead.
- Kimi capacity fallback is two profiles of the same role differing only in provider: the direct provider is primary, and the alternate is used only when fresh headroom says the direct route is exhausted.

## Boundary policy

- The generic pre-tool hook guards native subagent dispatch: it enforces the provider allowlist, requires fresh usage headroom before a Kimi route, chooses between approved fallback profiles, admits only child profiles whose permissions satisfy the parent boundary, converts inherited asks to denies for unattended parents, and normalizes content-filter-shaped failures into ordinary error results.
- After guarding, the adapter hands off to the native mechanism unchanged; it never manages sessions, never polls or waits, and never synthesizes child output.
- Quota exhaustion fails fast with an ordinary error result; nothing sleeps or waits for a reset window, and stale, unknown, or errored headroom counts as exhausted.
- Unattended parents never let a child surface a native approval prompt, since a native ask waits for a human who is not there.
- Child profiles carry mechanically enforced restrictive permissions because native parent-permission inheritance is incomplete; the dispatch guard rejects any profile that cannot satisfy the parent's denies and directory boundary.
- The guard disappears only when native inheritance is proven to enforce the same boundary.
- Every dispatch creates a fresh child; continuity comes from re-briefing with accumulated context, and no resume mechanism is carried over unless the runtime grows a native one.

## Enforcement boundary

- The pinned runtime's generic pre-tool hook is the only local interception point; its behavior is verified against native subagent calls before V1 delegation code is removed.
- Every policy above is enforced mechanically at that boundary or by the selected profile; advisory prompt instructions never substitute for quota, provider, or unattended-permission safety.
- Loss of the hook or inability to reject an unsafe dispatch blocks landing.

## Failure behavior

- A hung child is interrupted through the native per-child interrupt; no local watchdog re-implements activity polling.
- A backend crash destroys in-flight children without a guaranteed parent error; affected work is reconciled as unknown and re-dispatched fresh.

## Acceptance

- A background child dispatched from a tab shows live progress, delivers its result to the parent, and accepts an interrupt mid-run.
- The daily TUI exposes child text, reasoning, tool calls, permission/form state, parent linkage, and interruption on the pinned revision; a reduction is a landing blocker.
- An exhausted-quota dispatch returns immediately with an error naming the attempted route.
- A child of an unattended parent cannot surface an approval prompt under any inherited rule.
