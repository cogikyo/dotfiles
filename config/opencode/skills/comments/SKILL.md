---
name: comments
description: Use when auditing or editing code comments, doc comments, or structural banners; supplies source-fidelity and deletion judgment plus relevant language and banner subskills.
---

# Comments

This procedure extends the global comment and prose rules for any owner with an approved comment assignment.
Loading it does not grant write authority or permission to change behavior.

## Compose

Read the relevant subskills before editing:

- `banners` (`comments/banners/SKILL.md`) for structural headers, subsection labels, or external-document blocks.
- `go` (`comments/go/SKILL.md`) for Go doc comments and package documentation.
- `typescript` (`comments/typescript/SKILL.md`) for TypeScript doc comments and TSDoc.

Combine language and banner guidance when both apply.
Load `adhd` (`prose/adhd/SKILL.md`) for wording: lead with the point and keep only what the reader needs.
Keep comment-only, banner-only, and combined scopes distinct; unclear audit versus update intent needs a decision before mutation.

## Audit and edit

Read the declaration, implementation, relevant callers, and any external contract needed to verify the comment.
Target drift first, then delete redundant narration and noise before rewriting.
Preserve useful local voice without treating stale comments as authority, and avoid wording churn without a concrete gain.
If a structural fix would remove the need for a comment, recommend that fix instead of hiding the problem in prose.

Match detail to the reader's dependency on the code.
Public surfaces need what callers cannot infer from names and types, such as a surprising error or edge case.
Core entities and cross-system seams need non-obvious ownership, invariants, ordering, and boundary assumptions.
Private helpers and idiomatic glue rarely need more than a surprising constraint; none of these roles requires a comment template.

## Length

A comment says what the code does and why it matters, not how it does it; the code already shows how.
Default to one sentence and stop at two.
Merge related short facts into one sentence instead of stacking a line per fact.
Leave branch-by-branch behavior and edge cases to the code unless a caller would get them wrong without the comment.
Longer doc comments are fine only for tags that carry separate facts, such as `@example`, `@throws`, or `@param`.

Keep doc comments attached to their declarations.
Prefer a right-side comment for scalar constants, variables, fields, and individual entries when it fits the line width.
Keep consecutive side-commented declarations together without blank lines between them.
Use a leading doc comment for functions, types, classes, and object or array values, or when the note needs more than one line.
Delete member comments that only repeat the name.
In Bash and config, explain cryptic expansion, traps, quoting, external formats, non-obvious values, or cross-file coupling rather than straightforward assignments.

Use `TODO` only for a concrete unfinished action that cannot be completed in scope, and `NOTE` for durable surprising context.
Return a decision to the owner before adding `FIXME` or `HACK`, which assert debt or intentional compromise.
Do not add a marker when the in-scope fix is available.

Before returning, compare comments to source again and inspect the diff for behavior changes, accidental detachment, and unrelated churn.
Preserve compiler directives, lint controls, generated-file markers, and other machine-significant comments unless their change is explicitly authorized.
Report source ambiguity, deletion candidates, structural recommendations, and any unverified contract.
