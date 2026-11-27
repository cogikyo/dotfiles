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

Sidebar, retitle, and drive keep named slash and palette commands in `mode: "global"` layers so they remain available during autocomplete.
Their four leader binds use separate base-mode inline layers, so they do not fire in dialogs and have no shortcut hints in the palette.

## Lanes

`lanes/server.ts` extends native `subagent` with a reusable `lane` name, and `lanes/pick.ts` resolves each launch's model.
Inputs are `agent`, `description`, `prompt`, and optional `model`, `sessionID`, `background`, `lane`, and `step`.
Models use `provider/model#variant`; without `#variant`, the model's default applies.
A launch uses the explicit `model`, else the `step` card's model, and fails if the model or variant is not listed.
A new launch with neither fails and points to `ROUTING.md`, which guides the choice; the plugin does not read it.
A resume with neither keeps the stored launch model, or fails asking for `model` if config dropped it.

Before a launch, `lanes/pick.ts` checks the usage cache of a `claude`, Cursor, Gemini, xAI, OpenAI, or OpenCode Go pool; other providers, including direct `anthropic*` accounts, launch unchecked.
A pool is capped when a fresh window reads 100% and has not reset; missing, stale, or unusable usage counts as open and adds a note.
Cursor's `composer-*`, `grok-*`, `auto`, and `default` use its `C` window, and other models use `O`.
Gemini checks its 5-hour and weekly windows, and xAI checks its credit window `W` and monthly window `M`.
The launch note shows the tightest window's headroom and reset time.

The `claude` pool picks an Anthropic account from usage caches and keeps a lane's account while it is open.
It treats Trend's 5-hour window as full at 90%, so headroom notes count to 90%; request failover keeps 100%.
When every account is capped, the launch moves to the same model on Cursor if Cursor lists it, keeping the variant if Cursor has it, and adds a note.
A lane resumed there stays while Cursor's pool is open.
When no backup is open, the launch fails and lists the newest non-deprecated model of each family whose pool is open, with its headroom or why usage is unknown.

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
Lane names live in child session metadata; plugin KV entries `child/<parent>/<child>` track children, and `anchor/<child>` stores each lane's launch model.
Collab delegates; among its children, `verify/decide`, `build/owner`, and `review/entropy` may launch read-only one-shots in the foreground, within their frontmatter permissions.
`experimental.subagent_depth` is 2 in `opencode.json`, so their children cannot launch anything.
`lanes` lists only the caller's direct children, so Collab's list omits these helpers.
Their one-shots also need `model`.

## Mail

`mail/server.ts` exposes `mail` with `to`, required `kind` (`info` or `ask`), `subject`, `body`, and optional `replyTo` and `urgent` for cross-session handoffs, including across directories.
`to` accepts a top-level session ID or an exact title with its tag stripped; missing or ambiguous title matches fail and list up to eight close matches when available.
`replyTo` must name a mail message this session received from the target.
Subjects are single lines of at most 80 characters, and bodies are plain text of at most 4000 characters.
Callers and targets must be open, non-`REFUSALS`, top-level sessions, as defined by `mailable()` in `mail/mail.ts`.
Title matching uses the same rule; self and missing targets are refused.
Mail comes from the sending session's agent and carries no approval on its own; a relayed user decision counts as the user's answer.
This trust is a prompt rule; the plugin checks no authority.
`info` expects no reply, and acknowledgment-only replies are forbidden.
`ask` expects a `mail` reply with `replyTo`, even when the answer is that the agent can't act or is waiting on the user.
`urgent` is independent of `kind`.

Sending mail immediately attempts to wake an idle target; a running target gets it at its next step, without interrupting a long tool.
At setup and every 10 seconds, a backstop sweep attempts wakes for mail parked at least 10 seconds.
A dismissed question or permission prompt can leave a stale execution claim; the server gives the local `session.wait` probe one second and skips the wake unless it succeeds.
A staged revert blocks the wake, as does a failed last turn that ended after the mail parked.
An in-memory `waking` set prevents duplicate wakes in the service process; cross-process races are accepted because one service process hosts the sessions.

Parked `ask` mail withdraws a `question` popup after 5 minutes, and urgent mail withdraws it after 5 seconds, checked every 2 seconds, unless the session's last typing report is less than 5 minutes old, even for urgent mail.
Non-urgent `info` mail waits until the form or prompt closes, then arrives as usual.
The same `ask`/urgent and mail-age rules set permission prompts aside by rejecting them with feedback, never approving them.
The host rejects the session's whole pending permission batch, including `terminal` requests.
Terminal popups and permission prompts have no other exemption; open Kitty windows and child sessions' prompts and popups are untouched.
`patience` and `poll` in `mail/store.ts` own these timings.

`mail/tui.tsx` shows only open items: unread mail here, an ask owed, mail waiting in the target, or a sent ask with no reply yet.
Delivered info and answered asks leave the sidebar.
An answer counts only when its `replyTo` names the ask and its `from` and `to` reverse the ask's endpoints.
Owed asks and delivered sent asks also leave when the peer is closed, `REFUSALS`, or deleted.
The section hides when nothing is open.
Incoming rows show the kind glyph and age; sent rows have a closed envelope while waiting in the target and an open envelope while awaiting an ask's reply.
Urgent incoming glyphs and waiting sent envelopes are red; byline postmarks read `urgent info` or `urgent ask`.
Rows open the peer session when clicked.
The sidebar refreshes on inbox enqueue, delivery, and cancellation, `session.deleted`, `server.connected`, and successful `mail` calls, debounced by 300 ms.

