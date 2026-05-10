---
name: commit
description: Smart git commits. Use when user says /commit or asks to commit changes. Supports /commit quick for small in-session commits; handles messy states with stash-first safety, atomic staging, and conventional commit messages.
invocation: user
---

Invoke with `/commit`, `/commit quick`, or when committing is requested.

If session already in progress, likely means to commit only work relevant to the sesion.
If no extra provided, usually means to commit everything.

Read `/home/cullyn/dotfiles/skills/commit/INSTRUCTIONS.md` before executing.
