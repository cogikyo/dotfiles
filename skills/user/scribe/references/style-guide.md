# Comment Style Guide

## Comment Tiers

Match comment depth to the code's role:

| Tier            | When                                                          | What to include                                                         |
| --------------- | ------------------------------------------------------------- | ----------------------------------------------------------------------- |
| **Thorough**    | Shared/reusable code: utils, types, interfaces, exported APIs | Full doc: purpose, params, returns, errors, example. LSP-hover friendly |
| **Intentional** | Core entities: handlers, orchestrators, main runners          | Purpose doc + coupling/implementation notes when they prevent foot-guns |
| **Minimal**     | Helpers, small idiomatic functions                            | Only when syntax is unfamiliar or internal library quirks exist         |

These are guidelines, not rigid rules. Adjust based on context.

## Formatting Rules

- **Doc comments**: capitalize first word, end with period. Complete sentences
- **Inline comments**: relaxed — lowercase fragments fine, periods optional
- **Never wrap mid-sentence**: one thought per line, soft limit ~120 chars
- **Bulleted lists**: welcome in doc comments for enumerating behavior, params, etc
- **Imperative voice** for inline comments: "Retry up to 3 times" not "Retries up to 3 times"

## Doc Comment Structure (All Languages)

Hybrid format — prose summary lead, then tags/sections only when they add value:

```
[summary line — starts with function/type name where convention requires]

[prose body — brief, covers behavior, edge cases, coupling notes]

[tags/sections — only when they improve clarity or LSP experience]

[example — when non-obvious usage exists]
```

## Section Headers

Use comment-box decorative styles for file sections in **large files (~100+ lines)**:

```
# ╭───────────────────────────────────────────────────────────────────────────────╮
# │ major section                                                                 │
# ╰───────────────────────────────────────────────────────────────────────────────╯
```

```
# ├─ sub-section label ──────────────────────────────────────────────────────────┤
```

```
# ╓
# ║ https://some-external-doc — what this link is for
# ║ https://another-reference — brief description
# ╙
```

- **Major section** (`╭╮╰╯`): group large logical blocks within a file
- **Sub-section** (`├┤`): label smaller divisions within a major section
- **External references** (`╓║╙`): link to relevant docs, specs, or APIs

Box width is 80 chars (matching comment-box.nvim config). Adapt comment prefix to language (`#`, `//`, etc).

Works in any language, not just bash/configs. The rule is file size, not file type.

## Role of Comments

- **Contract-first**: describe what code promises, not how it works internally
- **Coupling notes**: when a function depends on or is depended on by non-obvious code, say so
- **Implementation notes**: only when the approach is surprising or fragile

## Markers

| Marker  | Meaning                                   |
| ------- | ----------------------------------------- |
| `TODO`  | Planned improvement, not blocking         |
| `FIXME` | Known bug or correctness issue            |
| `HACK`  | Intentional shortcut, explain why         |
| `NOTE`  | Non-actionable context for future readers |

Scribe can leave markers when it finds issues needing user input. Always confirm with user before adding FIXME/HACK.

---

## Go

Follow [godoc conventions](https://go.dev/doc/comment) with extras:

```go
// ParseConfig reads a YAML config file at path and
// returns a validated Config.
//
// Missing fields fall back to defaults:
//   - Timeout defaults to 30s
//   - Port defaults to 8080
//
// Deprecated: Use LoadConfig instead.
```

- Start doc comment with the function/type name
- Use `//` style (not `/* */`)
- Section headers: `// Deprecated:`, bulleted lists, code examples via indented lines
- Package comments go in `doc.go` or above `package` declaration

## TypeScript

Use [TSDoc](https://tsdoc.org/) in `/** */` blocks:

```typescript
/**
 * Parse and validate a YAML config file.
 *
 * @remarks
 * Falls back to defaults for missing fields.
 *
 * @param path - absolute path to config
 * @returns validated config object
 */
```

- `@remarks` for extended description
- `@param`, `@returns` when params/returns aren't self-evident from types
- `@example` for non-trivial usage
- `@deprecated` with migration path

## Bash / Shell

- File header: purpose + usage (1-2 lines)
- Section separators for logical blocks (comment-box style)
- Inline comments for cryptic bash syntax (parameter expansion, process substitution, etc.)

```bash
#!/usr/bin/env bash
# Install and link dotfiles.
# Usage: ./install [step]

set -euo pipefail

source "$HOME/.config/shell/env.sh"

# parameter expansion: default to 'all' if unset
step="${1:-all}"
```

## Config Files (YAML, TOML, Hyprland, etc.)

- Section headers are essential — these are often monolithic files
- Inline comments for non-obvious values or references to other config
- Group related settings visually
