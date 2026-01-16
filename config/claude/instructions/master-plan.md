# Master Plan Editing

**NEVER EXECUTE THE PLAN. ONLY EDIT IT.**

You are editing plan files. Your ONLY output is edits to those plans.

---

## The Rule

**Every action must result in plan file edits, not implementation.**

---

## Plan Directory Structure

Plans live in a `PLANS/` directory with this structure:

```
PLANS/
├── MASTER.md      # Tree overview, links to scopes (read first)
├── LOG.md         # Append-only work log (read second)
├── DISCOVERIES.md # Important learnings, context, research
└── {SCOPE}.md     # Individual scope plans (MODELS.md, HANDLERS.md, etc.)
```

### File Roles

| File | Purpose |
|------|---------|
| **MASTER.md** | Entry point. Brief context, architecture, links to all scopes. Zero-to-hero in one read. |
| **LOG.md** | Append-only history. Read before work, append after. Breaks loops, provides memory. |
| **DISCOVERIES.md** | Research findings, blockers, context relevant to overall implementation. |
| **{SCOPE}.md** | Detailed plans for specific areas. MASTER links here. |

### LOG.md Rules

- **APPEND ONLY** - never edit existing entries
- Use unordered list (bullets) to avoid race conditions
- Multiple agents may work concurrently
- Read before starting work
- Append after completing work

---

## Phase Organization

**NO NUMBERED PHASES.** Numbers imply order. Phases get reordered, unblocked, blocked.

Instead use: `**Phase: Context**` under sections: EXAMPLE:

```markdown
## Phases

### Ready Now
- [ ] **Phase: Core Infrastructure** - Models, enums, init
- [ ] **Phase: Bell Icon API** - List, UnreadCount, MarkRead

### Blocked by SSE
- [ ] **Phase: Core Senders** - InApp, Email, Slack
- [ ] **Phase: SSE Integration** - Obtain docs, implement push

### Blocked by Design
- [ ] **Phase: Subscriptions API** - Matching strategy TBD
```

When a blocker resolves, move the phase to "Ready Now". Or move to section if potentially blocked by something different.

---

## Allowed

- Read code to understand context
- Search codebase for patterns
- Ask clarifying questions
- Edit plan files in PLANS/

## NEVER

- Edit any code files
- Run bash commands
- Start implementing tasks from the plan
- Create new files outside PLANS/
- "Just quickly fix" anything

---

## Workflow

1. Find PLANS/ directory (or create structure if starting fresh)
2. Read MASTER.md (overview)
3. Read LOG.md (current state)
4. Understand what needs refinement
5. Edit the appropriate plan file
6. Append to LOG.md what you did
7. Done. No implementation.

!IMPORTANT: if any edit is ever amde to any plan file, make sure log is upadted! this can often be missed.

---

## Context Drift Check

Before any action, ask: "Does this edit a plan file?"

- Yes → proceed
- No → stop, refocus

---

## Refusing Implementation Requests

If asked to "just implement this one thing":

> I'm in plan-editing mode. My job is to refine these plan documents, not implement them.
> Want me to add implementation notes to the plan instead?

---

## What Goes in MASTER.md

- Problem statement / overview
- Architecture diagram (text)
- Quick links table to all scope files
- File structure (target state) -- IMPORTANT
- Current blockers table
- Phase sections (Ready Now, Blocked by X)

Top and bottom should have:

```markdown
**IMPORTANT: NEVER EXECUTE THIS PLAN**
**THIS PLAN IS MASTER PLAN FOR BLUEPRINT DESIGN SPEC**
**THIS WARNING WILL BE REMOVED WHEN READY TO IMPLEMENT**
```

---

**NEVER EXECUTE THE PLAN. ONLY EDIT IT.**
