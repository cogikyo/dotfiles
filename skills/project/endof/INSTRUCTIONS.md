# End of Day/Week Review

Generate work impact reviews from git activity across `~/LeadPier` repos. Writes to weekly files in `life/work/impact/`.

## Commands

- `/endof day [days...]` — Generate daily review(s)
- `/endof week` — Generate weekly review from daily entries

## Arguments

**Day targets** (space-separated, parallel agents):

- Day names: `Mon`, `Tue`, `Wed`, `Thu`, `Fri`
- ISO dates: `2026-02-18`
- No args → today

**Weekly file**: Determined from CWD or user-provided filename. Files live at `YYYY/MM/DD-DD.md` relative to `life/work/impact/`. The week header (e.g., `## Feb 16 - 20`) maps days to dates: Mon=16, Tue=17, etc.

---

## End of Day

Launch one **Task agent** (`subagent_type: "general-purpose"`) per day target. Run all day agents in parallel. Each agent does the following:

### 1. Resolve date

Map the day name to a concrete date using the weekly file header. Example: `## Feb 16 - 20` with `Wed` → `2026-02-19`.

### 2. Collect git activity

```bash
bash <SKILL_DIR>/scripts/git-activity.sh YYYY-MM-DD
```

Where `<SKILL_DIR>` is the directory containing this INSTRUCTIONS.md file. This scans all `~/LeadPier` repos for commits by `cullyn` on that date.

### 3. Read commits in depth

For each repo with activity, **read the actual diffs and commit descriptions** — not just subjects. Use `git show` or `git log -p` on interesting commits to understand what really changed and why. The goal is to understand the work well enough to explain it, not to parrot commit messages.

Also read for context:

- `~/LeadPier/CLAUDE.md` — backend/core repo structure and naming
- `~/LeadPier/leadpierui/CLAUDE.md` — frontend structure
- **Previous week's file** — avoid repeating work already summarized from squashed/rebased commits

### 3b. Check reflog for squashed/WIP branch work

The git-activity script only finds commits by author date. Work on WIP branches that gets repeatedly squashed (common pattern — commits labeled `WIP`) won't show up because the commit date stays old.

For each repo, also check the reflog for activity on the target date:

```bash
git -C <repo> reflog --format="%h %ci %gs %s" | grep "YYYY-MM-DD"
```

Look for `rebase (squash)`, `commit (amend)`, `commit: WIP`, or branch checkouts that indicate active development. If a WIP branch shows significant reflog activity on the target date, read that branch's content and include it in the summary (noting it as WIP).

### 4. Group by scope, then feature

**Scope** from repo path — each scope appears **once** as an `###` header:

| Path | Scope |
| --- | --- |
| `leadpierui` | Frontend |
| `services/*` | Backend |
| `core/*` | Backend |

Under each scope, group related commits into **feature area** sub-sections as `####` headers based on the substance of the changes. `core/*` repos are grouped under Backend — use the service name in the feature heading if disambiguation is needed (e.g., `#### Push Service`).

### 5. Synthesize — don't regurgitate

Write a **narrative summary** for each feature area. The agent's job is to understand the commits and describe the work in its own words — the why, the what, and the how.

```markdown
### Frontend

#### Feature Area

- Description of what was done and why. Additional detail if warranted.

#### Another Feature

- Description.

### Backend

#### Feature Area

- Description.

#### Push Service

- Core service changes described here.
```

Rules:

- **Heading hierarchy** — Scopes are `###` (`### Frontend`, `### Backend`), feature areas are `####`. Each scope appears at most once per day.
- **Synthesize**: read the diffs, understand the change, describe it — don't just echo commit subjects back
- **No line counts** — focus on what changed and why, not how many lines
- **No commit prefixes** — don't write `feat(x): ...` or `fix(y): ...` in the output; those are git conventions, not prose
- **Bullets for description** — each feature area gets a bullet list of what was done beneath its `####` heading
- **Mark ongoing work** with `(WIP)` on the feature area heading
- **Merge minor unrelated work** into a single `#### Minor Fixes & Cleanup` under the relevant scope

### 6. Write to file

Insert content under the correct `## Day` heading (e.g., `## Mon`) in the weekly file. Preserve all other sections unchanged.

If no commits found for a day, leave the section empty.

---

## End of Week

Single agent reads the full weekly file (all daily sections), then synthesizes.

### 1. Synthesize

Group all daily work by **project/feature** (not by day or scope). Consolidate smaller work into broader buckets (e.g., "Table & Infrastructure") rather than giving each its own section. Aim for 3–5 feature sections.

```markdown
**Feature Name [WIP if applicable]**

- Frontend: single-line summary when scope fits on one line
- Backend:
  - Sub-bullet when a scope has multiple distinct items
  - Another item
- Standalone bullet for cross-scope or minor items
```

### 2. Style

- **Bulleted lists** — each feature section is a bullet list, not prose paragraphs
- **Scope prefixes** — `Frontend:` and `Backend:` inline at the start of bullets
- **Nest when needed** — if a scope has 2+ distinct items, use sub-bullets; if it fits on one line, keep it flat
- **Bullets shouldn't wrap** — keep each bullet concise enough to read on one line
- **Slightly bigger picture** — summarize the what/why, not implementation details (e.g., "hardened template parser" not "strip escaped newlines, handle bare placeholders")
- Lead with highest-impact work
- Include communication items if present (demos, meetings, reviews)
- Keep the whole review section scannable — someone should get the week's story in 30 seconds

### 3. Write to file

Insert content under the `## Review` heading at the bottom of the weekly file. Preserve the `---` separator above it.
