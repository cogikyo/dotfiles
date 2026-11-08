---
name: slack-table
description: Use when the user asks to copy a table to their clipboard, or wants a Slack-ready or paste-ready table.
---

# Slack table

Slack needs `text/html` with a `<table>` and TSV `text/plain` simultaneously.
`wl-copy` offers one payload per run; `clip.py` serves both.

## Copy

1. Build TSV: header row first, one row per line, cells separated by tabs.
   - Use **display-formatted** cells and the user's sort order.
   - Keep tabs and newlines out of cell text.
2. Pipe it to the script in this skill's directory:

   ```bash
   python3 clip.py < table.tsv
   ```

   Exit 0 confirms clipboard ownership; a detached child serves pastes until the next copy.

3. Verify: `wl-paste --list-types` lists `text/html` and `text/plain`, and `wl-paste -t text/html` and `wl-paste -t text/plain` read back the table.

`clip-expire` clears the clipboard after 15 minutes; copy again for later pastes.
