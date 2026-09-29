---
name: go
description: Use with comments when auditing or editing Go doc comments, exported API documentation, or package documentation.
---

# Go comments

This skill adds Go conventions to `comments`, which owns the general procedure.

- Write doc comments as `//` line comments in full sentences that start with the declared name, such as `// Parse reads ...`.
- Start package documentation with `// Package <name>`, in `doc.go` or directly above `package` in one file.
- Mark deprecation with a paragraph that starts `Deprecated:` and names the replacement.
- Link other identifiers with doc links such as `[Reader]` or `[io.EOF]` when the reference helps the reader.
- Use gofmt doc syntax for structure: a bare `//` line between paragraphs, indented lines for code, and `-` for lists.
- Describe errors, ownership, concurrency safety, and ordering in prose; Go has no parameter or result tags, so skip per-parameter inventories.
- Keep `//go:` directives and `//nolint` markers without a space after `//`, because tools only read them in that form.
