---
name: commit
kind: command
description: Smart git commits. Use when user says /commit or asks to commit changes. Supports /commit quick for small in-session commits; handles messy states with stash-first safety, atomic staging, and conventional commit messages.
invocation: user
---

# commit

Treat the following text (if any):

```text
$ARGUMENTS
```

as mode, filter input, or user context for skill `commit`.
