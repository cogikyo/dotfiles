---
name: go
description: Use with comments when auditing or editing Go doc comments, exported API documentation, or package documentation.
---

# Go comments

Read `comments` for the general procedure.
Use godoc conventions and prefer `//` comments.
Start exported declaration comments with the symbol name and keep them attached to that declaration.
Place package documentation in `doc.go` or immediately above `package`.

Describe caller-visible semantics that the signature does not supply, such as ownership, ordering, errors, or meaningful edge cases supported by inspected source.
Do not enumerate parameters and results merely because they exist.
Keep field and constant comments local and omit them when names already carry the meaning.
