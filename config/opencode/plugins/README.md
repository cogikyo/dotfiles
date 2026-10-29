# OpenCode V2 plugins

This guide covers the local plugins for OpenCode V2.
OpenCode discovers immediate plugin directories and loads their server and TUI entrypoints from source.
`opencode.json` owns agents and permissions; `cli.json` owns terminal settings and built-in plugin exclusions.
The sidebar shows usage, Markdown Context, lanes, Mail, then browser; claims that share a placement render in plugin directory-name order.
Run `bun run check` from `config/opencode` for type, lint, and format checks.
For renderer checks, `uv run plugins/capture.py SESSION [--dir DIR] [--seconds N] [--keys TEXT] [--size COLSxROWS] [--out FILE]` captures a real TUI screen in a pty; offscreen fixtures can miss host render nodes.
`dctl update bun` installs plugin dependencies from `config/opencode/bun.lock` with `bun install --frozen-lockfile --ignore-scripts`.
Setup's `extra-bun` item does the same when a locked package for this platform is missing from `node_modules` or has the wrong version.
Saves apply live: the TUI watches plugin sources, discovery directories, and `cli.json`, and the server watches its plugin modules.
After removing or renaming an export another plugin imports, check `opencode.log` for `not found in module`; long-running TUIs can keep failing to load the plugin until `/reload`.
Plugin types under-declare the host; some extra fields, such as `titleView` on `dialog.select` options, still reach the native component at runtime.
The server runs one copy of each plugin per loaded location, and a copy's `ctx.event.subscribe` can receive other locations' session events, so a handler that acts on a session should first check that `event.location` matches `ctx.location`.
Execution and inbox events can omit `event.location`; `compact/server.ts` handles those with state that is safe to update in any copy.

Drive, sidebar, and retitle keep named slash and palette commands in `mode: "global"` layers so they remain available during autocomplete.
Their four leader binds use separate base-mode inline layers, so they do not fire in dialogs and have no shortcut hints in the palette.

## Lanes

`lanes/server.ts` extends native `subagent` with a reusable `lane` name and applies `ROUTING.md` routes.
Inputs are `agent`, `description`, `prompt`, and optional `model`, `sessionID`, `background`, and `lane`.
Explicit models use `provider/model#variant`; frontmatter routes keep `provider/model [variant]`.
The `claude` pool picks an Anthropic account from usage caches, keeps an open account on resume, and reports stale-cache or fallback notes in results.
Background children return immediately and send completion, cancellation, or error to the parent's inbox.

The `lanes` tool lists children, including closed ones; `subagent` reuses a lane name to steer or resume one.
`lanes_close` takes `{ lanes }` and returns `closed <name>` or `skipped <name>: <reason>` per name; it never closes a running child.
Closing moves the name from `lane` to `closed: { lane, by, at }` in child metadata and appends a session rule that denies every permission.
`subagent` refuses the `sessionID` of a closed child; naming the old lane starts a fresh child.

`lanes/tui.tsx` appends to `sidebar.content` and adds `/lanes` with Steer, Interrupt, Continue, and Open actions.
Continue sends `Continue.` directly without a parent completion notice.
Click a non-running named lane's status icon or use `/lanes-close` to close lanes through the `lanes` RPC in `lanes/rpc.ts`, so the TUI and `lanes_close` share the server implementation.
The sidebar hides closed children.
Lane names live in child session metadata, and plugin KV entries `child/<parent>/<child>` track children.
Only Collab delegates, with one level of children.

## Mail

`mail/server.ts` exposes `mail` with `to`, required `kind` (`info` or `ask`), `subject`, `body`, and optional `replyTo` for cross-session handoffs, including across directories.
`to` accepts a top-level session ID or an exact title with its tag stripped; missing or ambiguous title matches fail and list up to eight close matches when available.
`replyTo` must name a mail message this session received from the target.
Subjects are single lines of at most 80 characters, and bodies are plain text of at most 4000 characters.
Callers and targets must be open, non-`REFUSALS`, top-level sessions, as defined by `mailable()` in `mail/mail.ts`.
Title matching uses the same rule; self and missing targets are refused.
Mail comes from the sending session's agent and carries no user approval.
`info` expects no reply, and acknowledgment-only replies are forbidden.
`ask` expects a `mail` reply with `replyTo`, even when the answer is that the agent can't act or is waiting on the user.

