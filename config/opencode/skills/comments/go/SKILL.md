---
name: go
description: Use with comments when auditing or editing Go doc comments, exported API documentation, or package documentation.
---

# Go comments

This skill adds Go conventions to `comments`, which owns the general procedure.

- Treat Go's convention of documenting every exported identifier as optional; follow the repository's density, and leave undocumented what `comments` would not admit.
- When a doc comment is warranted, write it as `//` line comments in full sentences that start with the declared name, such as `// Parse reads ...`.
- Put one sentence on each doc-comment line, under about 80 columns including `//`; shorten it or split it into two sentences instead of manually wrapping it.
- Start package documentation with `// Package <name>`, in `doc.go` or directly above `package` in one file.
- Mark deprecation with a paragraph that starts `Deprecated:` and names the replacement.
- Link other identifiers with doc links such as `[Reader]` or `[io.EOF]` when the reference helps the reader.
- Use gofmt doc syntax for structure: a bare `//` line between paragraphs, indented lines for code, and `-` lists when they read better.
- State errors, ownership, concurrency safety, or ordering in prose only when a caller would get them wrong; Go has no parameter or result tags, so skip per-parameter inventories.
- Keep `//go:` directives and `//nolint` markers without a space after `//`, because tools only read them in that form.
