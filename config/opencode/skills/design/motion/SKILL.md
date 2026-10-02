---
name: motion
description: Use with design when building or reviewing animation, transitions, loading indicators, or recurring movement; ties motion to state, cause and effect, frequency, and reduced-motion needs.
---

# Motion

Read `design` first for the attention principle, shared test, floor, and project override.
This skill owns change over time; use `hierarchy` and `color` for static emphasis.

## Give movement a purpose

Motion should convey **state, cause and effect**, or a relationship that is hard to understand while still.
_People should see what changed and why without waiting for decoration._

- Name what each animation **explains** before adding it.
  - An opening panel can show where it came from; progress can show ongoing work.
- Ask: **how often will the user see it?**
  - Frequent actions need less movement and less delay than a rare introduction.
  - Repeated keyboard navigation should respond immediately.
- Consider **no animation** first; it is often the best animation.
  - Keep a transition only when the static version loses useful feedback or understanding.
- Keep feedback **connected to the action**.
  - Avoid delaying access to a control or completion of a routine task.

## Respect reduced motion

- Honor **`prefers-reduced-motion`** across entrances, transitions, and loops.
  - Remove non-essential movement and provide a still or reduced-motion version of needed feedback.
- Keep the resulting **state understandable** without its animation.
  - Do not make content or status depend on a reveal completing.
  - _Users who avoid movement still need to know what happened._

## Clutter tells

Apply the parent's test to these examples of **movement without change**:

- **Repeated entrances:** fade-and-slide-up on every section regardless of its role.
- **Static activity:** pulsing dots on unchanged status or blinking cursors on non-editable text.
- **Routine spectacle:** bouncing controls, floating badges, or hover movement repeated across every card.
  - Keep static status still; use an activity indicator only for real ongoing work.

## Procedure

1. Identify each trigger, the state or relationship conveyed, and the expected frequency of use.
2. Compare with a still version and apply the parent's test to what movement adds.
3. Inspect repeated use and interruption, not just a single successful playthrough.
4. Check the reduced-motion result and immediate feedback; report cases you could not observe.

Credits: [Emil Kowalski](https://emilkowal.ski/ui/you-dont-need-animations), [Anthropic frontend-design](https://github.com/anthropics/skills/tree/main/skills/frontend-design), and [Impeccable](https://impeccable.style/anti-patterns).
