# Comment Style Guide

## Default: no comment

Most code doesn't need a comment. Good names and structure carry the meaning.

Before writing or keeping a comment, run the reader check:

- Is there enough info without it?
- Does it help scanning or searching for functionality?
- Is it accurate now, and will it stay accurate?
- Could it be tighter?
- Would removing it make the file easier to read?

If the code is unclear, **prefer `FIXME` over prose explanation** — the fix is the real answer, narration is a workaround.

### FIXME categories

- `FIXME: idiomatic` — not idiomatic for the language
- `FIXME: clarity` — naming, structure, or flow obscures intent
- `FIXME: simplify` — can be shorter or less indirect

Grep-able breadcrumbs for future cleanup. Use these instead of explaining why the code is awkward.

## When a comment earns its place

- **Contract** — what an exported API promises (inputs, outputs, errors).
- **Coupling** — non-obvious dependencies on or from other code.
- **Invariant** — locking discipline, ordering, state-machine rules.
- **External format** — quirks of a file format, protocol, or third-party API.
- **Surprise** — workaround, hard-won fix, counterintuitive approach.

If a comment fits none of these, delete it.

Signal, not noise. Less is more.

## Formatting rules

### One sentence per line

If a sentence needs wrapping, the comment is too verbose. Rewrite it shorter, or split it into two sentences.

Bad — one sentence wrapped:
```go
// App/urgency keys and SilentApps are normalized to lowercase so dispatch
// logic can match against libnotify metadata case-insensitively.
```

Good — one line:
```go
// Notify keys and SilentApps are lowercased for case-insensitive libnotify matching.
```

Or two short lines, each a complete sentence:
```go
// Notify keys and SilentApps are lowercased.
// libnotify metadata matches case-insensitively.
```

### Other formatting

- Doc comments: capitalize first word, end with period, complete sentences.
- Inline comments: lowercase fragments fine, periods optional.
- Hard limit 120 chars per line. Target ~80. Rarely hit 120.
- Imperative voice for inline comments ("Retry up to 3 times", not "Retries up to 3 times").
- Bulleted lists are good when enumerating steps, behaviors, or options.

## Comment tiers

Guidelines, not rules. Adjust for context.

| Tier            | When                                              | What to include                                      |
| --------------- | ------------------------------------------------- | ---------------------------------------------------- |
| **Thorough**    | Exported APIs, shared types, interfaces           | Purpose, params, returns, errors, example if useful  |
| **Intentional** | Core entities: handlers, orchestrators, runners   | Purpose + coupling/invariant notes                   |
| **Minimal**     | Helpers, small idiomatic functions                | Nothing unless syntax or a quirk is rare             |

## Package-level comments

For Go code, this is required:
- Each package needs a detailed package-level doc comment.
- Prefer placing that package doc in the main entry file (the most important file), not only in a tiny side file.
- Each `.go` file needs a file-level purpose comment.
- File-level purpose comments go between the `package` declaration and the `import` block (if imports exist).
- Use a consistent package-doc shape:
  - First line: `Package <name> <does what>.`
  - Optional short context paragraph when needed.
  - Optional `Responsibilities:` block using `-` bullets (not numbered lists).

File-level comments can be longer than per-function docs when they explain what the file does and how it fits with sibling files.

Go:
```go
// Package windows provides reusable window-selection helpers over Hyprland client lists.
//
// Responsibilities:
// - Filter and order tiled windows for master/slave layouts.
// - Match windows by class/title for command targeting.
// - Expose geometry helpers reused by wm and session packages.
package windows

// tiled.go defines tiled-window filtering, matching, and geometry helpers.

import "context"
```

## Doc comment structure

Hybrid: summary line, optional short body (one sentence per line), optional structured block (params, returns, example) only when it adds value.

```
[summary — starts with identifier name where convention requires]

[short body — one sentence per line]

[structured block — only if genuinely helpful]
```

## Section headers

For files 100+ lines, use comment-box dividers to partition major regions:

```
# ╭───────────────────────────────────────────────────────────────────────────────╮
# │ major section                                                                 │
# ╰───────────────────────────────────────────────────────────────────────────────╯
```

Sub-section label:
```
# ├─ sub-section label ──────────────────────────────────────────────────────────┤
```

External references (specs, upstream docs, APIs):
```
# ╓
# ║ https://some-external-doc — purpose
# ║ https://another-reference — purpose
# ╙
```

Box width 80 chars. Adapt the comment prefix per language (`#`, `//`, etc).

File size drives the decision, not file type. Don't decorate small files.

## Markers

| Marker              | Meaning                                            |
| ------------------- | -------------------------------------------------- |
| `TODO`              | Planned improvement, not blocking                  |
| `FIXME: idiomatic`  | Not idiomatic for the language                     |
| `FIXME: clarity`    | Naming, structure, or flow obscures intent         |
| `FIXME: simplify`   | Can be shorter or less indirect                    |
| `HACK`              | Intentional shortcut, explain why                  |
| `NOTE`              | Non-actionable context for future readers          |

Scribe can leave markers when it finds issues needing user input. Confirm before adding `FIXME:*` or `HACK` (these imply follow-up work).

---

## Go

Follow [godoc conventions](https://go.dev/doc/comment):

- Start doc comment with the identifier name.
- Use `//`, not `/* */`.
- Bulleted lists via indented lines.
- Package doc above `package` declaration.
- For this skill, prefer package docs in the main entry file when practical.
- Add a file-purpose comment between `package` and `import`.
- Use `Responsibilities:` + `-` bullets for package responsibilities when listing behavior.
- For multi-line type/function/method docs, keep a one-line summary, then a blank `//` line, then details.

```go
// ParseConfig reads a YAML file at path and returns a validated Config.
//
// Missing fields fall back to defaults:
//   - Timeout: 30s
//   - Port: 8080
```

Preferred multi-line identifier doc shape:
```go
// ThreeBody implements a 3-window layout: master + active slave + hidden shadow.
//
// Invariant: when enrolled, exactly two windows are tiled and the shadow is parked on cfg.Windows.ShadowWorkspace.
type ThreeBody struct {
	// ...
}
```

Avoid:
```go
// ThreeBody implements a 3-window layout: master + active slave + hidden shadow.
// Invariant: when enrolled, exactly two windows are tiled and the shadow is parked on cfg.Windows.ShadowWorkspace.
```

File-purpose example:
```go
package main

// daemon.go defines ewwd runtime orchestration, provider wiring, and socket command handlers.

import (
	"context"
)
```

## TypeScript

TSDoc in `/** */` blocks. Use `@param`, `@returns`, `@example`, `@deprecated` only when they add information not obvious from types.

## Bash / Shell

- File header: purpose + usage, 1-2 lines.
- Section dividers for logical blocks.
- Inline comments for cryptic syntax (parameter expansion, process substitution, etc).

## Config files (YAML, TOML, Hyprland, etc.)

- Section headers are essential — these tend to be monolithic.
- Inline comments for non-obvious values or cross-file references.
