# OpenCode V2 plugins

This guide covers the local plugins for OpenCode V2.
OpenCode discovers immediate plugin directories and loads their server and TUI entrypoints from source.
`opencode.json` owns agents and permissions; `cli.json` owns terminal settings and built-in plugin exclusions.
The sidebar shows usage, lanes, Mail, Workflow, then browser, with Markdown Context below them; claims that share a placement render in plugin directory-name order.
Run `bun run check` from `config/opencode` for type, lint, and format checks.
For renderer checks, `uv run plugins/capture.py SESSION [--dir DIR] [--seconds N] [--keys TEXT] [--size COLSxROWS] [--out FILE]` captures a real TUI screen in a pty; offscreen fixtures can miss host render nodes.
`dctl update bun` installs plugin dependencies from `config/opencode/bun.lock` with `bun install --frozen-lockfile --ignore-scripts`.
Setup's `extra-bun` item does the same when a locked package for this platform is missing from `node_modules` or has the wrong version.
Saves apply live: the TUI watches plugin sources, discovery directories, and `cli.json`, and the server watches its plugin modules.
After removing or renaming an export another plugin imports, check `opencode.log` for `not found in module`; long-running TUIs can keep failing to load the plugin until `/reload`.
Plugin types under-declare the host; some extra fields, such as `titleView` on `dialog.select` options, still reach the native component at runtime.
The server runs one copy of each plugin per loaded location, and a copy's `ctx.event.subscribe` can receive other locations' session events, so a handler that acts on a session should first check that `event.location` matches `ctx.location`.
Execution and inbox events can omit `event.location`; `compact/server.ts` handles those with state that is safe to update in any copy.

Sidebar and retitle keep named slash and palette commands in `mode: "global"` layers so they remain available during autocomplete.
Their three leader binds use separate base-mode inline layers, so they do not fire in dialogs and have no shortcut hints in the palette.

## Lanes

`lanes/server.ts` extends native `subagent` with a reusable `lane` name and applies `ROUTING.md` routes.
Inputs are `agent`, `description`, `prompt`, and optional `model`, `sessionID`, `background`, and `lane`.
Explicit models use `provider/model#variant`; frontmatter routes keep `provider/model [variant]`.
The `claude` pool picks an Anthropic account from usage caches, keeps an open account on resume, and reports stale-cache or fallback notes in results.
When every account is capped for Opus or Sonnet, `lanes/pick.ts` follows the fallback chain through the same model on Cursor's O pool, then `opencode-go/grok-4.7`.
Cursor Grok checks only C and falls back directly to Go, leaving O for Claude overflow.
Cursor picks skip a route whose fresh pool window reads 100% and has not reset; missing, stale, or unusable usage counts as open and adds a note.
Background children return immediately and send completion, cancellation, or error to the parent's inbox.

The `lanes` tool lists children, including closed ones; `subagent` reuses a lane name to steer or resume one.
Its `tokens` column shows context usage from the latest assistant message since compaction in sidebar format, or `-` when unavailable, including just after compaction.
`lanes_close` takes `{ lanes }` with lane names or child session IDs and returns `closed <entry>` or `skipped <entry>: <reason>` per entry; it never closes a running child.
Closing moves the name from `lane` to `closed: { lane, by, at }` in child metadata and appends a session rule that denies every permission.
`subagent` refuses the `sessionID` of a closed child; naming the old lane starts a fresh child.
Children linked through `step` close when their workflow ends; running children close when they settle, and unlinked children stay open.

`lanes_compact` takes `{ lanes }` with lane names or child session IDs and returns `compacted <entry>`, `skipped <entry>: <reason>`, or `failed <entry>: <reason>` per entry.
Only Collab may call it; it runs each child's own model summary concurrently and waits without sending a follow-up prompt.
Skip reasons are `listed twice`, `missing`, `closed`, `running`, `pending input` (queued in the host inbox), `inbox unreadable`, or `nothing to compact` (no new compactable turns).
Compaction wakes the child and would run queued input, so resume pending-input lanes deliberately with `subagent` or start fresh.

