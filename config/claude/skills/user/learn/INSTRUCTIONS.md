# Learn - Skill Creator

Skills extend Claude with specialized knowledge for using tools, workflows, and domain expertise.

## Commands

### `/learn new`

**1. Ask scope:**

- **user** → `~/dotfiles/config/claude/skills/user/<name>/`
- **project** → `~/dotfiles/config/claude/skills/project/<name>/`

Skills are automatically available once created—no linking needed.

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

---

### `/learn refine`

**1. Identify skill:**

- User specifies skill name, OR
- Use `AskUserQuestion` to ask which skill to refine

**2. Read current state:**

- Read SKILL.md + INSTRUCTIONS.md
- Re-read this file's Principles section to verify conventions

**3. Understand the change:**

Refinements vary in scope:

- **Quick fix** (typos, small tweaks) → edit directly
- **Workflow improvement** → ask what's not working, what outcome is wanted
- **Tool/framework upgrade** → when a wrapped tool has breaking changes (major version), may require multiple refine passes

For non-trivial changes, ask clarifying questions first.

**4. Verify tool behavior:**

Don't assume how tools work. If unsure about current APIs, flags, or behavior, use `WebSearch` to check official docs before editing.

**5. Edit while ensuring:**

- Architecture matches patterns in this file
- Principles are followed (concise, tool-wrapping, proper triggers)
- No unnecessary content added

---

## Principles

### Skills Wrap Tools

Skills provide specialized knowledge for existing tools—they don't reimplement functionality.

**Good skill:** "Use `pdfplumber` to extract text, `pikepdf` for manipulation, here's how to handle edge cases..."

**Bad skill:** Reimplements PDF parsing in Python.

Most skills should reference: CLI tools, libraries, APIs, or file format specs.

### Concise is Key

Context is shared. Only add what Claude doesn't already know.

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
- Anything not needed by Claude to do the job

---

## Structure Patterns

See `references/patterns.md` for progressive disclosure patterns when skills get complex.
