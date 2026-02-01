# Skill Patterns

Use these when skills grow complex enough to need structure.

## Progressive Disclosure

Three-level loading:
1. **Metadata** (name + description) - Always loaded (~100 words)
2. **INSTRUCTIONS.md** - When skill triggers
3. **references/** - As needed

Keep INSTRUCTIONS.md under 500 lines. Split when approaching.

## Organization Patterns

### Domain-Specific

When skill covers multiple domains, organize by domain:

```
bigquery/
├── SKILL.md
├── INSTRUCTIONS.md (navigation + core workflow)
└── references/
    ├── finance.md
    ├── sales.md
    └── product.md
```

User asks about sales → Claude reads only `sales.md`.

### Tool/Framework Variants

When skill supports multiple tools:

```
cloud-deploy/
├── SKILL.md
├── INSTRUCTIONS.md (workflow + selection guide)
└── references/
    ├── aws.md
    ├── gcp.md
    └── azure.md
```

### Conditional Details

Basic in INSTRUCTIONS.md, advanced in references:

```markdown
## Editing documents

For simple edits, modify XML directly.

**Tracked changes**: See `references/redlining.md`
**OOXML details**: See `references/ooxml.md`
```

## Guidelines

- Keep references one level deep from INSTRUCTIONS.md
- For files >100 lines, add table of contents at top
- Don't duplicate between INSTRUCTIONS.md and references
