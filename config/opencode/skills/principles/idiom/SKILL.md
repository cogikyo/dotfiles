---
name: idiom
description: "Use when writing or reviewing code, choosing library facilities, or preparing to hand-roll a solution."
---

# Idiom

Write it the way the language and codebase already do.
Idiomatic, readable code carries intent and serves as the source of truth.
A private version of an existing facility adds maintenance.

- Read nearby code and library conventions before choosing a form.
- Seriously, do not recreate a helper likely to exist in the modern standard library, an installed dependency, or the repository.
- Before building a tool, check whether a plugin or extension for a tool the project already uses covers the task.
- Use a framework or technical name when it is the honest interface term, not camouflage.
- Treat deprecated APIs and stale idioms as debt; check current library guidance before copying them.
- Use repository-owned rebuild or update commands when they own binary placement and lifecycle.

Idiom chooses established forms.
Legibility judges the layers and state a reader must track.

See: `warrant`, `legibility`.