Mail waits for the target's next turn; a running target gets it at its next step.
At setup and every 60 seconds, the server wakes idle, mailable targets with mail parked for at least 3 minutes.
A staged revert blocks the wake, as does a failed last turn that ended after the mail parked.

`mail/tui.tsx` shows only open items: unread mail here, an ask owed, mail waiting in the target, or a sent ask with no reply yet.
Delivered info and answered asks leave the sidebar.
An answer counts only when its `replyTo` names the ask and its `from` and `to` reverse the ask's endpoints.
Owed asks and delivered sent asks also leave when the peer is closed, `REFUSALS`, or deleted.
The section hides when nothing is open.
Incoming rows show the kind glyph and age; sent rows have a closed envelope while waiting in the target and an open envelope while awaiting an ask's reply.
Rows open the peer session when clicked.
The sidebar refreshes on inbox enqueue, delivery, and cancellation, `session.deleted`, `server.connected`, and successful `mail` calls, debounced by 300 ms.

`mail/rpc.ts` exposes `ledger` and `parked`; `mail/store.ts` reads the OpenCode DB read-only because the TUI initially loads only the newest 20 messages per session.
The queries depend on host tables `session_v2`, `session_message`, and `session_inbox`, including their JSON message and metadata shapes; review these after host updates.
Received history is filtered by `to` and sent history by `from`, so forks do not inherit a parent's mail.
This plugin has no key or separate state file.
See [Byline](#byline) for transcript boxes and [Hyprd](#hyprd) for desktop notices.

## Drive

`drive/server.ts` sends each permission ask in an armed session tree to `openai/gpt-6-sol-fast` before asking the user.
The reviewer can allow, deny with a reason, or leave an ask waiting for the user; reviewer failure also leaves the ask.
Configured denials remain denied, and the plugin hides `question` while armed.
`drive/tui.tsx` adds `<leader>d`, `/drive`, and `/drive-log` for recent decisions.
It marks an armed tree on the native sidebar title with a steering-wheel glyph, blue text, and the title shimmer while any session in the tree runs.
V2 has no slot for that title, so `drive/title.ts` polls every 250 ms from the `session-pane` row, finds the node by its `_rename` and `pendingTitle` fields, and intercepts its `rename` setter to keep the shimmer on.
When that title is hidden or not found, such as a closed sidebar, a terminal or panel pane, a child session, or an upstream change, a DRIVE badge before `prompt.footer.status` shows instead.
Arming also reviews pending permission asks and cancels pending question forms in the tree.
Armed state is plugin KV `armed/<root>`, and decisions go to `${XDG_DATA_HOME:-~/.local/share}/opencode/drive.jsonl`, rotated to `.1` after 2 MiB.

## Compact

`compact/server.ts` exposes `compact` with `brief` and `reason`, queuing compaction after the current turn with a handoff brief for the summary.
Its root-session reminders at 150K and 250K tokens tell the agent to wait for a finished-work boundary and user approval; armed drive runs commit finished work and compact without asking.
At 300K tokens, it requests root-session compaction at a step boundary, even mid-task, without creating a handoff brief.
Thresholds use the last step's `input + output + reasoning + cache.read + cache.write`.

After root compaction from any trigger, the plugin waits for a successful idle session and queues a synthetic `recommended` follow-up.
If drive is confirmed armed, the follow-up instead asks the agent to restate, decide open items, and continue in the same turn; a drive lookup failure uses the normal follow-up.
User input, including queued input, suppresses the follow-up, as does a superseding compaction or a failure or non-shutdown interruption between the compaction request and idle.

Summary instructions cover user wording, approvals, remaining work, file ownership, and stored handoff briefs, plus the delegate's objective, scope, limits, and checks for child sessions.

## Usage

`usage/server.ts` polls provider usage once per minute, subject to adapter intervals and backoff.
`usage_status` only reads the local cache and reports capacity, freshness, and reset timing.
`usage/tui.tsx` prepends root-session usage rows to `sidebar.content`; click a provider heading to refresh it.
OpenAI and OpenCode Go use V2 integration credentials, Anthropic uses the fixed Claude directories, and xAI uses Grok CLI auth.
Cursor exchanges the bridge's API key at `~/.local/share/dotfiles/cursor-api-key` for a dashboard token and shows `no auth` only when that file is missing.
Caches are `${XDG_CACHE_HOME:-~/.cache}/opencode/usage-sidebar/<provider>.json`.
Locks are `${XDG_RUNTIME_DIR}/opencode/usage-sidebar-<provider>.lock`, or `/tmp/opencode-<uid>/usage-sidebar-<provider>.lock` without that variable.

## Anthropic

`anthropic/index.ts` signs and shapes subscription requests for Trend and Cogikyo with separate credential and beta-exclusion state.
It uses local credential refresh and vendored request-shaping modules from `opencode-claude-auth` 2.2.1.
See [account state and login](anthropic/README.md) before changing auth code.
This plugin has no tool, key, or TUI slot.

`anthropic/clearing.ts` adds `clear_tool_uses_20250919` to root and child primary requests: trigger at 150K input tokens, keep the latest five tool uses, and clear at least 50K tokens per batch.
Compaction, title, and generate requests receive no clearing policy.
Results from `mcp_Question`, `mcp_Subagent`, and `mcp_Skill` are excluded when those names appear in the request's tools or tool-use history.
These wire names must track the prefix rewrite in `anthropic/claude-auth/transforms.js`; a mismatch silently removes their protection.
Clearing invalidates cached prompt prefixes, so the 50K minimum batches cache rebuilds.
Anthropic reports usage after clearing (probed live 2026-10-15), so native compaction and the compact plugin's ceilings measure the cleared size.
The plugin logs `anthropic: tool results cleared` with `sessionID`, `toolUses`, and `inputTokens` because V2 discards `applied_edits`.

## Guard

`guard/server.ts` denies recognized shell Grok launches, OpenCode API calls, and credential or database reads.
It denies recognized `cd`, `pushd`, and `popd` commands, including inside substitutions and `bash -c`.
It also rejects broad home or secret-ancestor `grep` and `glob` roots and checks commit-message shape for agents outside the read-only set.
For agents outside the read-only set, `rm` requires literal absolute paths whose parents resolve inside `/tmp/opencode/`; use `trash -- <path>` for other deletion.
For all agents, `systemd-run` requires `--collect` (`-G`) or a `CollectMode=` property; use `-p CollectMode=inactive` to retain failed units deliberately, accepting hyprd health alerts.
For every agent and tool, it rejects inputs naming nonexistent default XDG user dirs relocated by `user-dirs.dirs`, except fields used to find or fix stale references, and lists the configured dirs in the denial.
Use `x` for X search, `sessions` for history, and `session_create` for new sessions.
Set the shell tool's `workdir` parameter; use absolute paths, `git -C`, or a separate call for other directories.
V2 resolves each `cd` argument against `workdir`, so chained relative changes can cause false external-directory prompts.
The `context`, `compaction`, and `generate` hooks add this rule to the shell tool's description for each request.
Agents outside the read-only set also learn that the shell is zsh, where an unquoted `$var` does not word-split; read-only commands run under `bash -c` in cage.

For `review/*`, `scout/*`, `verify/source`, and `verify/web`, the guard allows only `shell`, `read`, `grep`, `glob`, `webfetch`, `websearch`, `skill`, `sessions`, `x`, and `diagram`; agent permissions can restrict these further.
Code Mode `execute` is excluded, and `verify/web` denies all shell calls.
Each read-only shell runs through [cage](../../../cmds/cmd/cage/README.md) offline, with writes limited to `~/.local/state/opencode/scratch/<session-id>`.
`$OPENCODE_SCRATCH` names that child's directory; `tmp/` and `cache/` are fully mutable, while the root permits creation and in-place writes without deletion, rename, truncation, or symlink creation.
The pattern checks are guardrails; cage supplies the kernel sandbox and trusts the scratch path the guard computes.

`paths.ts` supplies one hide inventory for cage and native `read`, `grep`, `glob`, and file-backed `diagram` checks.
Native checks walk symlinks hop by hop and refuse hidden paths, `/proc`, `/sys`, `/dev`, and every session's scratch `tmp/` or `cache/`; recursive searches also refuse roots containing protected paths.
Unlisted secret stores remain readable, so the inventory needs maintenance.

Read-only `webfetch` is replaced by `fetch.ts`: public HTTP(S) on ports 80/443, with connect-time DNS vetting, connected-peer checks, and checks on every redirect.
It rejects non-public addresses and local interface prefixes, including locally assigned public ranges.
HTML conversion runs in a killable Worker (`convert.ts`) using `html.ts`, a verbatim copy of OpenCode v2.0.29's `packages/core/src/tool/html-markdown.ts` that requires `htmlparser2` 8.0.2.

Read-only shell and `x` calls append request and return records to `~/.local/state/opencode/scratch/runs.jsonl`, keyed by call and session IDs.
Return records contain tool status, not process exit codes; the index is not proof of command success.
`/tmp/opencode` is a disk-backed bind mount of `~/.cache/opencode-tmp` and survives reboots.
User tmpfiles (`config/user-tmpfiles.d/opencode.conf`) age out unused entries in `/tmp/opencode` and caged scratch after 3 days; the index is exempt.
There is no tool, key, or TUI slot for the index.

## Retitle

`retitle/server.ts` renames a titled root after two new user messages of at least three words and a successful turn, at most once per five minutes.
Auto-retitle skips any tagged title, and manual retitle refuses a `REFUSALS` title (see [Sessions](#sessions)).
It asks the configured `title` agent's model for a two-to-four-word title naming the session's main work, keeping the current title when it still fits.
It generates the title without a temporary child session.
A title containing letters but no lowercase letters stays pinned.
Counters live in memory, and the resulting title lives on the session.

Context comes from stored history through `ctx.session.context`, using the first two qualifying user messages, up to four spread samples, and the latest three.
After compaction, it includes the completed summary's first nonempty section and samples the user messages since that summary.
The trigger and message context skip non-user items and messages under three words after removing image markers.

Generated titles are Title Case and at most 36 characters, so they fit on one sidebar line; words that already contain a capital keep their casing.

`retitle/tui.tsx` adds `<leader>s` (`/retitle`) to retitle the root now with the same stored context, omitting the current title from the prompt.
A pinned title gets the new name in caps.
`<leader>shift+r` (`/caps`) pins the title by writing it in caps, or unpins it with a fresh Title Case title.
Clicking the sidebar title copies the shown session's `Title (ses_…)` to the clipboard via OSC 52.
V2 has no sidebar-title slot, so the plugin hooks the native title node.
If that lookup breaks, one warning toast appears; retitle keybinds still work.

## Hyprd

`hyprd/tui.ts` loads the notification and pane-context plugins together.
`hyprd/notify.ts` sends permission, question, complete, idle, error, lane, and viewed events to `/tmp/hyprd.sock` using the TUI's Kitty pane identity.
It needs `KITTY_PID` and `KITTY_WINDOW_ID` and follows child requests to their root.
Except for mail, desktop notices are limited to open sessions outside the focused view.
Successful children send a `lane` notice with `<lane> done`; one-shot children use their agent name, such as `verify/test done`.
Root turns skip completion notices when no user message started the turn and all lane notices that woke it report `completed`, because each lane already sent its own notice; if the TUI has not loaded the start of the turn, the notice is sent anyway.
User turns, turns woken by failed or cancelled lanes, and turns woken only by other notices still send completion notices.
Notice text uses the first line containing a letter or digit, skipping Markdown rules such as `---`.
Mail sends a `lane` notice with recipient, sender, and subject, unless the target is displayed and focused.
Only TUIs with both Kitty pane identifiers claim `$XDG_RUNTIME_DIR/opencode-mail/<mailID>` with an exclusive file create so at most one sends the notice; without `XDG_RUNTIME_DIR`, the directory is under the system temp directory.
Claim errors count as not claimed and suppress that TUI's notice.
Other notification deduplication and timers live in memory.

Every 60 seconds, the plugin checks each open root and its lanes for pending requests, with a separate reminder clock per root.
With tabs enabled, open roots are the roots of tab sessions; without tabs, only the displayed root is open.
Permission and question reminders repeat every five minutes on any open root until answered.
Only the displayed root gets idle reminders every 15 minutes, unless its title starts with a closed tag or its tree has running work or pending non-synthetic items.
Each root's clock pauses while Hyprland reports the screen locked and resets while the TUI is focused on that root, leaving other roots' clocks unchanged.
Clocks are dropped when their roots are no longer open.

`hyprd/context.ts` publishes the displayed root session to `$XDG_RUNTIME_DIR/opencode/tui/<TUI-pid>.json` once per second when the selection changes.
It requires `XDG_RUNTIME_DIR` and `KITTY_WINDOW_ID`, records `session` or `null` with the process `start` time from `/proc/self/stat`, and removes the file on disposal.
It creates the directory with mode `0700` and the file with mode `0600`, then publishes by atomic rename.
Hyprd matches the record to a discovered TUI pane and checks the process start time before using its session for relaunch.
There is no tool, key, or TUI slot.

## Sessions

`sessions/server.ts` exposes the read-only `sessions` tool over `${XDG_DATA_HOME:-~/.local/share}/opencode/opencode.db`.
Queries return at most 20 rows and 800-character excerpts, with a 30-second subprocess deadline.
Use `list`, `text`, `reasoning`, `tools`, or `part` before raw `sql`.
Assistant item addresses are `<message ID>#<index>`; bare message IDs read non-assistant text.
Raw queries accept one bound `SELECT` or `WITH` over `session_v2`, `session_message`, `project`, and `sqlite_schema`, with `json_each` and `json_tree` allowed.
Message reads must use an indexed `session_id` bound.

A title tag is an uppercase prefix such as `[CLOSED,FIX] ` or `[REFUSALS] `, defined in `sessions/tags.ts`.
It holds an optional state (`CLOSED`, `ARCHIVED`, or `RESEARCH`) first, then flags (`FIX`, `ISSUES`, `REVIEWME`, `TODO`, or `REFUSALS`), with at least one name.
Only tags with a state count as closed; unknown names or misplaced states make a prefix unrecognized.
Writes deduplicate flags in the order above; see [done's tag criteria](../skills/orchestration/done/SKILL.md#tags) to choose them.

`sessions/query.ts` filters session and message views to omit `REFUSALS` sessions and their descendants through child or fork links; the TUI still shows them.
Successful queries append the store-wide hidden count when nonzero; direct session and part lookups refuse hidden targets.
Visible text results and query errors replace exact hidden-title copies with `[hidden by refusals]` before clipping or paging.
Plain-title copies are included only with at least two words and 10 characters, excluding default titles; matching is case-sensitive and covers up to two JSON-escape layers.

`sessions/server.ts` exposes `tools.opencode.session_tag` in Code Mode with a required lowercase `state` and optional `flags`.
It replaces the tag, preserves an existing `refusals` flag and the rest of the title, and skips an unchanged title.
It refuses another session flagged `REFUSALS` without echoing its title.
Only top-level sessions can call it or be tagged; `sessionID` defaults to the caller, and a missing title becomes `New session - <ISO time>`.

`sessions/tui.tsx` binds `ctrl+o` to Open and `<leader>r` to Sessions.
`<leader>shift+return` sends `/done` with no arguments to the root session.
`/archive` and `/research` set the root session's state to `ARCHIVED` or `RESEARCH` without running `/done`, keeping its flags, then open Home in its directory so the next prompt starts a new session.
`/refusals` toggles `REFUSALS` on the root session, keeping its state and other flags, and stays in the session.
`cli.json` sets the native `open.menu` and `session.list` bindings to `none`; `/open` and `/sessions` still open the native dialogs.
The plugin omits native pins, quick-switch hints, and direct opening by pasted `ses_` ID.

Without a search, Open ranks sessions needing permission or question input together with open, non-`REFUSALS` sessions with parked mail, then other open sessions, then titles starting with a closed tag, newest first within each rank.
Sessions groups by date, newest day first, and ranks permission or question input first within each group, except closed sessions always come last even when they need input.
Closed titles are muted, with `RESEARCH` titles in a faint scout blue, unless selected or current; rows retain pending-input and running markers.
In Open, an envelope after a session title marks parked mail for open, non-`REFUSALS` sessions.
The marker and mail ranking share one `mail.parked` RPC request on opening, refreshed on inbox events and `server.connected`, instead of querying each session's inbox.

- Open shows up to eight recents plus projects, expanding the list to include all sessions in its first rank; typing searches the full loaded session list.
- Right arrow on a Git project opens its worktrees, including a `New worktree…` row; left arrow returns to Open.
- Sessions lists only the current directory's sessions and offers `ctrl+d` to delete and `ctrl+r` to rename; Open covers all projects.

The plugin relies on host behavior beyond the published types: extra option fields such as `titleView` and `gutter`, selection preservation with a defined `current`, the `modal` mode after dialog replacement, the focused `FILTER` input, and a home route carrying `location`.
If the host stops forwarding the extra row fields, rows degrade to plain titles without muting or status markers.
The other host assumptions govern search expansion, selection, and navigation and need review after an OpenCode update.

## Launch

`launch/server.ts` adds `session_create` to the `opencode` Code Mode namespace, so agents call `tools.opencode.session_create` through `execute`.
Inputs are `directory`, `title`, and optional `prompt` and `agent`; the directory must be absolute or start with `~` and must exist.
It creates a top-level session with session metadata `origin` set to the caller's session ID, then sends `prompt` as the first message to start a turn.
Without `agent`, the session uses the configured default primary agent; a named agent must be a visible primary agent at that directory.
Only top-level sessions can call it, and the new session is not a lane and is not armed for drive.
V2 gives plugins no permission-ask path, so the user's request in chat is the approval; the guard's `opencode api` denial points here.
If the prompt fails, the error names the created session, so the agent does not retry and create a duplicate.
There is no persistent state, key, or TUI slot.

## Diagram

`diagram/server.ts` exposes `diagram` for display-cell geometry and connectivity checks on documentation diagrams and trees.
Pass `text`, or `file` with `start` and `end`; optional `prefix` and `width` describe the host layout.
File mode requires a project session directory and rejects paths outside it; pass text for external layouts.
There is no persistent state, key, or TUI slot.

## X search

`x/server.ts` exposes `x` with a self-contained `brief` and runs Grok CLI native X search with a ten-minute deadline.
Each call starts in an empty temporary directory under `~/.local/state/opencode/x/`, removed when the call finishes.
Fixed flags disallow local tools and add explicit denials, disable web search and child agents, and use permission mode `default`; Grok's own sandbox is off, and this call does not run in cage.
The brief is NFKC-normalized; control and format characters are rejected except newline and tab.
Bare `@handle` text becomes `X user handle <name>`, other `@` text is rejected, and lines starting with `/` or `!` are rejected before the `X research brief:` prefix is added.
Scouts and source/web verifiers can call `x`; Collab keeps it for `/x` and delegates other X research.
Returned claims remain untrusted evidence.
The plugin rejects near-duplicate briefs within a session and keeps that history only in memory.
There is no key, TUI slot, or separate state file.

## Browser

`browser/server.ts` exposes `chrome-devtools_*` tools from `chrome-devtools-mcp` 1.7.0 with one isolated Chromium process per session.
It opens headless at 1920×1080 under an armed drive root and closes the MCP process on execution success, failure, interruption, or session deletion.
URL-bearing calls accept HTTP(S) or `about:blank`; MCP launch patterns restrict network schemes.
The tool catalog lives in plugin KV `catalog/1.7.0`.
`browser/tui.tsx` places browser workspaces after `sidebar.content`, subscribes through `hyprd`, and focuses a workspace when clicked.
It needs `/usr/bin/chromium`, `npx`, and `hyprd` for headed workspace controls; there is no key or separate plugin state file.

## Statusline

`queue/tui.tsx` replaces the queued-prompt dock above the prompt with one row per queued prompt, at most three and then `+N more`, and a count on the right; a click opens the queued prompts dialog.
It hides the native dock only after it matches the dock's shape and `N queued` label; when that match fails, both docks show.

`sidebar/tui.tsx` adds `<leader>a` and `/sidebar`, which move the session sidebar to the left and back; the choice persists in plugin storage.
V2 has no sidebar-side setting, so it polls every 250 ms for the renderer node with id `session-pane` and reverses its parent row while the 42-cell sidebar is the sibling.
That id and width are upstream internals in `session-frame.tsx` and `ui/layout.ts`; when they change, the toggle silently stops moving the sidebar.
`<leader>a` replaces `agent.list`, which is unbound in `cli.json`; `/agents` and `shift+tab` still reach agents.

`statusline/tui.tsx` always replaces `prompt.footer.status` with directory and Git status, so the native spinner and `esc interrupt` hint never show.
It appends a context-pressure bar to `prompt.footer`, measured against the model input or context limit minus the compaction buffer, capped at 300K for root sessions.
The gauge starts with ㊌ (water) while idle and ㊋ (fire) while the session runs.
While running, the fire glyph fades between the gauge color and up to 35% red over a two-second cycle; a wave sweeps only the dots, and a newly filled dot flashes.
V2 has no slot on the prompt's model row, so the bar is absolutely positioned two rows up into the prompt box; a prompt layout change upstream can misplace it.
It replaces the built-in prompt footer, disabled in `cli.json`, with running subagent and shell counts and context size, without session cost.
It replaces the built-in sidebar footer, also disabled, with the OpenCode version line.
A linked worktree shows its main repository path and a worktree glyph in place of the branch glyph.
While the session runs, `statusline/working.tsx` widens the prompt's left border to a half cell and pulses it in a brightened agent color.
The border pulse, fire fade, and gauge wave stop when `animations` is off in `cli.json`; the wide border stays.

`statusline/delivery.tsx` adds Ctrl+Enter to toggle Enter between steer and queue; `prompt.queue` is unbound (`"none"`) in `cli.json`, so Ctrl+Enter only toggles.
In queue mode, Enter dispatches `prompt.queue` only while the session runs, the prompt is in normal mode, and the prompt has text.
Otherwise Enter behaves natively, including empty Enter promoting the next queued prompt to steer.
The binding targets the prompt textarea in base mode, so autocomplete, composer pickers, dialogs, and permission prompts keep Enter.
Typed TUI slash commands that accept arguments show the native "This prompt cannot be queued" toast in queue mode while running.
The mode is shared across all sessions in one TUI and resets to steer on restart.
While a session is open, the input's top-right corner shows 󰒊 (`nf-md-send`, U+F048A) in muted text for steer or 󱅣 (`nf-md-send_clock`, U+F1163) in yellow for queue, unless typed text or the placeholder reaches it.

`statusline/toasts.tsx` hides the core top-right toast while the sidebar footer is on screen and shows it in that footer instead; clicking the card runs the toast action or dismisses it.
The card matches the native border color to theme feedback colors to choose a variant icon.
It has at least three rows, centers single-row content, and truncates title and message lines at the end with `…` instead of wrapping.
A footer row shows `+N more` and the action when present.
The countdown bar assumes the native 5000 ms default and hides if the toast lasts longer; hover is forwarded to pause the native timer and the bar.
It finds the core toast by its layout shape before each frame's layout pass, so an unrecognized upstream toast or a hidden sidebar falls back to the stock top-right toast.
Git polling and configuration caches live in memory; this plugin has no tool or state file.
Drive owns its own toggle and badge.

## Byline

`byline/tui.tsx` hides primary-agent labels, currently Collab, on the prompt model row and assistant message footers, and gives the prompt model name the agent color.
Non-primary agents keep their labels; assistant footers also keep the native label below 28 terminal columns.

Each successful subagent notice and Collab's `---`…`---` reply block form one indented box, tinted in the lane's role color with a flush right edge bar.
The rules are hidden and text wraps inside the box.
The notice puts the subagent's description on the left and `icon Scope / icon Agent · model/variant · tokens` on the right, truncating the description first when space is short.
Lane icons, colors, model names, and token counts use the same source as the Lanes sidebar.

New lane notices reveal the whole row in about 1.2 s: build forge, review scan, scout ping, or verify decode.
The icon then breathes while Collab replies, then settles.
Newly delivered mail opens and types its header behind a pink cursor in up to about 2.6 s, ending with a blink and postmark stamp.
The body fades as typing ends; the icon then breathes until the delivering turn ends or its joined reply box closes.
Newly sent mail types its header, then closes and seals in at most about 1.1 s without breathing.
Both mail reveals play only for fresh items; received freshness uses delivery time, so long-parked mail still reveals on arrival.
Set `animations` to `false` in `cli.json` to keep these effects static.

Failed and cancelled lanes show a red `✗` and the state word without a box.
User messages and question-tool blocks end the current lane or mail box.

Delivered mail and the sender's completed `mail` tool call render in pink boxes with the peer, subject, and body.
The header shows the kind glyph, keeps `<kind> · <time>` on the right, and cuts long subjects with `…` instead of wrapping.
After parked mail is delivered at turn start, a reply whose first Markdown part starts with `---` gets its own pink commentary box, even with the user's prompt between the mail and reply.
Question-tool rows end the commentary slot.
Mail delivered mid-turn still joins a `---`…`---` reply block only when no user message comes between them.
At low verbosity the sender's `mail` call folds into `+ N tools`; the host's `grouping/session.ts` exempts only `question` from tool grouping.

Each completed question-tool block becomes one rounded box in the theme's accent color, titled with the question count.
Each question shows its header, text, and all options with descriptions.
Picked options get `✓`; the recommended option gets `★`, green when picked and orange otherwise.
Typed answers get `✎` and keep their paragraphs; dismissed forms show `Dismissed`.
Pending questions and the live form stay native.
Unrecognized renderer layouts leave the plain transcript in place.

## Code blocks

`code-blocks/tui.ts` renders supported fenced code blocks as padded cards with a raised background and a muted top-right language label.
Cards always use Vagari highlighting, independent of the TUI theme.
Converted nvim-treesitter queries use private `vagari.<ft>` parsers for cards; the hand-maintained `sql/` parser is public because OpenCode has no SQL parser.

Add a parser folder at `config/opencode/tree-sitter/<ft>/` with `highlights.scm` and `tree-sitter-<ft>.wasm`; optional `manifest.json` sets `filetype`, `wasm`, and fence `aliases`.
Sync needs `~/vagari/nvim`, the nvim-treesitter `main` checkout at `~/.local/share/nvim/lazy/nvim-treesitter`, headless Neovim, and the installed OpenCode source in the `src` cache (`src get github.com/anomalyco/opencode@<version>`).
From `config/opencode`, run `bun plugins/code-blocks/sync.ts` after changes to Vagari, nvim-treesitter queries, or the host parser catalog.
Sync regenerates queries, manifests, and resolved Vagari styles; it replaces `tree-sitter/` but keeps every parser folder whose manifest sets no `filetype`, and fails if such a folder shares a name with a generated one.

## Workflow

`plugins/workflow/` registers a renderer for `workflow` fences and draws a card with an automatic dependency graph.
Graph nodes are numbers in the owner's color; lane steps are role-color pills, and gates use `◆N`.
Rows lead with the scope icon and scope name in the role color, then the number, a square lane pill for lane steps, and the headline.
A headline's main idea, before any `: `, takes the owner's color, bold for lanes and italic for one-shots; the rest is regular text.
Delegated rows show the model family flush right in the role color, bold for lanes and italic for one-shots.
The renderer reads `ROUTING.md` and shows a brain glyph and the variant left of the model only for a variant departure; no brain appears when the file is missing.
`plugins/lanes/routing.ts` holds route parsing shared by the lanes server and the card.
Gate checklist items get a distinct watching-for glyph; `if:` steps show an `only if` row.
Only the summary wraps; other rows clip, and bad specs show red error lines.
From `config/opencode`, run `bun plugins/workflow/preview.ts [file] --width N` to print the card with ANSI color.

## Bullets

`bullets/tui.ts` uses `•` for depth-0 unordered list markers and `◦` at deeper levels.
Ordered lists keep their native markers.
V2's Markdown plugin hook only replaces fenced code-block rendering, so list markers need a patch.
An opentui upgrade can break this private `getListItemInputs` override; the plugin throws on load if the method disappears.

## Markdown context

`context/tui.tsx` appends Markdown Context to `sidebar.content` with root and project instructions, the agent prompt, skills, and observed Markdown loads.
Rows before the latest completed compaction use a compacted marker; the list is a view of recorded loads rather than proof that every body remains in active context.
Click the heading to fold the list or a row to open its file with `hyprd edit`.
The plugin does not change loaded context and has no tool, key, or state file.

## Shell environment

`env.ts` restores or clears `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME` from supplied `HOST_` variables before shell creation.
It is a standalone source module, not a plugin directory, so verify its load path when using an isolated launch environment.
It has no tool, key, TUI slot, or state file.
