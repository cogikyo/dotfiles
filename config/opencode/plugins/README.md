# OpenCode V2 plugins

This guide covers the local plugins for OpenCode 2.0.27.
OpenCode discovers immediate plugin directories and loads their server and TUI entrypoints from source.
`opencode.json` owns agents and permissions; `cli.json` owns terminal settings and built-in plugin exclusions.
The sidebar shows usage, Markdown Context, lanes, then browser; claims that share a placement render in plugin directory-name order.
Run `npm run check` from `config/opencode` for type, lint, and format checks.

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
`drive/tui.tsx` adds `<leader>d` and `/drive`, a badge before `prompt.footer.status`, and `/drive-log` for recent decisions.
Arming also reviews pending permission asks and cancels pending question forms in the tree.
Armed state is plugin KV `armed/<root>`, and decisions go to `${XDG_DATA_HOME:-~/.local/share}/opencode/drive.jsonl`, rotated to `.1` after 2 MiB.

## Usage

`usage/server.ts` polls provider usage once per minute, subject to adapter intervals and backoff.
`usage_status` only reads the local cache and reports capacity, freshness, and reset timing.
`usage/tui.tsx` prepends root-session usage rows to `sidebar.content`; click a provider heading to refresh it.
OpenAI and OpenCode Go use V2 integration credentials, Anthropic uses the fixed Claude directories, and xAI uses Grok CLI auth.
The Cursor adapter currently returns `no auth` without fetching usage.
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

`statusline/tui.tsx` replaces `prompt.footer.status` only while idle with directory and Git status, leaving native running controls intact.
It appends a context-pressure bar to `prompt.footer`, measured against the model input or context limit minus the compaction buffer.
V2 has no slot on the prompt's model row, so the bar is absolutely positioned two rows up into the prompt box; a prompt layout change upstream can misplace it.
It replaces the built-in prompt footer, disabled in `cli.json`, with running subagent and shell counts and context size, without session cost.
It replaces the built-in sidebar footer, also disabled, with the same location line plus the OpenCode version; clicking the location runs `session.move`.
A linked worktree shows its main repository path and a worktree glyph in place of the branch glyph.
Git polling and configuration caches live in memory; this plugin has no tool, key, or state file.
Drive owns its own toggle and badge.

## Markdown context

`context/tui.tsx` appends Markdown Context to `sidebar.content` with root and project instructions, the agent prompt, skills, and observed Markdown loads.
Rows before the latest completed compaction use a compacted marker; the list is a view of recorded loads rather than proof that every body remains in active context.
Click the heading to fold the list or a row to open its file with `hyprd edit`.
The plugin does not change loaded context and has no tool, key, or state file.

## Shell environment

`env.ts` restores or clears `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME` from supplied `HOST_` variables before shell creation.
It is a standalone source module, not a plugin directory, so verify its load path when using an isolated launch environment.
It has no tool, key, TUI slot, or state file.