`lanes/tui.tsx` appends to `sidebar.content` and adds `/lanes` with Steer, Interrupt, Continue, and Open actions.
Continue sends `Continue.` directly without a parent completion notice.
Click a non-running child's status icon or use `/lanes-close` to close every non-running child in the sidebar through the `lanes` RPC in `lanes/rpc.ts`, so the TUI and `lanes_close` share the server implementation.
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
A dismissed question or permission prompt can leave a stale execution claim; the server probes `session.wait` for up to one second and wakes the target only if no run is live.
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
The question plugin reads this parked-mail store for its withdrawal check.
The queries depend on host tables `session_v2`, `session_message`, and `session_inbox`, including their JSON message and metadata shapes; review these after host updates.
Received history is filtered by `to` and sent history by `from`, so forks do not inherit a parent's mail.
This plugin has no key or separate state file.
See [Byline](#byline) for transcript boxes and [Hyprd](#hyprd) for desktop notices.

## Question

`question/server.ts` wraps the built-in `question` tool's `execute` with `Effect.raceFirst` and hooks its calls; it adds no tool, key, state file, or TUI slot.
Wrap built-in tools that open forms through the Effect API to keep the calling fiber's location; promise-API `editor.update` drops `form.created`'s location, so the popup never appears.

Before each call, the wrapper prepends the calling message's nearest progress update to the first question's text when available; no assistant text part is required.
It searches backward from this call for reasoning whose decoded signature prefix contains `narration`, stopping at any intervening tool part.
The stored tool input stays unchanged, and the model result has the original question text restored.
No note, an unreadable signature, or a failed history read opens the popup unchanged; failed reads log `question: history read failed; asking without the note`.
Anthropic treats signatures as opaque, so format changes can silently leave the popup without a note.

After a completed call, any answer not exactly matching an option label triggers a reminder to answer questions or doubt in text and end the turn without `question`.
The reminder replaces `You can now continue with the user's answers in mind.`, or is appended if that sentence is absent.

The wrapper first checks for parked mail after 50 minutes, then every 60 seconds, and withdraws the question if the session is still mailable.
A store read failure logs `question: withdraw check failed` and cannot trigger withdrawal.
The tool error tells the model to read and act on the mail, then re-ask if it still needs the user; mail reaches the resumed run at its next step.
The 50-minute threshold leaves 10 minutes before the Anthropic plugin's 1-hour prompt-cache TTL (`anthropic/index.ts`) expires, so the resumed run can reuse the cache last written when the question was asked.
Permission prompts are not withdrawn.

After host updates, recheck that closing sentence in `toModelContent`, `metadata.answers`, option values matching labels, progress-update reasoning and signatures being available before tool execution, and wrapped forms publishing a location so the popup appears without a reconnect.

## Compact

`compact/server.ts` exposes `compact` with no input, queuing compaction after the current turn.
After a successful root `workflow end` at or above 100K tokens, the tool's `execute.after` hook immediately queues compaction without asking; it runs when the turn ends.
The tool result ends with a line telling the agent to end the turn without starting new work; after compaction, `recommended` proposes what comes next.
Root sessions receive soft nudges at 150K and 200K tokens and a strong "finish up now" nudge at 250K.
Each level is sent at most once per compaction cycle; a step that crosses several levels gets only the highest.
With a running workflow, soft nudges say to keep going if the remaining steps fit before 300K.
At a natural break with no lanes in flight, repair pending, or open user question, consider ending the workflow, re-scoping the rest into a new workflow, and compacting first.
Without a running workflow, soft nudges say to compact without asking past 150K once a commit lands or the task is done, with no lanes, edits, or repairs in flight and no open question, including an unanswered user question.
Ask through `question` only if that point is unclear, subject to COLLAB.md's answer-first rule.
The strong nudge says to start no new steps or lanes, let in-flight lanes land, end the workflow at the first point with no lanes in flight, repair pending, or open user question, and commit finished work.
Without a running workflow, it says to start no new work, commit finished work, and compact under the same finished-work rules.
In a driven run, commit finished work and queue compaction without asking.
At 300K tokens, the plugin forces root-session compaction at a step boundary, even mid-task.
Thresholds use the last step's `input + output + reasoning + cache.read + cache.write`.

Every root compaction stores a fixed recovery summary without a model call: “Compaction finished. This session is `<id>`. Load the `recommended` skill and follow it.”
This applies to `compact`, workflow end, the user's `/compact`, and the ceiling.
The [`recommended` skill](../skills/orchestration/recommended/SKILL.md) owns state recovery.
Child sessions keep model summaries and delegate-scope instructions.

After root compaction, the plugin waits for a successful idle session and queues a wake-up follow-up.
The follow-up is suppressed by user input, including queued input, an assistant reply after compaction, a superseding compaction, or a failure or non-shutdown interruption between the compaction request and idle.

`limits.ts` treats a missing configuration location as no configured buffer and caches that result without a toast; other config errors still show one.

## Usage

`usage/server.ts` polls provider usage once per minute, subject to adapter intervals and backoff.
`usage_status` only reads the local cache and reports capacity, freshness, and reset timing.
`usage/tui.tsx` prepends root-session usage rows to `sidebar.content`; click a provider heading to refresh it.
OpenAI and OpenCode Go use V2 integration credentials, Anthropic uses the fixed Claude directories, and xAI uses Grok CLI auth.
Cursor exchanges the bridge's API key at `~/.local/share/dotfiles/cursor-api-key` for a dashboard token and shows `no auth` only when that file is missing.
Caches are `${XDG_CACHE_HOME:-~/.cache}/opencode/usage-sidebar/<provider>.json`.
Locks are `${XDG_RUNTIME_DIR}/opencode/usage-sidebar-<provider>.lock`, or `/tmp/opencode-<uid>/usage-sidebar-<provider>.lock` without that variable.

## Anthropic

`anthropic/index.ts` signs and shapes subscription requests for Trend, Cogikyo, and Cullyn with separate credential and beta-exclusion state.
It uses local credential refresh and vendored request-shaping modules from `opencode-claude-auth` 2.2.1.
See [account state and login](anthropic/README.md) before changing auth code.

`anthropic/tui.ts` binds `ctrl+m` to toggle the prompt's model between Fable 5.2 at `high` and Opus 5.5 at `xhigh`; other models switch to Fable.
`pool.ts` picks Cogikyo or Cullyn from usage caches.
When both accounts are capped, it picks the one that reopens first and shows a warning toast.
The TUI plugin API has no model setter, so it steps `model.cycle_favorite`; each step enters the recent-models list.
The chosen account's target model must be a favorite, or an error toast names it.

`anthropic/claude-auth/transforms.js` moves non-core system text to the first user message and keeps its cache breakpoint when fewer than four other breakpoints remain.

`anthropic/clearing.ts` adds `clear_tool_uses_20250919` to root and child primary requests: trigger at 150K input tokens, keep the latest five tool uses, and clear at least 50K tokens per batch.
Compaction, title, and generate requests receive no clearing policy.
Results from `mcp_Question`, `mcp_Subagent`, and `mcp_Skill` are excluded when those names appear in the request's tools or tool-use history.
These wire names must track the prefix rewrite in `anthropic/claude-auth/transforms.js`; a mismatch silently removes their protection.
Clearing invalidates cached prompt prefixes, so the 50K minimum batches cache rebuilds.
Anthropic reports usage after clearing (probed live 2026-10-15), so native compaction and the compact plugin's ceilings measure the cleared size.
The plugin logs `anthropic: tool results cleared` with `sessionID`, `toolUses`, and `inputTokens` because V2 discards `applied_edits`.

## Guard

`guard/server.ts` denies recognized shell Grok launches, OpenCode API calls, and credential or database reads.
`cd` is allowed; the guard folds one leading `cd <literal path>` into `workdir` after checking that the directory exists.
It also rejects broad home or secret-ancestor `grep` and `glob` roots and checks commit-message shape for agents outside the read-only set.
For agents outside the read-only set, `rm`, `find -delete`, and `find -exec rm` require paths inside `/tmp/opencode/`, written literally or through variables assigned earlier in the command, absolute or relative to a known `workdir` or `cd <dir> &&` target.
They reject `..`, trailing slashes, and top-level globs; `xargs rm` is denied, and other deletion goes through `trash -- <path>`.
Every agent except `collab` and `build/git` may run recognized read-only Git commands or Git in self-contained scratch repos under `/tmp/opencode`, subject to path and config checks; mutations in real repos are denied.
Git checks use parsed commands, so Git text in `echo` arguments or data heredocs does not count as a command.
For agents outside the read-only set, the guard rejects pushes whose repo paths (`-C`, `--git-dir`, or `--work-tree`) or arguments expand at run time; write them literally or through variables assigned earlier in the command.
For `collab` and `build/git`, pushes ask for approval under `~/LeadPier`, when the repo is otherwise undeterminable, with `-c` or `--config-env`, or for forced, destructive, or remote-program forms; other pushes pass this guard.
For all agents, `systemd-run` requires `--collect` (`-G`) or a `CollectMode=` property; use `-p CollectMode=inactive` to retain failed units deliberately, accepting hyprd health alerts.
For every agent and tool, it rejects inputs naming nonexistent default XDG user dirs relocated by `user-dirs.dirs`, except fields used to find or fix stale references, and lists the configured dirs in the denial.
Use `x` for X search, `sessions` for history, and `session_create` for new sessions.
Prefer `workdir`; V2 resolves each remaining `cd` argument against it, so chained relative changes can cause false external-directory prompts.
The `context`, `compaction`, and `generate` hooks describe parallel calls and the shell environment: zsh without unquoted `$var` word-splitting outside the read-only set, or offline `bash -c` in cage for read-only agents.

For `review/*`, `scout/*`, `verify/source`, and `verify/web`, the guard allows only `shell`, `read`, `grep`, `glob`, `webfetch`, `websearch`, `skill`, `sessions`, `x`, `diagram`, and `workflow`; agent permissions can restrict these further.
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
System tmpfiles (`system/etc/tmpfiles.d/opencode-tmp.conf`) remove unused entries in `/tmp/opencode` and caged scratch as root after 3 days; the index is exempt.
There is no tool, key, or TUI slot for the index.

## Terminal

`terminal/server.ts` exposes `terminal` with `command`, `reason`, and optional `workdir`, which defaults to the session directory.
Use it only for commands that need the user's keyboard, including passwords, interactive prompts, and `dctl` root stages; run other commands in `shell`.
Only top-level sessions can call it; children get an error before any permission ask or window and must return the command to their parent as a blocker.
The `reason` tells the user what the command does and what they must type or answer.
Before calling, state your results and what the user must do in your reply, because the window covers the session.
Before the permission ask, the plugin rejects control characters except newline and tab in `command`; `reason` and `workdir` allow none.
The plugin changes permission `allow` to `ask` for `terminal`, including under the global allow-all rule.

Approval opens a fresh Kitty window with class `opencode-terminal`, `--title` set to `<session title> — <reason>`, and remote control disabled.
Session-title control characters become spaces; a title that is empty after trimming becomes `OpenCode`.
`config/hypr/hyprland.lua` floats and centers it with a red border.
`terminal/run.sh` sets the Kitty tab title to the session title with OSC 2 and prints a header with the session badge, wrapped reason, and working directory (`~`-abbreviated and `%q`-escaped).
The command uses `bat` highlighting with a plain-text fallback and `$` on its first line; a rule separates the header from command output.
The runner executes the command in a real tty without another confirmation.
The footer shows `✓ exit N · Enter to close`, with `✗` for a nonzero exit; runner failures show a separate message.
Queued input is discarded before the Enter wait; only a fresh Enter or closing the window dismisses it.
The call blocks until the window exits, then returns the recorded command exit code, cleaned output tail, and log path under `/tmp/opencode/terminal/<call id>/` when available.
The report distinguishes a window closed before startup from one closed during execution without an exit code.
For debugging, inspect the call directory's `result` record and `runner.err`; runner failures are reported separately from command exit codes.

The plugin rejects recognized `shell` calls to `sudo` without `-n` or `--non-interactive` before execution and points to `terminal`.
It also appends that pointer when shell output contains `sudo: a terminal is required` or `sudo: a password is required`, with `ssh -t` inside `terminal` suggested for remote sudo.

## Retitle

`retitle/server.ts` renames a titled root after two new user messages of at least three words and a successful turn, at most once per five minutes.
Auto-retitle skips any tagged title, and manual retitle refuses a `REFUSALS` title (see [Sessions](#sessions)).
It asks the configured `title` agent's model for a two-to-four-word title naming the session's main work, keeping the current title when it still fits.
It generates the title without a temporary child session.
A title containing letters but no lowercase letters stays pinned.
Counters live in memory, and the resulting title lives on the session.

Context comes from full stored session history, using the first two qualifying user messages, up to four spread samples, and the latest three.
The trigger and message context skip non-user items and messages under three words after removing image markers.

Generated titles are Title Case and at most 36 characters, so they fit on one sidebar line; words that already contain a capital keep their casing.

`retitle/tui.tsx` adds `<leader>s` (`/retitle`) to retitle the root now with the same stored context, omitting the current title from the prompt.
A pinned title gets the new name in caps.
`<leader>shift+r` (`/caps`) pins the title by writing it in caps, or unpins it with a fresh Title Case title.
Clicking the sidebar title copies the shown session's `Title (ses_…)` to the clipboard via OSC 52.
V2 has no sidebar-title slot, so the plugin hooks the native title node.
If that lookup breaks, one warning toast appears; retitle keybinds still work.

## Hyprd

`hyprd/tui.ts` loads the notification, pane-context, and focus plugins together.
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
Permission and question reminders repeat every 30 minutes on any open root until answered.
Only the displayed root gets idle reminders every hour, unless its title starts with a closed tag or its tree has running work or pending non-synthetic items.
Each root's clock pauses while Hyprland reports the screen locked and resets while the TUI is focused on that root, leaving other roots' clocks unchanged.
Clocks are dropped when their roots are no longer open.

`hyprd/context.ts` checks the displayed root, prompt text, tab count, and pending input each second and publishes changes to `$XDG_RUNTIME_DIR/opencode/tui/<TUI-pid>.json`, with a heartbeat each minute.
Pending input covers current permissions and forms plus unresolved forms seen before navigation, whose partial answers remain in host memory.
It requires `XDG_RUNTIME_DIR` and `KITTY_WINDOW_ID`, records the process `start` time from `/proc/self/stat` and report time `at`, and removes the file on disposal.
It creates the directory with mode `0700` and the file with mode `0600`, then publishes by atomic rename.
Hyprd matches the record to a discovered TUI pane and checks the process start time before using it for [recycling gates and relaunch](../../../cmds/cmd/hyprd/README.md#opencode).

`hyprd/focus.ts` reports renderer focus and blur through `hyprd/rpc.ts`, plus root-session changes and a 60-second heartbeat while focused.
It reports `focused: false` on disposal.
`hyprd/server.ts` emits each report as the ephemeral `rpc.hyprd.focus` event with `{ tui, session, focused }`, where `tui` is the TUI PID and `session` is the root ID or `""`.
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

`sessions/tui.tsx` binds `ctrl+o` to Open and `ctrl+r` to Recent; `sessions/open.tsx` serves both pickers.
The `ctrl+r` opener does not fire in modal dialogs except from Recent or Open.
`<leader>shift+return` sends `/done` with no arguments to the root session.
`/archive` and `/research` set the root session's state to `ARCHIVED` or `RESEARCH` without running `/done`, keeping its flags, then open Home in its directory so the next prompt starts a new session.
`/refusals` toggles `REFUSALS` on the root session, keeping its state and other flags, and stays in the session.
`cli.json` sets the native `open.menu` and `session.list` bindings to `none` and `session.rename` to `<leader>r`; `/open` and `/sessions` still open the native dialogs.
The plugin omits native pins, quick-switch hints, and direct opening by pasted `ses_` ID.

Open lists only top-level sessions across all projects, excluding closed sessions even from search results.
Recent lists only the current directory's sessions, including closed ones.
Running lists sessions mid-turn or with a child mid-turn; Open follows with the rest.
Both pickers put sessions needing permission or question input and non-`REFUSALS` sessions with parked mail first within Running and Open, newest first within each rank.
The Open group's footers use the statusline's context-pressure colors by time since last update, from blue under 15 minutes to pink at three days or more.
Recent adds a Closed group last, newest first.
Closed titles are muted, with `RESEARCH` titles in a faint scout blue, unless selected or current; rows retain pending-input and running markers.
In both pickers, an envelope after a session title marks parked mail for open, non-`REFUSALS` sessions.
The marker and mail ranking share one `mail.parked` RPC request on opening, refreshed on inbox events and `server.connected`, instead of querying each session's inbox.

In Recent, `ctrl+d` deletes and `<leader>r` renames the selected session.

The plugin relies on host behavior beyond the published types: extra option fields such as `titleView` and `gutter`, selection preservation with a defined `current`, the `modal` mode after dialog replacement, and a home route carrying `location`.
If the host stops forwarding the extra row fields, rows degrade to plain titles without muting or status markers.
Review the selection and navigation assumptions after an OpenCode update.

## Launch

`launch/server.ts` adds `session_create` to the `opencode` Code Mode namespace, so agents call `tools.opencode.session_create` through `execute`.
Inputs are `directory`, `title`, and optional `prompt` and `agent`; the directory must be absolute or start with `~` and must exist.
It creates a top-level session with session metadata `origin` set to the caller's session ID, then sends `prompt` as the first message to start a turn.
Without `agent`, the session uses the configured default primary agent; a named agent must be a visible primary agent at that directory.
Only top-level sessions can call it, and the new session is not a lane.
V2 gives plugins no permission-ask path, so the user's request in chat is the approval; the guard's `opencode api` denial points here.
If the prompt fails, the error names the created session, so the agent does not retry and create a duplicate.
There is no persistent state, key, or TUI slot.

## Diagram

`diagram/server.ts` exposes `diagram` for display-cell geometry and connectivity checks on documentation diagrams and trees.
Pass `text`, or `file` with `start` and `end`; optional `prefix` and `width` describe the host layout.
File mode accepts files under the project session directory or the caller's own scratch directory; pass text for other layouts.
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
It opens headed and closes the MCP process and forgets the preview login on execution success, failure, interruption, or session deletion.
The user `headless-reap.timer` also kills headless Chromium and Firefox older than 3 hours on a 15-minute schedule; headed browsers are excluded.
URL-bearing calls accept HTTP(S) or `about:blank`; MCP launch patterns restrict network schemes.
The tool catalog lives in plugin KV `catalog/1.7.0`.
`browser/tui.tsx` places browser workspaces after `sidebar.content`, subscribes through `hyprd`, and focuses a workspace when clicked.
It needs `/usr/bin/chromium`, `npx`, and `hyprd` for headed workspace controls; there is no key or separate plugin state file.

`browser/preview.ts` signs in automatically on `navigate_page` URL calls or `new_page` calls to `https://lpadmin.cullyn.dev` or `https://lpuser.cullyn.dev`.
It reads the `leadpier-preview` keyring entry that Cullyn sets with `cogictl secret set leadpier-preview` and authenticates as preview user 12919 (`cullyn@trendcapital.com`, admin).
Each execution logs in again on its first matching navigation and caches the login until execution ends.
Each matching URL navigation seeds the page's `authentication` localStorage entry before the app loads, only when the entry is absent, including after browser relaunches or in new browser contexts.
Clicked links, redirects, and back/forward/reload do not seed the session; navigate to the URL directly.
If the app logs out, the next matching URL navigation seeds the session again.

The plugin hides `take_heapsnapshot` from agents.
`browser/mcp.ts` launches MCP with `--redactNetworkHeaders`, which hides headers such as `Authorization` but does not redact request or response bodies.
Agents must still avoid reading or returning authentication data and sensitive page content.

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
`statusline/view.ts` shares one `hyprd subscribe view` process per TUI to pause these animations in out-of-view panes; unknown view state leaves them running.

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

`workflow/server.ts` stores one workflow per root session in plugin KV `flow/<root>`: fence source, revision, step marks, checklist ticks, repair rounds, child links, and the revision diff.
The `workflow` tool reads status with `{}` and adopts a fence with `start`; see the [workflow skill](../skills/prose/diagrams/workflow/SKILL.md) for fence rules.
Start and status show root context, headroom to forced compaction at 300K, and unfinished steps × 18K as a rough cost; an estimate above headroom warns without imposing a step cap.
A new revision matches steps by owner and headline, carries unchanged results, and reports added, renumbered, changed, and dropped steps.
Changed checklists reset ticks and unlink the child; `end: true` deletes the workflow, so the next start begins at revision 1.

`subagent({ …, step: n })` links a child and appends the step's checklist to its brief.
The child can tick only its own items with `workflow({ check: [n…] })`; Collab owns step marks and repairs.
Status distinguishes running and returned children; `Ready:` excludes both.
After compaction, recovery reads `workflow` status; the store remains the source of step state.

Starting a revision in parallel with `subagent({ …, step: n })` can link a child to the new revision with the old checklist, so its ticks land on the new step.
Start a revision and launch its children in separate turns.

The user's `/replan` pauses dispatch and sends Collab workflow status plus each lane's step, status, idle time, and context size.
The [drive skill](../skills/orchestration/drive/SKILL.md#replan) owns lane verdicts, questions, pre-approved compaction, and the next revision that unpauses dispatch.

The sidebar shows the heading's short title and progress count, groups done steps into a `✓ 1, 2, 3` row, and lists remaining steps with agent icons and colors.
Click the section or press `<leader>w` to replace only the chat with the full card, including checklists, repair rounds, and revision diff; the sidebar and prompt stay visible.
Press `<leader>w` again to return to chat.

`workflow/tui.ts` also renders `workflow` fences as cards with an automatic dependency graph.
Graph nodes are numbers in the owner's color; lane steps are role-color pills, and gates use `◆N`.
Rows lead with the scope icon and scope name in the role color, then the number, a square lane pill for lane steps, and the headline.
A headline's main idea, before any `: `, takes the owner's color, bold for lanes and italic for one-shots; the rest is regular text.
Delegated rows show the model family flush right in the role color, bold for lanes and italic for one-shots.
The renderer reads `ROUTING.md` and shows a brain glyph and the variant left of the model only for a variant departure; no brain appears when the file is missing.
`plugins/lanes/routing.ts` holds route parsing shared by the lanes server and the card.
Gate checklist items get a distinct watching-for glyph; `if:` steps show an `only if` row.
Only the summary wraps; other rows clip, and bad specs show red error lines.
From `config/opencode`, run `bun plugins/workflow/preview.ts [file] --width N` to print the card with ANSI color.

Files under `workflow/` divide the work by role:

- `server.ts` owns storage and the tool; `state.ts` owns revisions, marks, ticks, and repair rules.
- `parse.ts` reads fences and dependencies; `progress.ts` derives step phases from child state.
- `rpc.ts` defines state, pause, link, and change events; `live.ts` keeps the TUI state current.
- `tui.ts` registers fence rendering and commands; `replan.ts` builds the paused-run brief.
- `sidebar.tsx` shows compact progress; `view.tsx` swaps the chat for the full card.
- `card.ts`, `graph.ts`, and `style.ts` draw cards; `preview.ts` prints them for inspection.

## Bullets

`bullets/tui.ts` uses `•` for depth-0 unordered list markers and `◦` at deeper levels.
Ordered lists keep their native markers.
V2's Markdown plugin hook only replaces fenced code-block rendering, so list markers need a patch.
An opentui upgrade can break this private `getListItemInputs` override; the plugin throws on load if the method disappears.

## Markdown context

`context/tui.tsx` places Markdown Context after `sidebar.content`, with root and project instructions, the agent prompt, skills, and observed Markdown loads.
The list opens by default and collapses only when expanding it would make the sidebar scroll, re-checking on sidebar resize and as other sections grow.
Rows before the latest completed compaction use a compacted marker; the list is a view of recorded loads rather than proof that every body remains in active context.
Click the heading to toggle the list and pin that choice until the sidebar remounts, or a row to open its file with `hyprd edit`.
The plugin does not change loaded context and has no tool, key, or state file.

## Shell environment

`env.ts` restores or clears `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME` from supplied `HOST_` variables before shell creation.
It is a standalone source module, not a plugin directory, so verify its load path when using an isolated launch environment.
It has no tool, key, TUI slot, or state file.
