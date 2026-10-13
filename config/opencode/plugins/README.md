# OpenCode V2 plugins

This guide covers the local plugins for OpenCode 2.0.27.
OpenCode discovers immediate plugin directories and loads their server and TUI entrypoints from source.
`opencode.json` owns agents and permissions; `cli.json` owns terminal settings and built-in plugin exclusions.
The sidebar shows usage, Markdown Context, lanes, then browser; claims that share a placement render in plugin directory-name order.
Run `npm run check` from `config/opencode` for type, lint, and format checks.

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
The ⊗ on an idle row and `/lanes-close` close lanes through the `lanes` RPC in `lanes/rpc.ts`, so the TUI and `lanes_close` share the server implementation.
The sidebar hides closed children.
Lane names live in child session metadata, and plugin KV entries `child/<parent>/<child>` track children.
Only Collab delegates, with one level of children.

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

`compact/server.ts` exposes `compact` with `brief` and `reason`; `delivery: "queue"` runs compaction after the current turn, then the session waits for the user.
V2 gives plugins no permission-ask path, so the agent must get the user's approval in chat; in an armed drive run, commit finished work, then compact without asking.
When a compaction this tool queued completes and the `drive` RPC reports the tree armed, the plugin queues a synthetic "Continue the armed drive run from the handoff brief." so the run does not stall.
It does not continue when the compaction fails or is cancelled, when the session was interrupted (except by shutdown) or failed between the tool call and compaction, or when the execution that ran the compaction ends without `succeeded`.
The `context` hook nudges root sessions at 100K and 250K tokens; hold compaction while edits, lanes, or repairs are in flight.
Tokens are the last step's `input + output + reasoning + cache.read + cache.write`, the measure native compaction and the statusline use; V2 reports `input` without cached tokens.
The final nudge names the model's input window, or its context window when no input limit is set; server plugins cannot read `compaction.buffer`, so the text gives no exact trigger.

The `compaction` hook adds summary rules for user wording, approvals, remaining work, and path ownership, plus the stored handoff brief.
Child summaries also preserve the delegate brief's objective, scope, limits, and checks.
Each brief lives in plugin storage at `pending/<sessionID>/<inboxID>`, so settling one compaction never touches a newer brief; the hook uses the newest.
A brief is removed when its compaction fails or is cancelled, the session is deleted, or a manual compaction completes and session context shows its own compaction finished or a newer manual one completed; automatic compactions leave it in place.
At startup the plugin applies the same check to every stored brief, because terminal events missed while the server was down are not replayed.

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

## Guard

`guard/server.ts` denies recognized shell deletion with `rm`, shell Grok launches, OpenCode API calls, and credential or database reads.
It also denies recognized shell mutations for read-only agents, rejects broad home or secret-ancestor `grep` and `glob` roots, and checks commit-message shape before execution.
Use `trash -- <path>` for deletion, `x` for X search, and `sessions` for history.
These checks are guardrails rather than a shell sandbox.
There is no persistent plugin state, tool, key, or TUI slot.

## Retitle

`retitle/server.ts` renames a titled root after two new user messages and a successful turn, at most once per five minutes.
It generates a two-to-four-word title with the configured `title` agent's model, without a temporary child session.
A title containing letters but no lowercase letters stays pinned.
Counters live in memory, and the resulting title lives on the session.

Generated titles are Title Case and at most 36 characters, so they fit on one sidebar line; words that already contain a capital keep their casing.

`retitle/tui.tsx` adds `<leader>t` (`/retitle`) to retitle the root now from its last six user messages; a pinned title gets the new name in caps.
`<leader>shift+t` (`/caps`) pins the title by writing it in caps, or unpins it with a fresh Title Case title.
The sidebar title is centered; left-click retitles the shown session and right-click toggles caps.
V2 publishes no slot for the sidebar title, so a zero-size anchor in `sidebar.content` finds the native title node by its `_rename` and `pendingTitle` fields.
If a V2 update breaks that lookup, the plugin shows one warning toast and the keybinds still work.

## Hyprd

