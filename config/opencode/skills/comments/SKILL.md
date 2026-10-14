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
For wording, lead with the point and keep only what the reader needs.
Keep comment-only, banner-only, and combined scopes distinct; unclear audit versus update intent needs a decision before mutation.

## Default

No comment is the default.
Add or keep one only where a competent reader would otherwise form a wrong belief, such as about a sign, unit, source, scope, surprising edge case, or the reason behind odd code.
These examples say what can qualify; do not treat them, or a brief's list of them, as a checklist of facts to document.
Match the comment density of the surrounding file and repository, so sparse code stays sparse.
Keep a comment that carries hard-won context, such as why a workaround exists or which external quirk forces a value, even when it runs long.

## Audit and edit

Read the declaration, implementation, relevant callers, and any external contract needed to verify the comment.
Target drift first, then delete redundant narration and noise before rewriting.
Preserve useful local voice without treating stale comments as authority, and avoid wording churn without a concrete gain.
If a structural fix would remove the need for a comment, recommend that fix instead of hiding the problem in prose.

Match detail to the reader's dependency on the code.
Public surfaces and cross-system seams earn a comment only for what callers would get wrong from names and types, such as a surprising error, ordering, or ownership rule.
Private helpers and idiomatic glue rarely earn one, and no role requires a comment template.

## Length

A comment says what the code does and why it matters, not how it does it; the code already shows how.
Default to one sentence, and rarely go past two.
Merge related short facts into one sentence instead of stacking a line per fact.
Leave branch-by-branch behavior and edge cases to the code unless a caller would get them wrong without the comment.
Longer doc comments suit tags that carry separate facts, such as `@example`, `@throws`, or `@param`.

Keep doc comments attached to their declarations.
Prefer a few words trailing a field, constant, variable, or entry line over a leading doc block.
Keep consecutive side-commented declarations together without blank lines between them.
Give a type a one-line doc only when its name and fields leave its meaning unclear.
Do not write a doc paragraph that walks a type's fields; put each fact on its field's line, or drop it.
Use a leading doc comment for a function, or for a note that belongs to the whole declaration.
Delete member comments that only repeat the name.
In Bash and config, explain cryptic expansion, traps, quoting, external formats, non-obvious values, or cross-file coupling rather than straightforward assignments.

Use `TODO` only for a concrete unfinished action that cannot be completed in scope, and `NOTE` for durable surprising context.
Return a decision to the owner before adding `FIXME` or `HACK`, which assert debt or intentional compromise.
Do not add a marker when the in-scope fix is available.

Before returning, compare comments to source again and inspect the diff for behavior changes, accidental detachment, and unrelated churn.
Preserve compiler directives, lint controls, generated-file markers, and other machine-significant comments unless their change is explicitly authorized.
Report source ambiguity, deletion candidates, structural recommendations, and any unverified contract.
