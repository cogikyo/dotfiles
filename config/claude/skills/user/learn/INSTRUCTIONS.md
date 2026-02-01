# Learn - Skill Creator

Skills extend Claude with specialized knowledge for using tools, workflows, and domain expertise.

## Commands

### `/learn new`

**1. Ask scope:**

- **user** → `~/.dotfiles/config/claude/skills/user/<name>/`
- **project** → `~/.dotfiles/config/claude/skills/project/<name>/`

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

**5. Link (once per new skill):**

```bash
~/.dotfiles/config/claude/skills/link.sh user  # or: link.sh project <name>
```

Symlinks point to directories—edits sync automatically.

---

### `/learn refine`

**1. Identify skill** - ask or infer from context

**2. Read current state** - SKILL.md + INSTRUCTIONS.md

**3. Gather feedback:**

- "What's not working?"
- "What should change?"

**4. Edit**

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
