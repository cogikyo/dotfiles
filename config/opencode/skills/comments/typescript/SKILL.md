---
name: typescript
description: Use with comments when auditing or editing TypeScript doc comments or TSDoc on exported surfaces.
---

# TypeScript comments

This skill adds TypeScript conventions to `comments`, which owns the general procedure.

- Use TSDoc `/** */` blocks for exported functions, types, classes, and object or array values that need documentation.
- Give exported scalar constants a right-side `//` comment instead, per `comments`.
- Add `@param`, `@returns`, `@throws`, or `@example` only when names and types leave the contract unclear, and do not restate the type signature in prose.
- Mark deprecation with `@deprecated` and name the replacement.
- Link related symbols with `{@link Name}` when the reference helps the reader.
- Prefer the project's existing TSDoc or JSDoc conventions and lint rules over new tags.
