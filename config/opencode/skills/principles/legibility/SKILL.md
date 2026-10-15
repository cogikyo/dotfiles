---
name: legibility
description: "Use when naming packages, files, or identifiers, tracing confusing code, or adding layers, interfaces, or mutable state."
---

# Legibility

Count what a reader must chase and hold; cut what costs more than it saves.
Layers to trace and state to hold are independent costs; reducing one can leave the other untouched.
Large files, high child counts, and deep nesting are reasons to inspect those costs.

- Let paths, packages, files, receivers, and modules carry the namespace; do not repeat their context in names.
- Keep core or local names short, generic names stable and widely understood, and edge names specific; a name of 3+ words is a smell unless it is a real compound noun.
- Avoid `utils`, `shared`, or `helpers` as owners unless they group clearly owned children; name one-off values only for domain meaning, reuse, config, validation, or rendering structure.
- Read top-down with early returns; keep behavior local until a real concern needs separation.
- Let a large function find its working shape, then decompose for readability, testability, or reuse; extract one-off helpers only to flatten complex nesting or clarify ownership.
- Collapse one-caller wrappers and pass-through layers unless they hide meaningful decisions or ownership.
- Expose hidden coupling or move it to its owner: call-order rituals, shared mutation, meaningful strings, leaked edge shapes, and steering flags.
- Check that a new reader can find a value's origin and every writer in 30 seconds; stop adding layers or state when they cost more effort than they remove.

Weigh these counts as pressure points; a tripped count alone is not a finding.
About 6 visible concepts in one scene calls for chunking, splitting, renaming, or reframing.
About 3 layers of variation usually signals a missing axis, boundary, or domain concept.
A directory with fewer than 3 meaningful children often wants to be flatter; one with more than 6 wants grouping or stronger names.

See: `ownership`, `membrane`.
