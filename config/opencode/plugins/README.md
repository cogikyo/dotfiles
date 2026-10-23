# OpenCode V2 plugins

This guide covers the local plugins for OpenCode V2.
OpenCode discovers immediate plugin directories and loads their server and TUI entrypoints from source.
`opencode.json` owns agents and permissions; `cli.json` owns terminal settings and built-in plugin exclusions.
The sidebar shows usage, Markdown Context, lanes, then browser; claims that share a placement render in plugin directory-name order.
Run `bun run check` from `config/opencode` for type, lint, and format checks.
`dctl update bun` installs plugin dependencies from `config/opencode/bun.lock` with `bun install --frozen-lockfile --ignore-scripts`.
Setup's `extra-bun` item does the same when a locked package for this platform is missing from `node_modules` or has the wrong version.
Saves apply live: the TUI watches plugin sources, discovery directories, and `cli.json`, and the server watches its plugin modules.
Plugin types under-declare the host; some extra fields, such as `titleView` on `dialog.select` options, still reach the native component at runtime.

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
V2 gives plugins no permission-ask path, so the agent offers compaction with the `question` tool and calls `compact` once the user approves; in an armed drive run, commit finished work, then compact without asking.
When a compaction this tool queued completes and the `drive` RPC reports the tree armed, the plugin queues a synthetic "Continue the armed drive run from the handoff brief." so the run does not stall.
It does not continue when the compaction fails or is cancelled, when the session was interrupted (except by shutdown) or failed between the tool call and compaction, or when the execution that ran the compaction ends without `succeeded`.
The `context` hook nudges root sessions at 150K and 250K tokens; both nudges allow compaction only after a commit lands or a task is done, never mid-task.
Tokens are the last step's `input + output + reasoning + cache.read + cache.write`, the measure native compaction and the statusline use; V2 reports `input` without cached tokens.
At or above 300K tokens, `session.step.ended` triggers root-only compaction with `delivery: "steer"` once per crossing.
It runs at the next step boundary and continues the turn without creating a handoff brief or triggering drive resume.

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

`anthropic/clearing.ts` adds `clear_tool_uses_20250919` to root and child primary requests: trigger at 150K input tokens, keep the latest five tool uses, and clear at least 50K tokens per batch.
Compaction, title, and generate requests receive no clearing policy.
Results from `mcp_Question`, `mcp_Subagent`, and `mcp_Skill` are excluded when those names appear in the request's tools or tool-use history.
These wire names must track the prefix rewrite in `anthropic/claude-auth/transforms.js`; a mismatch silently removes their protection.
Clearing invalidates cached prompt prefixes, so the 50K minimum batches cache rebuilds.
Anthropic reports usage after clearing (probed live 2026-10-15), so native compaction and the compact plugin's ceilings measure the cleared size.
The plugin logs `anthropic: tool results cleared` with `sessionID`, `toolUses`, and `inputTokens` because V2 discards `applied_edits`.

## Guard

`guard/server.ts` denies recognized shell deletion with `rm`, shell Grok launches, OpenCode API calls, and credential or database reads.
It denies recognized `cd`, `pushd`, and `popd` commands, including inside substitutions and `bash -c`.
It also rejects broad home or secret-ancestor `grep` and `glob` roots and checks commit-message shape for agents outside the read-only set.
Use `trash -- <path>` for deletion, `x` for X search, `sessions` for history, and `session_create` for new sessions.
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
The index and scratch files persist; there is no tool, key, or TUI slot.

## Retitle

`retitle/server.ts` renames a titled root after two new user messages of at least three words and a successful turn, at most once per five minutes.
Auto-retitle skips titles that start with `[CLOSED] `, which `/done` sets.
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
The sidebar title is centered; left-click retitles the shown session and right-click toggles caps.
V2 publishes no slot for the sidebar title, so a zero-size anchor in `sidebar.content` finds the native title node by its `_rename` and `pendingTitle` fields.
If a V2 update breaks that lookup, the plugin shows one warning toast and the keybinds still work.

## Hyprd

`hyprd/tui.ts` loads the notification and pane-context plugins together.
`hyprd/notify.ts` sends permission, question, complete, idle, error, lane, and viewed events to `/tmp/hyprd.sock` using the TUI's Kitty pane identity.
It needs `KITTY_PID` and `KITTY_WINDOW_ID`, follows child requests to their root, and sends desktop notices for open sessions outside the focused view.
Successful children send a `lane` notice instead of a root completion notice.
Notification deduplication and timers live in memory.

Every 60 seconds, the plugin checks the displayed root and its lanes for pending requests.
Permission and question reminders repeat every ten minutes.
An idle root gets reminders every 15 minutes unless its title starts with `[CLOSED] ` or its tree has running work or pending non-synthetic items.
Reminder clocks pause while Hyprland reports the screen locked and reset while the TUI is focused on that root.

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

`sessions/tui.tsx` binds `ctrl+o` to Open and `<leader>r` to Sessions.
`<leader>shift+return` sends `/done` with no arguments to the root session.
`cli.json` sets the native `open.menu` and `session.list` bindings to `none`; `/open` and `/sessions` still open the native dialogs.
The plugin omits native pins, quick-switch hints, and direct opening by pasted `ses_` ID.

Without a search, Open orders sessions needing permission or question input first, then other open sessions, then titles starting with `[CLOSED] `, newest first within each rank.
Sessions groups by date, newest day first, and applies that order within each group, except closed sessions always come last even when they need input.
Closed titles are muted unless selected or current; rows retain pending-input and running markers.

- Open shows up to eight recents plus projects, expanding the list to include all sessions needing input; typing searches the full loaded session list.
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

`byline/tui.tsx` hides primary-agent labels, currently Collab, on the prompt model row and assistant message footers, and gives the model name the agent color.
Non-primary agents keep their labels.
V2 has no slot for these rows, so `byline/rewrite.ts` rewrites native text nodes in a lifecycle pass each frame.
An upstream layout change can leave the native label visible; assistant footers also keep the native label below 28 terminal columns.

## Code blocks

`code-blocks/tui.ts` renders supported fenced code blocks as padded cards with a raised background and a muted top-right language label.
Cards always use Vagari highlighting, independent of the TUI theme.
Converted nvim-treesitter queries use private `vagari.<ft>` parsers for cards; the hand-maintained `sql/` parser is public because OpenCode has no SQL parser.

Add a parser folder at `config/opencode/tree-sitter/<ft>/` with `highlights.scm` and `tree-sitter-<ft>.wasm`; optional `manifest.json` sets `filetype`, `wasm`, and fence `aliases`.
Sync needs `~/vagari/nvim`, the nvim-treesitter `main` checkout at `~/.local/share/nvim/lazy/nvim-treesitter`, headless Neovim, and the installed OpenCode source in the `src` cache (`src get github.com/anomalyco/opencode@<version>`).
From `config/opencode`, run `bun plugins/code-blocks/sync.ts` after changes to Vagari, nvim-treesitter queries, or the host parser catalog.
Sync regenerates queries, manifests, and resolved Vagari styles; it replaces `tree-sitter/` but keeps every parser folder whose manifest sets no `filetype`, and fails if such a folder shares a name with a generated one.

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
