# End of Quarter - Quarterly Summary

Aggregate weekly Reviews into a quarterly summary.

## Steps

1. **Determine the quarter** - Based on current date or user input (Q1, Q2, Q3, Q4).

2. **Find all weekly files for the quarter:**
   - Q1: `01/*.md`, `02/*.md`, `03/*.md`
   - Q2: `04/*.md`, `05/*.md`, `06/*.md`
   - Q3: `07/*.md`, `08/*.md`, `09/*.md`
   - Q4: `10/*.md`, `11/*.md`, `12/*.md`

3. **Read the Review sections** from each weekly file.

4. **Synthesize into quarterly themes:**
   - Major projects and outcomes
   - Key metrics or achievements
   - Patterns and focus areas
   - Growth and learnings

5. **Update the quarterly file** (`~/Documents/life/work/impact/2026/Q#.md`) Review section.

## Review Format

```markdown
## Review

### Major Projects
- **[Project Name]**: What was built/achieved, impact

### Key Accomplishments
- Significant deliverable or milestone
- Another accomplishment

### Themes
- Recurring focus areas across the quarter

### Learnings
- What worked well, what to improve
```
