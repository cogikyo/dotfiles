---
name: redesign
description: "Use when adding a requirement that changes an existing design."
---

# Redesign

Build a new requirement in as if it was there from day one.
A requirement can make an old design decision obsolete, even when one extra flag would make the code run.

- Read the affected design and sketch what you would build with the requirement already known.
- Question an added flag or branch that preserves an obsolete design decision.
- Carry the change through types, callers, docs, examples, and rationale.
- Plan the whole change before dividing delivery into steps.
- Stop at the requirement's scope; do not turn it into an unrelated rewrite.

This rule covers a new requirement.
A fact the current design assumes needs evidence before a redesign follows from it.

See: `cutover`, `parsimony`, `doubt`.
