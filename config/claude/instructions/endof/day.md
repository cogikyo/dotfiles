# End of Day - Daily Impact Capture

Analyze the day's git commits and add them to the current week's impact file.

## Steps

1. **Determine the date** - Use today's date, or if user specifies a different day (e.g., "monday", "yesterday"), use that.

2. **Find the current week's impact file** - Look in `~/Documents/life/work/impact/2026/MM/DD.md` where DD is the last workday of the current week.

3. **Gather commits from all repos** by the user (author: cullyn or cullyn@trendcapital.com):

   **Frontend:**
   - `~/LeadPier/leadpierui`

   **Backend services (all git repos in):**
   - `~/LeadPier/services/*`

   Run for each repo:
   ```bash
   git log --author="cullyn" --since="YYYY-MM-DD 00:00" --until="YYYY-MM-DD 23:59" --pretty=format:"%h %s" --no-merges
   ```

4. **For commits found, analyze the diffs:**
   ```bash
   git show --stat <commit-hash>
   git show <commit-hash> # for detailed diff if needed
   ```

5. **Handle date discrepancies** - Commits may show different dates due to rebasing or merging. If commit dates don't match the expected day, ask the user for clarification rather than assuming.

6. **Skip trivial chore commits** - Ignore minor housekeeping commits like .gitignore updates, formatting-only changes, or other trivial chores unless they're part of meaningful work.

7. **Categorize and summarize** the work:
   - Group by project/service
   - Extract key changes from commit messages and diffs
   - Use format: `- feat/fix/refactor(scope): description`

8. **Update the weekly file** - Add the summary under the appropriate day header (## Mon, ## Tue, etc.)

## Output Format

Under the day header, add:

```markdown
## [Day]

**[Frontend/Backend] Project/Service Name**
- change description
- another change

**[Category] Another Area**
- changes here
```

Keep entries concise but descriptive. Group related commits together.