`mail/rpc.ts` exposes `ledger` and `parked`; `mail/store.ts` reads the OpenCode DB read-only because the TUI initially loads only the newest 20 messages per session.
The question plugin reads this parked-mail store for its withdrawal check.
The queries depend on host tables `session_v2`, `session_message`, `session_part`, and `session_inbox`, including their JSON message and metadata shapes; review these after host updates.
Received history is filtered by `to` and sent history by `from`, so forks do not inherit a parent's mail.
This plugin has no key or separate state file.
See [Byline](#byline) for transcript boxes and [Hyprd](#hyprd) for desktop notices.

## Question

`question/server.ts` wraps the built-in tool to store one queued form per session and return at once.
The agent then writes its full reply as final text and ends the turn.
An `experimental.idle` hook opens the real form below that reply, raced against the [Mail](#mail) parked-mail deadline.
Questions stay short; the reply above the form carries the explanation.
Wrap built-in tools that open forms through the Effect API to keep the calling fiber's location; promise-API `editor.update` drops `form.created`'s location, so the popup never appears.

`question/tui.ts` reports keypresses and pastes in a focused question form through `question.typed` RPC, at most once a second per session.
The server's typing hold uses `store.patience.normal`, survives hot reloads in process-global state, and clears when the form settles.

Answers resume the session as the next message and remain visible in the transcript.
Dismissal does not resume it; the user can answer in chat.
Mail withdrawal applies only while the session is mailable and resumes it with instructions to read the mail first.
Interrupts and errors clear the stored form so it cannot open later.
The `terminal` tool's popup stays synchronous.

Outcome descriptions use `Answered`, `Dismissed`, `Withdrew`, or `Dropped` plus the question count.
Without Byline, the host shows a line such as `◈ Dismissed 1 question`.
See [Byline](#byline) for outcome boxes and colors.

After host updates, recheck wrapped forms publishing a location so the popup appears without a reconnect.

## Compact

`compact/server.ts` exposes `compact` with no input, queuing compaction after the current turn.
After a successful root `workflow end` at or above 300K tokens, the tool's `execute.after` hook immediately queues compaction without asking; it runs when the turn ends.
The tool result tells the agent to end the turn without starting new work.
Root sessions receive four escalating nudges, each sent at most once per compaction cycle; a step that crosses several levels gets only the highest.
A good spot means a commit landed or the task is done, with no lanes, edits, or repairs in flight and no open question, including an unanswered user question.
With a running workflow, the matching spot is a step boundary where the workflow can end and re-scope the rest; the plugin compacts after the turn that ends it.

| Level | Nudge                                                                                                |
| ----- | ---------------------------------------------------------------------------------------------------- |
| 300K  | Compact at a good spot, never mid-task; ask through `question` only if the spot is unclear.          |
| 400K  | Plan the compaction point, name it in one line of the next reply, and compact before any new work.   |
| 475K  | Warn that native auto-compaction runs mid-turn; compact at the next good spot and start nothing new. |
| 500K  | Compact now at the first good spot; only a critical deploy, migration, push, or wave finishes first. |

A user's decline holds until the 500K nudge, which asks once more instead of compacting.
In a driven run, commit finished work and queue compaction without asking.
Thresholds use the last step's `input + output + reasoning + cache.read + cache.write`.

Every root compaction stores a script-built checkpoint with no model call or kept tail.
This applies to `compact`, workflow end, the user's `/compact`, and native auto-compaction.
Its first line names the session ID and directs Collab to the [`recommended` skill](../skills/orchestration/recommended/SKILL.md), which owns recovery.
Child sessions keep model summaries and delegate-scope instructions.

The checkpoint uses the latest successfully ended workflow as its boundary, or session start if none has ended.
It records stored workflow status, open lanes and a count of closed lanes, live `git status --short --branch` and `git worktree list`, and the last reply.
It quotes user messages and question-form answers since the boundary verbatim, newest first, and puts older user words in a separate section.
The renderer trims history toward a 64,000-character budget, roughly 16K tokens.
History cuts show `[omitted: …]` with a message ID range or `[clipped: …]` inside a message.
Failed sections show `Unavailable: <reason>`.

After root compaction, the plugin waits for a successful idle session and queues a wake-up follow-up.
The follow-up is suppressed by user input, including queued input, an assistant reply after compaction, a superseding compaction, or a failure or non-shutdown interruption between the compaction request and idle.

`limits.ts` treats a missing configuration location as no configured buffer and caches that result without a toast; other config errors still show one.

## Drive

`drive/server.ts` stores one drive state per root session in plugin KV `root/<root>`: `{ on, at }`, with `at` as a UTC time without milliseconds.
The user flips it in the TUI with `<leader>d`, `/drive`, or Toggle drive in the palette; no tool turns it on.
`drive_off` turns it off for the caller's tree; only a root session may call it, and children get an error.
State survives compaction and restart, and a session made with `session_create` starts off.
The plugin removes a key on `session.deleted` and sweeps keys of already-deleted sessions at setup.

On every primary model request, the `context` hook adds exactly one drive line to the system text of each reader: every root session, and every `verify/decide` whose parent is a root.
Title, compaction, and generate requests do not run this hook, and other children get no line.
The line takes one of four forms:

- `Drive is on since <UTC time>.` while the stored state is on.
- `Drive is off since <UTC time>.` once it has been turned off.
- `Drive is off.` when the tree has no record.
- `Drive state is unavailable; treat drive as off.` when any step fails, after logging `drive: context failed`.

A failed reader lookup also pushes the unavailable line, so a non-reader can receive it.
`drive_off` returns `Drive turned off.` or `Drive was already off.`, neither in a line's form.
The line is never stored in history, so a flip takes effect at the next request.
On Anthropic requests, `claude-auth/transforms.js` moves the line with the rest of the non-core system text into the first user message (see [Anthropic](#anthropic)).
[COLLAB.md's Boundaries](../COLLAB.md#boundaries) own what the line authorizes; permissions are unchanged.

`drive/tui.tsx` toasts `Drive on` or `Drive off` after each flip.
It mounts statusline's `Pace` in a slot appended to `prompt.footer`; while the shown session's root has drive on, `Pace` runs `drive/look.ts`.
A draw error stops the look and reports `look off, <reason>` through the drive error toast.
`drive/look.ts` draws the wheel and the chevrons, and hides statusline's work light while the chevrons have room.
`drive/arrows.ts` draws `❯` chevrons on the prompt's model row, in its longest run of empty cells and 4 cells clear of the text or gauge at either end.
Each input row above it, from the editor's top padding down, runs its own group of about three chevrons, so the rows move out of step; groups draw only in empty cells 4 cells clear of text, the cursor, and the wheel.
While a session in the root's family runs, the groups take that family's [work color](#usage) at full brightness and travel left to right at 16 cells per second; otherwise they are dim bright blue at 8 cells per second.
A group that leaves its row re-enters from the left after a random wait.
With `animations` off, nothing travels: three chevrons park at the right end of the model row in the same colors.
With `animations` on, changes in color, brightness, and speed ease over 600 ms; with it off they snap.
While the chevrons travel, `Pace` holds statusline's shared animation clock, so an idle session renders about 10 frames per second while its terminal has focus.
When the model row's run is shorter than 7 cells, `drive/arrows.ts` draws no chevrons in any row and the work light shows instead.
`drive/wheel.ts` puts a steering wheel in the editor's top-right corner when text leaves room, in the work color while the family runs and bright blue otherwise, and preserves the queue glyph.
`drive/rpc.ts` defines the `state` and `set` methods and the `changed` event the TUI follows.

## Usage

`usage/server.ts` polls provider usage once per minute, subject to adapter intervals and backoff.
`usage_status` only reads the local cache and reports capacity, freshness, and reset timing.
`usage/tui.tsx` prepends root-session usage rows to `sidebar.content`; click a provider heading to refresh it.
OpenAI and OpenCode Go use V2 integration credentials, Anthropic uses the fixed Claude directories, and xAI uses Grok CLI auth.
xAI's `G` row is the X Grok Bot window's weekly usage, read through `grok-bot/bot.ts --pool x pools --json`.
`G` shows `--` while that window is down or unresponsive, and appears only when the Grok CLI billing reads.
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
On a main session's first turn, if its model is the agent or config default on Cogikyo or Cullyn and `session_create` did not pin it, `anthropic/index.ts` moves it to the ranked Cogikyo or Cullyn pick.

`anthropic/claude-auth/transforms.js` moves non-core system text to the first user message and keeps its cache breakpoint when fewer than four other breakpoints remain.

`anthropic/clearing.ts` adds `clear_tool_uses_20250919` to root and child primary requests: trigger at 150K input tokens, keep the latest five tool uses, and clear at least 50K tokens per batch.
Compaction, title, and generate requests receive no clearing policy.
Results from `mcp_Question`, `mcp_Subagent`, and `mcp_Skill` are excluded when those names appear in the request's tools or tool-use history.
These wire names must track the prefix rewrite in `anthropic/claude-auth/transforms.js`; a mismatch silently removes their protection.
Clearing invalidates cached prompt prefixes, so the 50K minimum batches cache rebuilds.
Anthropic reports usage after clearing (probed live 2026-10-15), so native compaction and the compact plugin's thresholds measure the cleared size.
The plugin logs `anthropic: tool results cleared` with `sessionID`, `toolUses`, and `inputTokens` because V2 discards `applied_edits`.

## Gemini

`gemini/server.ts` adds the `gemini` provider, which runs Google's `agy` agent over ACP under the lane's assigned OpenCode agent rules.
It needs `agy` from the AUR package `antigravity-cli`, signed in on a paid Google AI subscription, and `bun` on PATH for its hook and bridge shim.
`opencode.json` must allow `gemini` in its `provider.use` policies, which deny every provider they don't name.
The plugin lists no models; config declares each model with its `effort` variants, and [ROUTING.md](../ROUTING.md#models) says when to use them.
Each turn starts `agy acp` in the lane's session folder, bridging allowed OpenCode tools over MCP.
When `agy` advertises `systemPrompt.append`, `gemini/session.ts` uses `session/resume` to send only a ~1 KB note (`gemini/prompt.ts`) that maps OpenCode tool names to `agy`'s and describes the sandbox, without forwarding OpenCode's system prompt; new conversations call `session/new` first.
`gemini/agy.ts` strips `GOOGLE_`, `GEMINI_`, `GCLOUD_`, `CLOUDSDK_`, and `AGY_` variables, and a turn fails unless the agent reports a paid Google AI plan: Plus, Pro, or Ultra.
A turn also fails after 10 minutes without an update from the agent, paused while a permission prompt or bridged tool call runs.
`usage/gemini.ts` reads the shared 5-hour and weekly windows from `agy --print /usage`, and a lane launch fails and lists open pools when either window is capped.
`agy` sums usage over every model call in a turn, so `gemini/model.ts` splits its last `usage_update` context size across prompt and output tokens, which OpenCode sums against the context window.
Output keeps the turn's summed count, capped one token below that size because OpenCode ignores zero-prompt readings.

The agent's settings live in `${XDG_STATE_HOME:-~/.local/state}/opencode/gemini/`, passed as `--gemini-dir`; the user's own `~/.gemini` is left alone.
On every start, the plugin rewrites `antigravity-cli/settings.json` to `{"toolPermission":"strict"}` so every tool call reaches the plugin as an ACP `session/request_permission`, and rewrites `config/hooks.json` to one `PreToolUse` hook on every tool that runs `gemini/hook.ts`.
Copies of the plugin loaded from different checkouts share this folder and overwrite each other's hook path.
It refuses to start when the session folder, or a parent up to the repository root, holds `hooks.json`, `mcp_config.json`, `plugins.json`, or `plugins/` under `.agents`, `.agent`, `_agents`, or `_agent`, because those can run code outside OpenCode's permission checks.

A `model.request` hook in `gemini/server.ts` passes the lane's agent in `x-opencode-agent`, and `gemini/session.ts` passes owner metadata to the hook in `OPENCODE_GEMINI_OWNER`.
`gemini/calls.ts` translates `read_file`, `write_file`, `edit_file`, `run_command`, `manage_task` (`list`, `status`, and `kill` only), and `search_web` into OpenCode tool inputs and permission checks.
Both `gemini/hook.ts` and `gemini/calls.ts` admit only those six tools and bridged `mcp_opencode_*` names; sub-agents, interaction requests, other `manage_task` actions, other MCP servers, and unknown tools are refused.
For native tools, `gemini/hook.ts` denies calls with no session, then runs `gemini/calls.ts` and [guard's](#guard) shared `guard/review.ts` for every agent, denying refusals in-turn.
At approval, `gemini/gate.ts` runs those checks again and builds permission checks from the input guard approved, including rewritten working directories and resolved symlink targets.
Each check goes through `ctx.permission.assert`, applying the lane agent's rules, session overrides, saved "always" rules, and `permission.evaluate` hooks (including guard's and Gemini's git-push ask holds); `ask` rules show a native prompt.

Calls refused at approval (such as permission-rule denials or approval-time changes the hook cannot see) are answered `reject_once`, and after `agy` ends its turn, `gemini/model.ts` sends the refusal reasons back as a follow-up prompt.
At most two follow-ups run per turn, and their output adds to the turn's total; after that, the turn ends with the last reasons shown.
A plain user decline ends the stream as declined without a follow-up.
When guard rewrites a call (caging read-only `run_command` through [cage](../../../cmds/cmd/cage/README.md), folding a leading `cd`, or resolving a read-only `read_file` `Path` to its realpath), `gemini/hook.ts` returns that `overwrite` and writes a marker under `cages/<toolCallId>` in the profile directory.
`gemini/gate.ts` refuses any call needing a rewrite with separate messages when the marker is missing or when its recorded rewrite differs from what guard requires; after 8 seconds, `gemini/hook.ts` falls back to `force_ask` so `gemini/gate.ts` still judges it.
For read-only agents, `write_file` runs natively through `agy` under the lane's `shell` permission, limited by `gemini/calls.ts` to regular files directly in `~/.local/state/opencode/scratch/<session-id>`.
It refuses `..` in `Path`, subdirectories, `tmp` and `cache`, symlinks, non-regular files, and hard links; `edit_file` is refused and points to `write_file`.

Each turn passes one stdio MCP server, `opencode`, in ACP `session/new` and `session/resume`.
`gemini/bridge.ts` listens on a per-turn Unix socket under `bridge/` in the profile directory and requires a random token on the first line, passed to `gemini/shim.ts` in the MCP server spec's environment.
`gemini/shim.ts` relays MCP stdio to that socket; the socket closes when `agy` exits, and the next turn removes sockets whose owning process is dead.
`gemini/tools.ts` lists registered tools that the lane's agent and session rules do not wholly deny, exposed to the model as `mcp_opencode_<tool>` and marked to load directly.
It leaves out tools `agy` already has natively (`shell`, `read`, `write`, `edit`, `patch`), interactive and delegation tools (`question`, `subagent`, `terminal`, `mail`, `lanes`, `lanes_compact`, `lanes_close`, `opencode_session_create`, `drive_off`, `compact`), and `x`.
`x` stays out because guard records its runs in execute hooks, which a direct call skips.
`gemini/schema.ts` reduces each tool's input schema to the subset Gemini's function declarations accept and drops any tool it cannot reduce, because one rejected declaration fails the whole turn.

When `agy` calls a bridged tool, `gemini/gate.ts` matches the approved permission request on exact arguments (ignoring the `toolSummary` field `agy` adds) and refuses a mismatch.
It then runs guard's review on the raw input, decodes it with the tool's schema, and runs the tool as the lane's session and agent.
Text output over 8 KiB is written to `~/.local/state/opencode/scratch/<session-id>/output/<call-id>.txt`, returning the head up to 8 KiB and that path.
Code Mode `execute` is not bridged because a plugin cannot run a Code Mode program, so Gemini lanes call each tool directly.
Both `gemini/shim.ts` and `gemini/hook.ts` start Bun with `--config=/dev/null --no-env-file`, so a repository's `bunfig.toml` cannot preload code.

The host caches each provider's SDK and language models for the server's lifetime, keyed on provider, package, and variant settings (`core/src/aisdk.ts`).
Gemini plugin edits reach only variants the running server has not used yet; to reload an already-used variant, restart the background server with `opencode service restart`.

Known gaps:

- Shell permission text comes from guard's parser and drops quoting.
  - That is equal or stricter on current rules, though a rule written against a quoted form may not match.
- A symlink retargeted between approval and file open is not caught.
- Gemini edits appear as Gemini tool rows without a diff, and OpenCode's formatter does not run on them.
- `ls` inside cage shows hidden file names, the same as native read-only agents.
- `agy`'s profile stores each turn's MCP config, including its token, and bridged results.
  - Guard hides and denies that profile for every agent, except `antigravity-cli/brain`, where `agy` saves overflowed tool output that lanes read back.
- Bridged images are not resized.
- MCP cancel notifications are ignored.

## Guard

`guard/server.ts` denies recognized shell Grok launches and OpenCode API calls.
The shared tool checks live in `guard/review.ts`, called from `tool.execute.before` and by the [Gemini](#gemini) plugin.
For agents outside the read-only set, the credential gate covers recognized Claude `.credentials.json`, OpenCode `auth.json`, `service[-…].json`, and `opencode.db[-wal|-shm]`, and `agy`'s profile outside `antigravity-cli/brain` in shell arguments, native `read`, `write`, and `edit`, and `grep` pointed directly at a file.
Native checks expand `~`, resolve relative paths against the session directory, and check realpaths.
Shell checks also resolve literal operands and redirect targets against the command's cwd and every literal `cd`/`chdir`/`pushd` destination, checking realpaths only for credential-like basenames; variables, globs, substitutions, and runtime-built paths are not resolved.
Credential basenames used as text can also reject when resolved against those candidate directories.
`cd` is allowed; the guard folds one leading `cd <literal path>` into `workdir` after checking that the directory exists.
For agents outside the read-only set, it rejects `grep` and `glob` roots at or above home or inside a hidden path, and checks commit-message shape.
For agents outside the read-only set, `rm`, `find -delete`, and `find -exec rm` require paths inside `/tmp/opencode/`, written literally or through variables assigned earlier in the command, absolute or relative to a known `workdir` or `cd <dir> &&` target.
`rm` is checked behind any command (`noglob`, `busybox`, and others) except subcommand-style `rm` under `docker`, `podman`, `nerdctl`, `kubectl`, `npm`, `pnpm`, `yarn`, `bun`, `cargo`, and `brew`; any `rm` inside a `find -exec` segment is handled by the `find` check (allowing `{}`).
For `rm`, this includes quoted, unchanged variables from `mktemp -d` under `/tmp/opencode` earlier in the same command; templates must not contain whitespace.
They reject `..`, trailing slashes, and top-level globs; `xargs rm` is denied, and other deletion goes through `trash -- <path>`.
Outside the read-only set, every agent except `collab` and `build/git` may run recognized read-only Git commands or Git in self-contained scratch repos under `/tmp/opencode`, subject to path and config checks; mutations in real repos are denied.
Denial checks evaluate every plausible reading of a command without wrapper option tables, scanning options across `--` without value-arity assumptions and matching abbreviated long options by prefix (such as `env --chd=`).
Because words behind any command are scanned, `echo git push --force` asks or denies and `echo rm …` denies; data heredocs still do not count as commands.
When a runtime-expanded word might be a command name only because a preceding wrapper flag's value arity is ambiguous, the denial names that flag and how to disambiguate it (`--` before the command, `--option=value`, or dropping the flag).
The parser inspects nested arithmetic, including command substitutions and base prefixes, within its eight-level nesting limit.
It also walks literal script bodies passed through `flock`, `watch` (even with a later `-x`), and `strace`/`ltrace -o '|cmd'`, and always unwraps `unshare` and `uv run`, denying unknown or chroot starts it cannot see.
Unrecoverable script handoffs (including `xargs -I` with a shell body and `find -exec sh -c '…{}…'`), active `trap` bodies, and `parallel` remain rejected outside the read-only set, except for `collab` and `build/git`.
For agents outside the read-only set, the guard rejects `git push` and `git send-pack` calls whose repo paths (`-C`, `--git-dir`, or `--work-tree`) or arguments expand at run time; write them literally or through variables assigned earlier in the command.
For `collab` and `build/git`, pushes and `send-pack` ask for approval under `~/LeadPier`, when the repo is otherwise undeterminable, when hidden behind redirects (including named fds like `{x}>`) or a `git-core` helper path, with `-c` or `--config-env`, or for forced or destructive forms; other pushes pass this guard.
For all agents, including `collab` and `build/git`, it denies Git options that run a program, such as `--upload-pack`, `--receive-pack`, `--exec`, or `rebase -x`, and their abbreviations.
For every agent outside the read-only set, including `collab` and `build/git`, it also denies dangerous `git -c`, `--config`, and `--config-env` settings (including short clusters like `-4c` and abbreviations like `--conf=`).
For all agents, `systemd-run` requires `--collect` (`-G`) or a `CollectMode=` property; use `-p CollectMode=inactive` to retain failed units deliberately, accepting hyprd health alerts.
It also denies `systemd-run` unit names starting with `cogikyo-` (`-u`/`--unit`), protecting the voice-loop bus rights granted to `cogikyo-voice.scope`, which Cullyn's key creates.
For every agent and tool, it rejects inputs naming nonexistent default XDG user dirs relocated by `user-dirs.dirs`, except fields used to find or fix stale references and `subagent` prompts, and lists the configured dirs in the denial.
Use `x` for X search, `tools.opencode.sessions` inside `execute` for history, and `session_create` for new sessions.
Prefer `workdir`; V2 resolves each remaining `cd` argument against it, so chained relative changes can cause false external-directory prompts.
The `context`, `compaction`, and `generate` hooks describe parallel calls and the shell environment: zsh without unquoted `$var` word-splitting outside the read-only set, or offline `bash -c` in cage for read-only agents.

For `review/*`, `scout/*`, `verify/source`, and `verify/web`, the guard allows only `shell`, `read`, `grep`, `glob`, `webfetch`, `websearch`, `skill`, `opencode_sessions`, `x`, `diagram`, `workflow`, `execute`, and `subagent`; agent permissions can restrict these further.
The global `subagent` deny keeps that last one from every read-only agent except `review/entropy`, whose frontmatter allows its helpers.
Their Code Mode scripts may call only those tools, including `tools.opencode.sessions`; the guard denies script `fetch` (use `webfetch`).
Their frontmatter hides `opencode_session_*` and `opencode_worktree_*` management tools from the catalog, and `verify/web` denies all shell calls.
Each read-only shell runs through [cage](../../../cmds/cmd/cage/README.md) offline, with writes limited to `~/.local/state/opencode/scratch/<session-id>`.
Caged calls skip the parser's dynamic-name, stdin, opaque-handoff, nesting, `rm`, and Git checks, because cage enforces those effects.
Program-running Git options and credential checks still apply.
`$OPENCODE_SCRATCH` names that child's directory; `tmp/` and `cache/` are fully mutable, while the root permits creation and in-place writes without deletion, rename, truncation, or symlink creation.
The pattern checks are guardrails; cage supplies the kernel sandbox and trusts the scratch path the guard computes.

For read-only agents, `paths.ts` supplies one hide inventory for cage and native `read`, `grep`, `glob`, and file-backed `diagram` checks; the uncaged credential gate uses the narrower set in `policy.ts`.
The inventory hides `agy`'s profile whole and keeps `antigravity-cli/brain` readable (`kept`, passed to cage via `--keep`).
Native checks walk symlinks hop by hop and refuse hidden paths, `/proc`, `/sys`, `/dev`, and every session's scratch `tmp/` or `cache/`; recursive searches also refuse home and roots containing scratch `tmp/` or `cache/`.
For every agent, including Gemini's bridged calls, `guard/server.ts` wraps `grep` and `glob` to drop results under this hide inventory by lexical path or realpath, so searches may start above hidden paths.
When screening drops truncated results, reruns stop after 3 rounds or a 30-second budget and return the screened results marked truncated.
Read scratch `tmp/` or `cache/` files through the shell, or copy them to new names at the scratch root for native reads; overwriting an existing root file fails.
An inventory entry that cannot be resolved or listed is hidden as a whole subtree; a symlink beneath an unlistable or unresolvable hide is not followed.
Unlisted secret stores remain readable, so the inventory needs maintenance.

Read-only `webfetch` is replaced by `fetch.ts`: public HTTP(S) on ports 80/443, with connect-time DNS vetting, connected-peer checks, and checks on every redirect.
It rejects non-public addresses and local interface prefixes, including locally assigned public ranges.
HTML conversion runs in a killable Worker (`convert.ts`) using `html.ts`, a verbatim copy of OpenCode v2.0.29's `packages/core/src/tool/html-markdown.ts` that requires `htmlparser2` 8.0.2.

Read-only shell and `x` calls append request and return records to `~/.local/state/opencode/scratch/runs.jsonl`, keyed by call and session IDs.
Return records contain tool status, not process exit codes; the index is not proof of command success.
`/tmp/opencode` is a disk-backed bind mount of `~/.cache/opencode-tmp` and survives reboots.
System tmpfiles (`system/etc/tmpfiles.d/opencode-tmp.conf`) remove unused entries in `/tmp/opencode` and caged scratch as root after 3 days; the index is exempt.
There is no tool, key, or TUI slot for the index.

`guard/harness/` compares the guard against a baseline rev with `bun`: `probe.ts` checks curated cases (`cases.ts`) and terminal `prompts()`, `fuzz.ts` checks generated mutations, and `replay.ts <history.json…>` replays recorded shell calls.
Set `BASE` (default `cd7d96c1`), `OUT` (default `${OPENCODE_SCRATCH:-/tmp/opencode}/guard-harness`), or `AGENTS` for `fuzz.ts` (default `collab,build/general,review/debug`) to adjust a run.

## Cogikyo

`cogikyo/server.ts` registers only for plugin locations resolving under `~/cogikyo`.
After successful `edit`, `write`, or `patch` calls, it appends a **Public dotfiles** notice when a target resolves into `~/dotfiles`, following symlinks through helpers from `guard/paths.ts`.
Notices are deduplicated in memory per session and top-level dotfiles path within each location instance.

The plugin never blocks or changes the write; hook errors log `cogikyo: dotfiles notice skipped`.
Shell writes (`cp`, `tee`, `>`) are not covered.

## Terminal

`terminal/server.ts` exposes `terminal` with `command`, `reason`, and optional `workdir`, which defaults to the session directory.
Use it only for commands that need the user's keyboard, including passwords, interactive prompts, and `dctl` root stages; run other commands in `shell`.
Only top-level sessions can call it; children get an error before any question or window and must return the command to their parent as a blocker.
The `reason` tells the user what the command does and what they must type or answer.
Before calling, state your results, show the command in a bash fence, and explain what the user must do, because the window covers the session.
Before asking, the plugin rejects control characters except newline and tab in `command`; `reason` and `workdir` allow none.
It calls `ctx.permission.assert`, so configured deny rules still block.

After its guards, the plugin calls the registered `question` tool after transforms, so the question plugin's wrapper applies; a missing tool fails with `Tool.Error`.
The popup has one question with header `Terminal`, showing the reason, `in <dir>` when the workdir differs from the session directory, then the command.
It offers `Run it`, `Run manually`, and the question tool's built-in typed answer.
`Run manually` opens no window and runs nothing; the result tells the model to reply with a copyable bash fence, short context, and then end its turn.
A typed answer also opens no window and runs nothing; the user's text returns to the model.
Esc propagates the question's dismissal and ends the step.
Completed results carry `choice: "run" | "manual" | "typed"` metadata.

`terminal/tui.tsx` follows `metadata.tool` to a tool part named `terminal`, then turns the docked form's left border and `Questions` title the theme's error red.
It renders the command as a bash code block with code-blocks' Vagari style and bash parser, matching transcript fences; if the exact command text is not found, the command stays plain.
Other forms keep host colors, and the plugin restores its changes when the form goes away.

`Run it` opens a fresh Kitty window with class `opencode-terminal`, `--title` set to `<session title> — <reason>`, and remote control disabled.
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

`hyprd/tui.ts` loads the notification, pane-context, focus, and session-open plugins together.
`hyprd/events.ts` shares one `hyprd subscribe workspace view navigate` child per TUI across plugin loads and hot-reload generations.
It replays the latest workspace and view state to new listeners; navigation requests are not replayed.
Browser workspace controls, statusline animations, and `hyprd/open.ts` consume this bus.

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

`hyprd/context.ts` checks the displayed root, prompt text, tab count, and pending input on route changes and each second, publishing changes to `$XDG_RUNTIME_DIR/opencode/tui/<TUI-pid>.json`, with a heartbeat each minute.
Pending input covers current permissions and forms plus unresolved forms seen before navigation, whose partial answers remain in host memory.
It requires `XDG_RUNTIME_DIR` and `KITTY_WINDOW_ID`, records the process `start` time from `/proc/self/stat` and report time `at`, and removes the file on disposal.
It creates the directory with mode `0700` and the file with mode `0600`, then publishes by atomic rename.
Hyprd matches the record to a discovered TUI pane and checks the process start time before using it for [recycling gates, relaunch, and session-open confirmation](../../../cmds/cmd/hyprd/README.md#opencode).

`hyprd/focus.ts` reports renderer focus and blur through `hyprd/rpc.ts`, plus root-session changes and a 60-second heartbeat while focused.
It reports `focused: false` on disposal.
`hyprd/server.ts` emits each report as the ephemeral `rpc.hyprd.focus` event with `{ tui, session, focused }`, where `tui` is the TUI PID and `session` is the root ID or `""`.
There is no tool, key, or TUI slot.

## Sessions

`sessions/server.ts` exposes read-only history queries over `${XDG_DATA_HOME:-~/.local/share}/opencode/opencode.db` as `tools.opencode.sessions` inside `execute`, returning `{ rows, hidden }`.
Agents denied `execute`, such as `verify/browser`, get the direct `opencode_sessions` tool with rendered text instead.
Queries default to 12 rows for `list` and 20 otherwise, accept `limit` up to 1000, and keep a 30-second subprocess deadline.
Code Mode strings stop at 50,000 characters with `…`; `text` and `part` rows include `length`, and `part` pages output with `from`.
The direct tool keeps 800-character excerpts.
Host-visible tool output stops at 2,000 lines or 50 KiB; return counts, IDs, and short excerpts rather than raw rows from scripts.
Use `list`, `text`, `reasoning`, `tools`, or `part` before raw `sql`.
Assistant item addresses are `<message ID>#<index>`; bare message IDs read non-assistant text.
The index follows the host's order: `session_part` rows by `ordinal`, then any inline `data.content[]` items.
Raw queries accept one bound `SELECT` or `WITH` over `session_v2`, `session_message`, `session_part`, `project`, and `sqlite_schema`, with `json_each` and `json_tree` allowed.
Message and part reads must use a bounded index seek, such as on `session_id` or a part's `message_id`.

A title tag is an uppercase prefix such as `[CLOSED,FIX] ` or `[REFUSALS] `, defined in `sessions/tags.ts`.
It holds an optional state (`CLOSED`, `ARCHIVED`, or `RESEARCH`) first, then flags (`FIX`, `ISSUES`, `REVIEWME`, `TODO`, or `REFUSALS`), with at least one name.
Only tags with a state count as closed; unknown names or misplaced states make a prefix unrecognized.
Writes deduplicate flags in the order above; see [done's tag criteria](../skills/orchestration/done/SKILL.md#tags) to choose them.

`sessions/query.ts` filters session and message views to omit `REFUSALS` sessions and their descendants through child or fork links; the TUI still shows them.
Successful Code Mode queries return the store-wide `hidden` count; rendered text appends it when nonzero.
Direct session and part lookups refuse hidden targets.
Visible text results and query errors replace exact hidden-title copies with `[hidden by refusals]` before clipping or paging.
Plain-title copies are included only with at least two words and 10 characters, excluding default titles; matching is case-sensitive and covers up to two JSON-escape layers.

`sessions/server.ts` exposes `tools.opencode.session_tag` in Code Mode with optional lowercase `state`, `flags`, and `add` (at least one required).
It rewrites the title tag, keeps any `refusals` flag and the rest of the title, and skips unchanged titles.
Omitting `state` keeps the current one, leaving a live session open.
`flags` replaces flags, `add` adds (after `flags` if both are passed), and omitting both keeps them.
It refuses a `todo` flag on a session without a state, so open sessions never show `[TODO]`.
It refuses another session flagged `REFUSALS` without echoing its title.
Only top-level sessions can call it or be tagged; `sessionID` defaults to the caller, and a missing title becomes `New session - <ISO time>`.

`sessions/server.ts` also exposes `tools.opencode.session_model` to switch a top-level session's stored model to a `provider/model#variant` ref.
It validates the ref against available models and starts no turn; send mail to wake the session.
Only top-level sessions can call it; it refuses child sessions and other sessions flagged `REFUSALS`.

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
With hyprd, Open sorts each group by the caller's workspace first, then workspace number, then newest update; rows on the caller's workspace are bold.
Recent and Open without hyprd put sessions needing permission or question input and non-`REFUSALS` sessions with parked mail first within Running and Open, newest first within each rank.
The Open group's footers use the statusline's context-pressure colors by time since last update, from blue under 15 minutes to pink at three days or more.
Recent adds a Closed group last, newest first.
Closed titles are muted, with `RESEARCH` titles in a faint scout blue, unless selected or current; rows retain pending-input and running markers.
In both pickers, an envelope after a session title marks parked mail for open, non-`REFUSALS` sessions.
The marker and mail ranking share one `mail.parked` RPC request on opening, refreshed on inbox events and `server.connected`, instead of querying each session's inbox.

In Recent, `ctrl+d` deletes and `<leader>r` renames the selected session.

Selections in Open and Recent use [`hyprd open`](../../../cmds/cmd/hyprd/README.md#opencode): focus an existing TUI, navigate locally on the target workspace, or open in that workspace's agents Kitty.
Without Kitty pane identity or a hyprd socket, selections navigate locally; other routing failures show an error toast.

The plugin relies on host behavior beyond the published types: extra option fields such as `titleView` and `gutter`, selection preservation with a defined `current`, the `modal` mode after dialog replacement, and a home route carrying `location`.
If the host stops forwarding the extra row fields, rows degrade to plain titles without muting or status markers.
Review the selection and navigation assumptions after an OpenCode update.

## Launch

`launch/server.ts` adds `session_create` to the `opencode` Code Mode namespace, so agents call `tools.opencode.session_create` through `execute`.
Inputs are `directory`, `title`, and optional `prompt`, `agent`, and `model` (`provider/model#variant`); the directory must be absolute or start with `~` and must exist.
It creates a top-level session with session metadata `origin` set to the caller's session ID and its model set at creation before the first turn, then sends `prompt` as the first message to start a turn.
Without `agent`, the session uses the configured default primary agent; a named agent must be a visible primary agent at that directory.
Without `model`, the session uses the calling session's model; an explicit `model` is validated against available models via `sessions/model.ts` and sets `pinned: true` in session metadata so Anthropic first-turn ranking skips that session.
Only top-level sessions can call it, and the new session is not a lane.
The plugin does not ask for permission through `ctx.permission.assert`, so the user's request in chat is the approval; the guard's `opencode api` denial points here.
If the prompt fails, the error names the created session, so the agent does not retry and create a duplicate.
There is no persistent state, key, or TUI slot.

## Diagram

`diagram/server.ts` exposes `diagram` for display-cell geometry and connectivity checks on documentation diagrams and trees.
Pass `text`, or `file` with `start` and `end`; optional `prefix` and `width` describe the host layout.
File mode accepts files under the project session directory or the caller's own scratch directory; pass text for other layouts.
There is no persistent state, key, or TUI slot.

## X search

`x/server.ts` exposes `x` with a self-contained `brief` and runs Grok CLI native X search with a ten-minute deadline.
`x` runs on Grok CLI auth, so its calls spend the xAI usage that `xai/*` lane launches check.
Each call starts in an empty temporary directory under `~/.local/state/opencode/x/`, removed when the call finishes.
Fixed flags disallow local tools and add explicit denials, disable web search and child agents, and use permission mode `default`; Grok's own sandbox is off, and this call does not run in cage.
The brief is NFKC-normalized; control and format characters are rejected except newline and tab.
Bare `@handle` text becomes `X user handle <name>`, other `@` text is rejected, and lines starting with `/` or `!` are rejected before the `X research brief:` prefix is added.
Scouts and source/web verifiers can call `x`; Collab keeps it for `/x` and delegates other X research.
Returned claims remain untrusted evidence.
The plugin rejects near-duplicate briefs within a session and keeps that history only in memory.
There is no key, TUI slot, or separate state file.

## Browser

`browser/server.ts` exposes `chrome-devtools_*` from `chrome-devtools-mcp` 1.13.0 and `firefox-devtools_*` from Mozilla's `@mozilla/firefox-devtools-mcp` 0.10.7.
Each session gets one isolated process per engine used: headed Chromium or headless Firefox.
Tool names and arguments differ between engines.
Chromium keeps mobile, touch, DPR, performance traces, Lighthouse, and LeadPier preview auto-login; Firefox has no DPR, touch, mobile-UA, throttling, or color-scheme emulation, traces, or Lighthouse.
Firefox viewport resizing checks layout, not mobile-device behavior.

`browser/mcp.ts` launches Chromium with `--no-page-id-routing` and network-scheme allowlist patterns.
Firefox has no URL allowlist; the plugin limits direct `navigate_page` and `new_page` URLs in both engines to HTTP(S) or `about:blank`.
The catalog cache key is `catalog/<engine>/<hash of launch command>`.
Both engines need `npx` and their executable: `/usr/bin/chromium` or `/usr/bin/firefox-developer-edition`.
`browser/tui.tsx` uses `hyprd` to list and focus headed browser workspaces after `sidebar.content`; headless Firefox has no window.

For signed-in Firefox checks, the user creates the agent template, signs in to the needed sites, then closes its window:

```bash
mkdir -p ~/.config/mozilla/firefox/opencode-agent && firefox-developer-edition --new-instance --profile ~/.config/mozilla/firefox/opencode-agent
```

If that template exists, `browser/profile.ts` copies it into a unique directory under `~/.config/mozilla/firefox/opencode-agent-runs/` for each Firefox session, excluding saved passwords, Sync state, and locks.
The plugin closes both engines and forgets preview login state on execution success, failure, interruption, or session deletion, and removes the Firefox copy after process exit.
The user `headless-reap.timer` runs every 15 minutes: it kills headless Chromium and Firefox older than 3 hours and runs `profile-reap` to remove orphaned copies older than 10 minutes.
Headed browsers and profiles still referenced by a user process are excluded from reaping.
Agents may use signed-in sessions but never type credentials, create accounts, or change account settings; when signed out, they report `sign-in needed: <site>` and stop that check.

`browser/preview.ts` provides Chromium-only auto-login on direct `navigate_page` URL calls or `new_page` calls to `https://lpadmin.cullyn.dev` or `https://lpuser.cullyn.dev`.
It reads the `leadpier-preview` keyring entry set with `cogictl secret set leadpier-preview` and authenticates as preview user 12919 (`cullyn@trendcapital.com`, admin), caching login state for the execution and browser User-Agent.
Matching URL navigation seeds `authentication` localStorage before the app loads when absent or when replacing a token issued during that execution, including after relaunches or in new contexts.
Clicked links, redirects, and back/forward/reload do not seed the session; navigate to the URL directly.

The plugin hides Chromium's `take_heapsnapshot`, strips `saveTo` and `preview` from tool inputs, and rejects calls supplying either field.
Chromium uses `--redactNetworkHeaders`; `browser/redact.ts` applies Chrome's header allowlist to Firefox network output.
Signed-in Firefox runs collect no request or response bodies; throwaway-profile Firefox runs still collect bodies, and URLs are not redacted in any run.
Sensitive page content remains unredacted; agents must not read or return authentication data or other secrets.

## Statusline

`queue/tui.tsx` replaces the queued-prompt dock above the prompt with one row per queued prompt, at most three and then `+N more`, and a count on the right; a click opens the queued prompts dialog.
It hides the native dock only after it matches the dock's shape and `N queued` label; when that match fails, both docks show.

`sidebar/tui.tsx` adds `<leader>a` and `/sidebar`, which move the session sidebar to the left and back; the choice persists in plugin storage.
V2 has no sidebar-side setting, so it polls every 250 ms for the renderer node with id `session-pane` and reverses its parent row while the 42-cell sidebar is the sibling.
That id and width are upstream internals in `session-frame.tsx` and `ui/layout.ts`; when they change, the toggle silently stops moving the sidebar.
`<leader>a` replaces `agent.list`, which is unbound in `cli.json`; `/agents` and `shift+tab` still reach agents.

`statusline/tui.tsx` always replaces `prompt.footer.status` with directory and Git status, so the native spinner and `esc interrupt` hint never show.
It appends a context-pressure bar to `prompt.footer`, measured against the model input or context limit minus the compaction buffer.
The gauge starts with ㊌ (water) while idle and ㊋ (fire) while the session runs.
While running, the fire glyph fades between the gauge color and up to 35% red over a two-second cycle; a wave sweeps only the dots, and a newly filled dot flashes.
V2 has no slot on the prompt's model row, so the bar is absolutely positioned two rows up into the prompt box; a prompt layout change upstream can misplace it.
It replaces the built-in prompt footer, disabled in `cli.json`, with running subagent and shell counts and context size, without session cost.
It replaces the built-in sidebar footer, also disabled, with the OpenCode version line.
A linked worktree shows its main repository path and a worktree glyph in place of the branch glyph.
While the session runs, `statusline/working.tsx` widens the prompt's left border to a half cell and pulses it in a brightened agent color.
The border pulse, fire fade, and gauge wave stop when `animations` is off in `cli.json`; the wide border stays.
`lead` in `statusline/working.tsx` gives a session its work color: the role color of the running sessions in its family, read from each agent name's prefix, with build before review, scout, and verify.
Running sessions with none of those roles, such as a root session that works alone, get the build color; a family with nothing running has no work color.
While a session's family works, `statusline/light.ts` thins the `▀` row under the prompt box to an upper quarter block (`🮂`) and moves one light in the work color along it, from side to side once every 4 seconds.
Typing in a focused terminal parks the light at the center as a broad dim glow, and so does `animations` off.
The light eases in and out over 600 ms, with the row at three eighths (`🮃`) for the first and last half of that time.
The row stays the host's own while nothing runs.
`Pace` in `statusline/pace.tsx` hosts the light: it paints after the prompt, skips `modal` and `menu` keymap modes, and holds the animation clock only while the light moves.
Another plugin hides the light on a prompt with `veil` from `statusline/light.ts`, as drive does while its chevrons show.
A draw error stops the light and shows a `Statusline: light off, <reason>` toast.
The shared clock in `statusline/working.tsx` runs only while the terminal has focus, so every clock-driven animation holds its last frame in a blurred window.
`statusline/view.ts` uses the shared [hyprd bus](#hyprd) to also pause them in out-of-view panes; unknown view state leaves them running.

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

Each successful subagent notice gets an indented box in the lane's role color with a flush right edge bar.
In boxed replies, Byline hides the `---` rules and wraps text inside the box.
The notice puts the subagent's description on the left and `icon Scope / icon Agent · model/variant · tokens` on the right, truncating the description first when space is short.
Lane icons, colors, model names, and token counts use the same source as the Lanes sidebar.

New lane notices reveal the whole row in about 1.2 s: build forge, review scan, scout ping, or verify decode.
Newly delivered mail opens and types its header behind a pink cursor in up to about 2.6 s, ending with a blink and postmark stamp.
The mail body fades as typing ends.
After their reveals, lane and delivered-mail icons breathe until their reply box closes or their slot drops; a queued row, a retry footer, or a still-streaming reply (even a partial `--`) does not by itself stop breathing.
Newly sent mail types its header, then closes and seals in at most about 1.1 s without breathing.
Both mail reveals play only for fresh items; received freshness uses delivery time, so long-parked mail still reveals on arrival.
Set `animations` to `false` in `cli.json` to keep these effects static.

Failed and cancelled lanes show a red `✗` and the state word without a box.

User prompts, question-tool rows, and retry footers split boxes visually.
A lane notice or delivered mail whose reply block has not opened stays pending across those rows.
Collab's next `---` blocks take matching slots in delivery order within each kind.
A lead line starting with `Mail from` takes a mail slot; any other lead line takes a lane slot, or a mail slot when no lane is pending.
Matched blocks use the lane's role color or mail pink and join the notice or mail box when adjacent.
Blocks without a matching slot stay unboxed.

Pending slots drop at turn end, when a finished reply contains no `---`, or when a reply after a prompt starts with text instead of `---`.
A new mail delivery replaces the previous pending mail slot.

Delivered mail and the sender's completed `mail` tool call render in pink boxes with the peer, subject, and body.
The header shows the kind glyph, keeps `<kind> · <time>` on the right, and cuts long subjects with `…` instead of wrapping.
At low verbosity the sender's `mail` call folds into `+ N tools`; the host's `grouping/session.ts` exempts only `question` from tool grouping.

Byline matches each queued outcome to its call by message ID and call ID.
It replaces the outcome row with a rounded box titled with the question count.
The box sits below the reply the form opened under, before the user's next message.
The call's own row stays `→ Asked N questions`.
Older synchronous question parts still show their box at the call.

Each question shows its header, text, and all options with descriptions.
Picked options get `✓`; the recommended option gets `★`, green when picked and orange otherwise.
Typed answers get `✎` and keep their paragraphs.
Answered boxes use accent borders and titles.
Outcome footers read `Dismissed`, `Withdrawn for waiting mail`, or `Not answered`.
For those three outcomes, the border, title, and footer use the theme's muted tone.
The live form stays native.
Unrecognized renderer layouts leave the plain transcript in place.
After host updates, recheck `metadata.answers` and answer values matching option labels.

## Code blocks

`code-blocks/tui.ts` renders supported fenced code blocks as padded cards with a raised background and a muted top-right language label.
Cards always use Vagari highlighting, independent of the TUI theme.
Converted nvim-treesitter queries use private `vagari.<ft>` parsers for cards; the hand-maintained `sql/` parser is public because OpenCode has no SQL parser.

Add a parser folder at `config/opencode/tree-sitter/<ft>/` with `highlights.scm` and `tree-sitter-<ft>.wasm`; optional `manifest.json` sets `filetype`, `wasm`, and fence `aliases`.
Sync needs `~/vagari/nvim`, the nvim-treesitter `main` checkout at `~/.local/share/nvim/lazy/nvim-treesitter`, headless Neovim, and the installed OpenCode source in the `src` cache (`src get github.com/anomalyco/opencode@<version>`).
From `config/opencode`, run `bun plugins/code-blocks/sync.ts` after changes to Vagari, nvim-treesitter queries, or the host parser catalog.
Sync regenerates queries, manifests, and resolved Vagari styles; it replaces `tree-sitter/` but keeps every parser folder whose manifest sets no `filetype`, and fails if such a folder shares a name with a generated one.

## Vagari theme

To tune colors, edit `themes/vagari.jsonc` and save; every open TUI window rebuilds the generated `themes/vagari.json` and reloads within about a second.
Write each color as a Vagari name such as `glc_0`, a `$` reference such as `$text.base`, or `transparent`; `categorical` lists hue names.
A bad value shows an error toast with its token path, and `vagari.json` keeps the last good build.
The `defs` block is a copy of the 124 colors in `~/vagari/vagari/colors.toml`, and nothing syncs the two.

In each window, `vagari/tui.ts` watches the source and runs `vagari/build.ts`, which writes a name on a hue ramp as a `$hue.<ramp>.<step>` reference and any other name as raw hex.
OpenCode lightens or darkens a referenced token by stepping along its ramp; a raw hex token does not move.
A name takes the first ramp whose members all share its family (`glc` for `glc_0`), else the first ramp that holds it, at its first step next to a different color.
A `base` token gets a reference only when every mode resolves it to the same one.
The build writes through a temporary file and a rename, and only when the output changes, so windows can build at once.

The build leaves the `transcript` block out of `vagari.json`; [Transcript](#transcript) reads it from the source.
A window whose build output or `transcript` block differs from its last sends `SIGUSR2` to its own process, which makes the TUI re-read custom themes and Transcript re-read its block.
It sends the signal only while a handler listens, since an unhandled `SIGUSR2` ends the process.
From `config/opencode`, `bun plugins/vagari/build.ts` builds by hand and prints errors, but it signals no window.

## Transcript

To recolor inline code, links, and headings in replies, edit the `transcript` block of `themes/vagari.jsonc` and save; every open window applies it.
Each value is a Vagari name, optionally followed by `bold` as in `"session": "prp_3 bold"`, and `transcript/tui.ts` picks a code color by what the span holds.

| Key            | Colors                                                                        |
| -------------- | ----------------------------------------------------------------------------- |
| `code.path`    | Paths, file names, `file:line`, URLs                                          |
| `code.command` | First word is a program on `PATH` or a builtin like `cd`                      |
| `code.danger`  | Destructive or privileged commands, such as `rm -rf`                          |
| `code.symbol`  | Names from code, such as `inlineHook`, `session_part`, `lead()`, `SIGUSR2`    |
| `code.key`     | Settings and switches: dot paths, `--flags`, `$VARS`, such as `markdown.code` |
| `code.number`  | Numbers, versions, hashes, `:line`, `true`, `null`                            |
| `code.other`   | Anything else                                                                 |
| `session`      | Session links such as `@Reply styles`, drawn bold                             |
| `link`         | Web links, including `localhost` and Markdown `[label](https://…)` links      |
| `icons`        | `true` puts a Nerd Font glyph before each link, `false` leaves them off       |
| `h1` to `h6`   | Headings at that level                                                        |

A span with spaces is `danger`, `command`, `number` for a timestamp, or `other`, and a span that holds a `#rrggbb` color draws in that color.
A single word is a `key` when it holds a dot path (`a.b`) or starts with `-` or `$`, and a `symbol` otherwise, so `foo.bar()`, `@scope/pkg`, and `ALL_CAPS` names are symbols.
`danger` covers privilege commands such as `sudo`, recursive or forced `rm`, disk writers such as `dd` and `mkfs`, history rewrites such as `git push --force` and `git reset --hard`, service stops, forced kills, power commands, and `dctl install` or `dctl iso usb|release`, anywhere in a `&&`, `;`, or `|` chain.
Plain `rm file` and `trash` stay `command`, and the rule table is `DANGER` in `transcript/kind.ts`.
Links also draw in `code.path` when they target a `file:` URL, whether from a path in prose, a `file://` URL, or a Markdown `[label](file:///…)` link.
With `icons`, a link gets a globe, and a file link gets a glyph for its extension, a folder glyph for a directory, or a plain file glyph.
The glyph is a separate span in the link color without underline or link target, so it joins the copied text and a click on it does nothing; table cells and session links get none.
`transcript/icons.ts` holds the extension table, which uses nvim-web-devicons glyphs, and xplr's `node_types.lua` in `config/xplr/` supplied the folder glyph.
A missing key keeps the theme's color, except that a missing `code.danger` uses `code.command` and a missing `code.key` uses `code.symbol`; headings keep their bold and level-1 underline, and session links keep their underline but are bold only when their value says `bold`.
Code inside a link and a span that is still streaming keep `markdown.code`.
Only spans drawn in Vagari's `markdown.code`, `markdown.link`, `markdown.linkText`, `markdown.heading`, and `text.base` colors change, so other themes and the muted reasoning style keep their colors, even when another plugin hooks reasoning blocks.
A bad name or key shows a `Vagari theme` error toast, and the last good colors stay.

OpenCode has no plugin API for inline spans, so the plugin wraps the `inlineHook`, `inlineHookFilter`, and `syntaxStyle` setters on OpenTUI's `MarkdownRenderable` prototype.
Setup fails with an error toast if OpenTUI drops one of those accessors.
Only renderables that set an inline hook change: reply text, compaction summaries, and `/btw` answers.
The wrapper colors code spans before OpenCode's session-link hook runs and recolors session links and every `link` span after it, finding session links by the `ses_` click target that hook sets.
Its getters return what callers set, so another plugin that wraps the same hook still finds its own wrapper.
It adds a backtick and `www.` to a non-empty hook filter, which moves every block with inline code or a bare `www.` link from OpenTUI's native write path to its slower host path; the measured cost is a few microseconds per block.
Headings use a per-theme copy of the syntax style, so diffs and file views of Markdown keep `markdown.heading`; without heading keys, renderables keep the shared style.
A palette change swaps in a fresh copy, because OpenTUI does not repaint a style that changes in place, and reruns the hook where the style stays the same.
Clearing a renderable's hook gives it back the shared style.
The plugin lists `PATH` executables when the theme loads, so a program installed later counts as a command after the next theme save.
Disposing restores the prototype and every hooked renderable.

## Paths

`paths/tui.tsx` links file paths in transcript Markdown and runs `xdg-open` on a plain click.
Stock OpenCode does not open `file://` links: its `openUrl` rejects anything but http(s) with an error toast, so the plugin handles the click on each transcript Markdown itself.
It chains the host's `ses_` session-link hook, adds `/` to that hook's filter, and coexists with [Transcript](#transcript)'s prototype setter wrapping.

`paths/split.ts` links only absolute and `~/` paths, and `file:///` URLs, that exist when the block renders; a trailing `:line` stays outside the link.
A `file://` URL links to itself, other schemes such as `http://` are left to OpenTUI's own autolinks, and [Transcript](#transcript) draws every `file:` link in the path color.
A code span links only when it holds nothing but the path, so commands stay copyable.
Fenced code blocks and tool output never link.
A file created after its paragraph rendered stays plain until that block runs the hook again.

Through `config/mimeapps.list`, text and source types open with `share/applications/hyprd-edit.desktop` (`hyprd edit %f`), in the workspace nvim.
`file` sniffs the type from the file's contents, and a type missing from that list falls through to xdg-open's browser fallback.

## Workflow

`workflow/server.ts` stores one workflow per root session in plugin KV `flow/<root>`: fence source, revision, step marks, checklist ticks, repair rounds, child links, and the revision diff.
The `workflow` tool reads status with `{}` and adopts a fence with `start`; see the [workflow skill](../skills/prose/diagrams/workflow/SKILL.md) for fence rules.
Start and status show root context, headroom to the 500K compact nudge, and unfinished steps × 18K as a rough cost; an estimate above headroom warns without imposing a step cap.
A new revision matches steps by owner and headline, carries unchanged results, and reports added, renumbered, changed, and dropped steps.
A gate's result carries only when the steps it judges, through its `after` dependencies and repair loop, are unchanged.
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
Delegated rows whose model has a `#variant` show a brain glyph and the variant left of the model.
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

`bullets/tui.ts` draws unordered list markers as `•` at the top level and `◦` when nested; ordered lists keep their numbers.
OpenTUI 0.9.9 hard-codes `-` in the private `MarkdownRenderable.updateList`, so the plugin wraps it and rewrites the markers afterward.
An OpenTUI upgrade can rename that method or the `_doc` and `_shown` fields it reads; setup throws if `updateList` disappears.

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
