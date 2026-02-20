---
name: eslint-fix
description: Fix ESLint/TS errors in leadpierui (TS/TSX only, ignore Go/other repos). Use when user says /eslint-fix, asks to fix lint errors, or when a pre-commit/pre-push hook fails. Use `/eslint-fix push` to push with automatic type error fixing.
invocation: user
---

- `/eslint-fix` — fix ESLint errors in staged files
- `/eslint-fix push` — push, fix type/lint errors from pre-push hook, commit, retry
