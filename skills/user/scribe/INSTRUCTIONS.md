# Scribe

Comment and documentation management. Read `references/style-guide.md` before making any changes.

## Prime directive

Signal, not noise. Less is more. Default to **no comment**; each one must earn its place.

When editing, prefer subtractive changes. Remove comments that restate code, wrap single sentences across lines, or narrate obvious flow. When code is unclear, leave a `FIXME:` marker (see style guide for categories) rather than explaining it in prose.

## Commands

### `/scribe review`

Audit comments across a scope (file, directory, or repo).

1. Read the scope (user explains what to review, or defaults to staged+modified files).
2. Read `references/style-guide.md`.
3. Check each file for:
   - **Drift** — comments that no longer match the code.
   - **Redundancy** — duplicated information across comments.
   - **Verbosity** — single sentences wrapped across lines, over-explanation, filler.
   - **Navigation** — do section headers and doc comments tell a coherent story?
   - **Style violations** — formatting, voice, punctuation per the style guide.
4. Present findings as a plan with suggested fixes.
5. Ask for confirmation before applying changes.
6. Leave TODO/FIXME markers (with user confirmation for FIXME/HACK) where implementation is unclear.

### `/scribe update`

Add or update comments for specific files. User provides paths or context about what changed.

Permission model:
- `/scribe update` may read scoped files, edit scoped documentation/comments, run formatters for edited files, and run non-destructive verification without asking first.
- For Go documentation edits, allowed verification includes `gofmt`, `go test`, `go test ./...`, `go vet`, `go build`, `go list`, and `go env` when scoped to relevant packages.
- Do not run commands that start, stop, or manipulate live user sessions unless explicitly authorized by the user or parent agent.

1. Read the target files and `references/style-guide.md`.
2. Apply the style guide directly — no confirmation needed.
3. **Prefer deletion over addition.** Remove before adding.
4. Determine comment tier per function/block (thorough / intentional / minimal — see style guide).
5. Add/update doc comments, inline comments, and section headers only where they earn their place (contract, coupling, invariant, external format, surprise).
6. Never wrap a single sentence across lines. If a sentence is too long, rewrite it shorter or split it into two.
7. If better naming or file organization would eliminate the need for a comment, suggest it instead.

Go-specific requirements:
- Treat these as required for Go-related edits, not optional style choices.
- Ensure each Go package has a detailed package doc comment. Prefer placing it in the main entry file (most important file), not only in a tiny side file.
- Keep package docs consistent: `Package <name> ...` summary, then optional `Responsibilities:` section with `-` bullets (not numbered).
- Ensure each Go source file has a file-level purpose comment between the `package` declaration and the `import` block (if imports exist).
- The file-level comment should be one clear sentence describing what the file primarily defines/orchestrates, e.g. `// daemon.go defines ewwd runtime orchestration, provider wiring, and socket command handlers.`
- For multi-line Go doc comments on types/functions/methods, format as:
  - One summary line tied to the identifier.
  - One blank comment line (`//`).
  - Follow-up details (invariants, behavior notes, lists).
- Do not place detail lines directly under the summary without that blank separator.
- If either comment is missing during `/scribe update`, add it.

### `/scribe question`

Answer a question about code using comments + source as context.

1. Read relevant code and comments to answer the question.
2. Provide a clear, concise answer.
3. **Leave it cleaner than you found it.** If comments were unclear or insufficient, update them so the next reader doesn't need to ask.
4. Minor comment fixes don't need confirmation; significant additions do.

## Principles

- **Default: no comment.** Most code doesn't need one. Good names carry meaning.
- **One sentence per line.** Never wrap mid-sentence. If a sentence needs wrapping, the comment is too verbose — rewrite shorter or split.
- **Architecture over comments.** If better naming or structure eliminates the need, prefer that.
- **FIXME over prose.** When code is unclear, leave a `FIXME: idiomatic|clarity|simplify` marker rather than explaining the awkwardness.
- **Reader check.** Before keeping a comment: enough info to navigate? aids searching? accurate? could be tighter? would removing help?
- **Language-aware.** Apply the correct doc convention per language (see style guide).
- **Go docs are required.** For Go work, require both package-level and per-file purpose comments in the mandated placement.
- **Markers are tools.** TODO/FIXME/HACK/NOTE are grep-able breadcrumbs; confirm with user before adding FIXME/HACK.
