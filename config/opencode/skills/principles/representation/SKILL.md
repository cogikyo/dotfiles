---
name: representation
description: "Use when shaping stateful logic, choosing core data structures, or decomposing a domain."
---

# Representation

Let the data structure carry the domain rules; carve modules late.
A domain-shaped model keeps valid states and shared rules together before control flow spreads them across phases.

- Identify valid states, forbidden combinations, and access patterns before writing branches.
- Gather domain rules in a state machine, typed model, registry, or union; revisit the shape when features add branches or booleans must stay in sync.
- Derive types from the schema that owns the shape; avoid a second handwritten authority.
- Group by domain ownership and vertical slices; check load, validate, and save modules for rules repeated across phases.
- Discover, then exploit: keep code together while the shape forms, and carve seams once contracts or conventions are real.
- Choose methods for domain concepts that own invariants and functions or pipelines for pure transforms; avoid FP or OOP ideology.

Leave a clear local shape alone when abstraction adds indirection without removing rules or invalid states.

See: `legibility`, `idiom`.