`hyprd/tui.ts` loads the notification and pane-context plugins together.
`hyprd/notify.ts` sends permission, question, completion, idle, error, and viewed events to `/tmp/hyprd.sock` using the TUI's Kitty pane identity.
It needs `KITTY_PID` and `KITTY_WINDOW_ID`, follows child requests to their root, and sends desktop notices for open sessions outside the focused view.
Idle reminders repeat every ten minutes; child completion does not produce a root completion notice.
Notification deduplication and timers live in memory.

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
Message reads must use an indexed `session_id` bound; there is no key, TUI slot, or separate state file.

## Diagram

`diagram/server.ts` exposes `diagram` for display-cell geometry and connectivity checks on documentation diagrams and trees.
Pass `text`, or `file` with `start` and `end`; optional `prefix` and `width` describe the host layout.
File mode requires a project session directory and rejects paths outside it; pass text for external layouts.
There is no persistent state, key, or TUI slot.

## X search

`x/server.ts` exposes `x` with a self-contained `brief` and runs Grok CLI native X search from `/tmp` with a ten-minute deadline.
Fixed flags disable shell, file, web-search, and child-agent tools; returned claims remain untrusted evidence.
The plugin rejects near-duplicate briefs within a session and keeps that history only in memory.
There is no key, TUI slot, or separate state file.

## Browser

`browser/server.ts` exposes `chrome-devtools_*` tools from `chrome-devtools-mcp` 1.7.0 with one isolated Chromium process per session.
It opens headless at 1920×1080 under an armed drive root and closes the MCP process when the session is idle or deleted.
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
It appends a context-pressure bar to `prompt.footer`, measured against the model input or context limit minus the compaction buffer.
While the session runs, a wave sweeps from the fire glyph through the filled dots in step with the border pulse, and a newly filled dot flashes.
V2 has no slot on the prompt's model row, so the bar is absolutely positioned two rows up into the prompt box; a prompt layout change upstream can misplace it.
It replaces the built-in prompt footer, disabled in `cli.json`, with running subagent and shell counts and context size, without session cost.
It replaces the built-in sidebar footer, also disabled, with the OpenCode version line.
A linked worktree shows its main repository path and a worktree glyph in place of the branch glyph.
While the session runs, `statusline/working.tsx` widens the prompt's left border to a half cell and pulses it in a brightened agent color.
The pulse and gauge wave stop when `animations` is off in `cli.json`; the wide border stays.
It also draws the `prompt.queue` shortcut as Mac-style glyphs, such as `󰘳 󰌑`, in the input's top-right corner, unless typed text or the placeholder reaches it.
`statusline/toasts.tsx` hides the core top-right toast while the sidebar footer is on screen and shows it in that footer instead; clicking the card runs the toast action or dismisses it.
The card matches the native border color to theme feedback colors to choose a variant icon.
It has at least three rows, centers single-row content, and truncates title and message lines at the end with `…` instead of wrapping.
A footer row shows `+N more` and the action when present.
The countdown bar assumes the native 5000 ms default and hides if the toast lasts longer; hover is forwarded to pause the native timer and the bar.
It finds the core toast by its layout shape before each frame's layout pass, so an unrecognized upstream toast or a hidden sidebar falls back to the stock top-right toast.
Git polling and configuration caches live in memory; this plugin has no tool, key, or state file.
Drive owns its own toggle and badge.

## Byline

`byline/tui.tsx` hides primary-agent labels, currently Collab, on the prompt model row and assistant message footers, and gives the model name the agent color.
Non-primary agents keep their labels.
V2 has no slot for these rows, so `byline/rewrite.ts` rewrites native text nodes in a lifecycle pass each frame.
An upstream layout change can leave the native label visible; assistant footers also keep the native label below 28 terminal columns.

## Markdown context

`context/tui.tsx` appends Markdown Context to `sidebar.content` with root and project instructions, the agent prompt, skills, and observed Markdown loads.
Rows before the latest completed compaction use a compacted marker; the list is a view of recorded loads rather than proof that every body remains in active context.
Click the heading to fold the list or a row to open its file with `hyprd edit`.
The plugin does not change loaded context and has no tool, key, or state file.

## Shell environment

`env.ts` restores or clears `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME` from supplied `HOST_` variables before shell creation.
It is a standalone source module, not a plugin directory, so verify its load path when using an isolated launch environment.
It has no tool, key, TUI slot, or state file.
