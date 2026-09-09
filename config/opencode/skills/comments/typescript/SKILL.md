---
name: typescript
description: Use with comments when auditing or editing TypeScript doc comments or TSDoc on exported surfaces.
---

# TypeScript comments

Read `comments` for the general procedure.
Use TSDoc `/** */` comments for exported surfaces that need documentation.
Add tags only when names and types do not convey the needed contract, and do not restate a type signature in prose.

Keep documentation attached to the declaration and verify claims against its implementation and callers.
Explain non-obvious effects, errors, ownership, or ordering only where source supports them and the reader needs them.
Follow the project's supported TSDoc conventions rather than introducing a new tag inventory.
