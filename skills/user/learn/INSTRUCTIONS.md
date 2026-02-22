# Learn - Skill Creator

Skills extend any AI agent with specialized knowledge for using tools, workflows, and domain expertise.

## Commands

### `/learn audit`

Run linter on all skills: `scripts/audit.sh`

Or lint a specific skill by name: `scripts/audit.sh learn`

Reports pass/fail for each skill in `user/` and `project/`, plus:
- Per-check status (`✔`, `✖`, faint `->` for skip)
- Compact per-check metadata (counts/limits/context), including command lists for command-sync

---

### `/learn new`

**1. Ask scope:**

- **user** → `~/dotfiles/skills/user/<name>/`
- **project** → `~/dotfiles/skills/project/<name>/`

After creating, link them:
- user skills: `~/dotfiles/skills/link.sh user`
- project skills (inside a project): `~/dotfiles/skills/link.sh project <name>`

**2. Gather requirements:**

- "What tools/binaries does this skill use?"
- "Give an example of how you'd use it"
- "What should trigger this skill?"

**3. Plan reusable content:**

For each example, ask:

- "What code do I rewrite every time?" → `scripts/`
- "What schemas/docs do I rediscover?" → `references/`
- "What boilerplate do I copy?" → `assets/`

**4. Create the skill:**

```
skill-name/
├── SKILL.md           # YAML frontmatter + summary
├── INSTRUCTIONS.md    # Full workflows
├── references/        # Domain docs (optional)
├── scripts/           # Deterministic code (optional)
└── assets/            # Templates, boilerplate (optional)
```

**5. Run linter:**

After creating, run: `scripts/audit.sh <skill-name>`

This validates:

- Correct path depth (user/\<name\> or project/\<name\>, not nested)
- Required files exist (SKILL.md, INSTRUCTIONS.md)
- SKILL.md stays lean (<30 lines, <200 words)
- YAML frontmatter present
- No forbidden files (README.md, etc.)
- Slash-command list stays in sync across SKILL.md and INSTRUCTIONS.md (when both define command docs)
- User skills are linked from a supported config skills directory

---

### `/learn edit`

**1. Identify skill:**

- User specifies skill name, OR
- Use `AskUserQuestion` to ask which skill to edit

**2. Read current state:**

- Read SKILL.md + INSTRUCTIONS.md
- Re-read this file's Principles section to verify conventions

**3. Understand the change:**

Edits vary in scope:

- **Quick fix** (typos, small tweaks) → edit directly
- **Workflow improvement** → ask what's not working, what outcome is wanted
- **Tool/framework upgrade** → when a wrapped tool has breaking changes (major version), may require multiple edit passes

For non-trivial changes, ask clarifying questions first.

**4. Verify tool behavior:**

Don't assume how tools work. If unsure about current APIs, flags, or behavior, use `WebSearch` to check official docs before editing.

**5. Edit while ensuring:**

- Architecture matches patterns in this file
- Principles are followed (concise, tool-wrapping, proper triggers)
- No unnecessary content added

**6. Run linter:**

After editing, run: `scripts/audit.sh <skill-name>`

---

## Principles

### Skills Wrap Tools

Skills provide specialized knowledge for existing tools—they don't reimplement functionality.

**Good skill:** "Use `pdfplumber` to extract text, `pikepdf` for manipulation, here's how to handle edge cases..."

**Bad skill:** Reimplements PDF parsing in Python.

Most skills should reference: CLI tools, libraries, APIs, or file format specs.

### Concise is Key

Context is shared. Only add what the agent doesn't already know.

- Prefer examples over explanations
- Challenge each paragraph: "Does this justify its token cost?"

### Degrees of Freedom

Match specificity to task fragility:

| Freedom | When                      | Format                            |
| ------- | ------------------------- | --------------------------------- |
| High    | Multiple valid approaches | Text instructions                 |
| Medium  | Preferred pattern exists  | Pseudocode, configurable scripts  |
| Low     | Fragile, error-prone      | Specific scripts, exact sequences |

### Description is the Trigger

The YAML `description` field determines when the skill activates. Put ALL "when to use" info there, not in the body.

```yaml
description: Work with PDFs using pdfplumber and pikepdf. Use for extracting text, merging, splitting, form filling, or any PDF manipulation task.
```

### Don't Include

- README.md, CHANGELOG.md, INSTALLATION.md
- User-facing documentation
- Setup/testing procedures
- Anything not needed by the agent to do the job

---

## Structure Patterns

See `references/patterns.md` for progressive disclosure patterns when skills get complex.
