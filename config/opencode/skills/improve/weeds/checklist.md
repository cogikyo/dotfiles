# Weeds checklist

Code problems models keep writing across repositories, and how to make each one impossible.
Reviewers read only the entries for the diff's languages and mark a sighting the diff adds as `weed: <name>`.
Only a `/weeds` run edits this file; its rules are in `SKILL.md` beside it.

## Entries

### reinvented-helper (Go, TypeScript)

- **Problem:** Private code repeats the standard library, an installed dependency, or a repository helper.
- **Spot:** Search the standard library, dependency docs, and the repository for an equivalent; grep for private helpers named like standard ones (`cmpOr`, `contains`, `equal*`), `JSON.parse` wrapped in try/catch, and hand-written types that mirror a dependency's exported type.
- **Kill:** Delete it and call the existing one; Go `modernize` for the idioms it knows, a fixed set such as `slicescontains` that misses private clones, wrappers, and hand-rolled library functions.
- **Seen:** `claude-opus-5-5`, lane level, 2026-11-13.
- **Judged:** 2026-11-13, quiet 0.

### duplicate-knowledge (Go, TypeScript, SQL)

- **Problem:** Copies of one rule, such as a function, query, constant, format, or type, live in several files and must change together.
- **Spot:** First show that a change to one copy must reach the others, since code that only looks alike is not this weed; then find identical bodies or literals across files, and grep the callers of a changed function, query, or constant for copies.
- **Kill:** One owned copy; `jscpd` (default 50 tokens and 5 lines, `--changed`), Go `dupl` (100 tokens standalone, 150 in golangci-lint).
- **Seen:** `claude-opus-5-5`, lane level, 2026-11-13.
- **Judged:** 2026-11-13, quiet 0.

### swallowed-error (Go, TypeScript, Python)

- **Problem:** A discarded error turns a failure into success.
- **Spot:** Trace each error path to the user-visible outcome; grep Go `x, _ :=`, `_ = f()`, and zero-value returns in `err != nil` branches, TypeScript `catch {}`, `.catch(() => …)`, and `Effect.ignore`, and Python bare `except`.
- **Kill:** Propagate, and name the errors that are benign; Go `errcheck` with `-blank` or `check-blank: true` (it ignores `_ =` by default), plus `nilerr`; `no-empty` for `catch {}` (oxlint `eslint/no-empty`), `no-empty-function` for `.catch(() => {})`; Ruff `S110`, `E722`, `BLE001` (all on by default).
- **Seen:** `claude-opus-5-5`, lane level, 2026-11-11.
- **Judged:** 2026-11-13, quiet 0.

### lost-intent (Go, TypeScript, Python, shell)

- **Problem:** An interruption or restart forgets what was in progress, so a rerun misses the outcome it promises.
- **Spot:** No grep pattern; walk "interrupt after step N, then rerun", looking for writes ordered before the record that prevents a retry and in-memory state whose loss on reload changes the result; a cache that is safe to rebuild empty is not this weed.
- **Kill:** Record intent before the destructive step, or derive it from current state; an interrupt-then-rerun test. No lint.
- **Seen:** `claude-opus-5-5`, lane level, 2026-11-13.
- **Judged:** 2026-11-13, quiet 0.

### dead-after-refactor (Go, TypeScript)

- **Problem:** Fields, parameters, exports, or branches outlive their last reader.
- **Spot:** Grep the readers of each changed field, parameter, export, and branch; run the Kill tools where the repository has them.
- **Kill:** Delete; Go `unused` (U1000) with `field-writes-are-uses: false`, `unparam` for parameters, `deadcode` for unreachable exported functions; TypeScript `knip` for files, exports, and dependencies, `noUnusedLocals`, `noUnusedParameters`; nothing named flags an unread object or interface field.
- **Seen:** `claude-opus-5-5`, lane level, 2026-11-13.
- **Judged:** 2026-11-13, quiet 0.

### second-source (Go, TypeScript)

- **Problem:** A stored field competes with the state that already determines it.
- **Spot:** For each new stored field, flag, or map, check whether existing state such as a count or a published value already determines it.
- **Kill:** Delete the field and derive it. No lint.
- **Seen:** `claude-opus-5-5`, lane level, 2026-11-13.
- **Judged:** 2026-11-13, quiet 0.

## Candidates

- `stale-idiom` (Go): code keeps an idiom that is obsolete for the language version the repository supports; first seen 2026-11-13.
- `steering-flag` (TypeScript): a boolean parameter exists only to pick a branch the caller already knows; first seen 2026-11-13.
- `unvalidated-cast` (TypeScript): outside data is cast into a trusted type without validation; first seen 2026-11-13.
- `unlocked-publish` (Go): a shared pointer is published to concurrent readers with no synchronization; first seen 2026-11-13.
- `missing-as-zero` (TypeScript): unknown data is reported as a measured zero; first seen 2026-11-13.
- `unbounded-retention` (Go, TypeScript): a map or cache keeps keys with no lifetime or cardinality bound; first seen 2026-11-13.
- `unbounded-lifetime` (Go, TypeScript): work drops or ignores its caller's deadline; first seen 2026-11-13.
