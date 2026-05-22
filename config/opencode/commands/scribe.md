---
name: scribe
kind: command
description: Audit or apply comment and documentation updates using the current message context.
invocation: user
---

# scribe

Manage comments and documentation from the current message, conversation context, changed files, or explicitly named scope.
Infer whether the user wants review or update behavior from their request.
Do not require literal subcommands or argument placeholders.

## Modes

### Review mode

Use review mode when the user asks to audit, assess, check, plan, or find comment/documentation issues.
Review comments and docs across the inferred scope, usually the files named by the user or the staged and modified files when no scope is given.
Return findings and suggested fixes.
Ask for approval before applying changes unless the user already delegated an implementation slice.

Assess:

- Drift: comments or docs that no longer match code behavior.
- Redundancy: comments that duplicate code or repeat nearby docs without adding contract, invariant, coupling, or navigation value.
- Navigation and story: section headers, doc comments, and file docs should help readers build the right map of the code.
- Conciseness: verbose comments are a problem when they obscure useful signal or violate local convention.
- Style violations: formatting, voice, punctuation, and prose shape should follow local convention first.
- Missing context: contracts, invariants, external formats, surprising behavior, fragile implementation details, and hard-won coupling knowledge.

### Update mode

Use update mode when the user asks to add, update, repair, or apply comment/documentation fixes.
Read the relevant files and local conventions first.
Apply the smallest comment/doc change that fixes the documented issue.
If better naming, organization, or structure would remove the need for a comment, suggest that architecture change instead of compensating with prose unless the user explicitly requested comment-only edits.

## Principles

- Prefer architecture over comments.
- Names, types, tests, file layout, and control flow should carry meaning before comments do.
- Comments must earn their place by documenting contracts, coupling, invariants, external formats, surprises, fragile implementation details, or hard-won context.
- Do not churn comments for taste.
- Prefer deleting stale, redundant, or noisy comments over rewriting them.
- Preserve local repo conventions over this guide when they conflict.
- Use one sentence per line in comments and Markdown prose where practical.
- Never wrap a single sentence across multiple lines just to fill width.

## Comment tiers

Match depth to the code role.
These tiers are guidelines, not rigid rules.

| Tier        | Use when                                                                                          | Include                                                                                                                      |
| ----------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Thorough    | Shared or reusable code such as utilities, types, interfaces, exported APIs, and public commands. | Purpose, params, returns, errors, edge cases, examples, and LSP-hover useful details when they add value.                    |
| Intentional | Core entities such as handlers, orchestrators, main runners, workflows, and cross-system seams.   | Purpose, contract, coupling notes, invariants, and implementation notes that prevent foot-guns.                              |
| Minimal     | Helpers, small idiomatic functions, straightforward wiring, and local glue.                       | Only non-obvious syntax, library quirks, external constraints, or context a future maintainer could not reconstruct cheaply. |

## Doc comment structure

Use a hybrid structure.
Lead with a prose summary, then add sections only when they improve clarity or tooling.

```markdown
[summary line, starting with the symbol name where the language convention requires it]

[brief body covering behavior, edge cases, coupling, invariants, or external contracts]

[tags or sections only when they improve clarity, generated docs, or LSP experience]

[example only when usage is non-obvious]
```

Doc comments should usually be complete sentences.
Capitalize the first word and end complete sentences with a period unless the local style says otherwise.
Inline comments may be fragments when that is clearer.

## Section headers and comment boxes

Use section headers for large files or monolithic config where they materially improve navigation.
Do not add decorative headers to small files or obvious code.
When local conventions allow comment boxes, use them for major sections, sub-sections, and external references.
Adapt the comment prefix to the language.

Rules of thumb:

Section Headers: when already exist, user asks for it, or file is > ~300 lines.
Sub-Section Label: deeply nested long functions that COULD be decomposed but is monolithic for legacy or other reasons.
External Docs: when cotains links with context explations -- almost always good.

```bash
# ╭───────────────────────────────────────────────────────────────────────────────╮
# │ major section                                                                 │
# ╰───────────────────────────────────────────────────────────────────────────────╯

# ├─ sub-section label ──────────────────────────────────────────────────────────┤

# ╓
# ║ https://some-external-doc — what this link is for
# ╙
```

## Markers

Markers are grep-able breadcrumbs, not a substitute for fixing known problems.

| Marker  | Meaning                                                     |
| ------- | ----------------------------------------------------------- |
| `TODO`  | Planned improvement, not blocking.                          |
| `FIXME` | Known bug or correctness issue.                             |
| `HACK`  | Intentional shortcut; explain why and what would remove it. |
| `NOTE`  | Non-actionable context for future readers.                  |

Ask for explicit approval before adding `FIXME` or `HACK` markers.
Prefer narrow markers like `FIXME: idiomatic`, `FIXME: clarity`, or `FIXME: simplify` when the user approved leaving a breadcrumb but the code needs a later structural change.
Do not add broad TODO markers when the issue can be fixed within the current approved scope.

## Language notes

### Go

- Follow godoc conventions.
- Start exported symbol comments with the symbol name.
- Use `//` comments rather than block comments for ordinary docs.
- Put package comments in `doc.go` or immediately above the `package` declaration.
- Use sections such as `Deprecated:` and indented examples only when they improve generated docs.

### TypeScript

- Use TSDoc `/** */` for public APIs and exported surfaces.
- Use `@remarks`, `@param`, `@returns`, `@example`, and `@deprecated` only when types and names are not enough.
- Avoid restating TypeScript types in prose.

### Bash and shell

- File headers should state purpose and usage in one or two lines when useful.
- Use section separators for long scripts and logically grouped shell/config blocks.
- Explain cryptic parameter expansion, process substitution, traps, quoting constraints, and shell-specific behavior.
- Prefer `#!/usr/bin/env bash` plus `set -euo pipefail` where local conventions require bash scripts.

### Config files

- Section headers are often useful because config files become monolithic.
- Inline comments should explain non-obvious values, external references, generated constraints, or cross-file coupling.
- Group related settings visually without adding noise to self-explanatory values.

## Output discipline

For review work, report findings first with file and line evidence when possible.
For update work, report changed files, what changed, verification, and residual risk.
Keep the scope bounded to comment and documentation work unless the user approves broader architecture changes.
