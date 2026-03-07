---
name: endof
description: Generate end-of-day or end-of-week work impact reviews from git activity across ~/LeadPier repos. Use when user says /endof day or /endof week. Scans all git repos, groups changes by scope and feature area, writes to weekly markdown files in life/work/impact/.
invocation: user
---

Generate work impact reviews from git history. Project-scoped to `life/work/impact/`.

## Repos

Scan all git repos under `~/LeadPier/` (leadpierui, services/*, core/*) for commits by `cullyn`.

## File structure

- Weekly files: `YYYY/MM/DD-DD.md` (e.g., `2026/02/23-27.md`)
- Quarterly reviews: `YYYY/QN.md`
- End-of-year: `YYYY/EOY.md`

## Daily entries (`/endof day`)

```
## Mon

### Frontend

#### Feature Area (WIP)

- Bullet point of work done
- Another bullet point

### Backend

#### Feature Area

- Bullet point
```

- One `###` section per stack (Frontend / Backend)
- One `####` section per feature area
- Bullet points describe concrete work done
- Mark `(WIP)` on feature areas that are in progress
- Each bullet should be meaningful but concise — no wrapping lines

## Weekly review (`/endof week`)

Goes at the bottom of the weekly file after `---` separator.

```
## Review

**Feature Area [WIP]**

Backend:
- Concise bullet
- Another bullet

Frontend:
- Concise bullet

**Another Feature Area**

- Bullet (no stack sub-header needed if single-stack or obvious)
```

### Format rules

- Group by **feature area**, not by day or stack
- Use `Backend:` / `Frontend:` / `Full:` as **sub-headers** with bullets underneath
  - NOT as a prefix on every bullet (not `- Backend: did thing`)
- If a feature area only touches one stack or it's obvious, skip the sub-header and just use bullets
- Mark `[WIP]` on feature areas still in progress
- **Lines must not wrap** — keep each bullet short and standalone
- Higher level than daily entries but still enough context to understand the work
- Don't just comma-separate things; each bullet should be a clear standalone item
- Draft the review for the user before writing to file

## General

- When commits are squashed/mixed across days, use diff content and context to attribute work to the right day
- Check existing entries in the file before writing to avoid duplicates
- Always read recent weekly files first to match the established voice and detail level
