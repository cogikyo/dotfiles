# OpenCode Plugins

Plugins in `config/opencode/plugins/` are loaded directly from source by OpenCode and Bun.
`config/opencode/tsconfig.json` is only for typechecking; there is no build artifact.

OpenCode has two plugin runtimes:

- **Server plugins** live where tools and hooks run; register them in `config/opencode/opencode.json`.
- **TUI plugins** render inside the terminal UI; register them in `config/opencode/tui.json`.

Changes to plugin entries or source only take effect after an OpenCode restart.
Running sessions keep the loaded plugin set.

## Plugin map

| Feature                | Entrypoint                         | ID                              | Runtime |
| ---------------------- | ---------------------------------- | ------------------------------- | ------- |
| Claude auth            | `anthropic/index.ts`               | (two provider hooks)            | server  |
| Delegate task          | `delegate/index.ts`                | `delegate-task`                 | server  |
| Usage status tool      | `usage/tool.ts`                    | `usage-status`                  | server  |
| Hyprland notifications | `hyprd/notify.ts`                  | `hyprd-notify`                  | server  |
| Isolated browser QA    | `hyprd/browser-isolation.ts`       | `hyprd-browser-isolation`       | server  |
| Tool guard             | `opencode/tool-guard.ts`           | `opencode-tool-guard`           | server  |
| Skill compact          | `opencode/skill-compact.ts`        | `opencode-skill-compact`        | server  |
| Primary compact        | `opencode/compact.ts`              | `opencode-compact`              | server  |
| Drive mode             | `opencode/drive.ts`                | `opencode-drive`                | server  |
| Media context prompt   | `opencode/media-context/prompt.ts` | `opencode-media-context-prompt` | server  |
| Input cap              | `opencode/input-cap.ts`            | `opencode-input-cap`            | server  |
| Code blocks            | `opencode/code-blocks.ts`          | `opencode-code-blocks`          | TUI     |
| Kitty context          | `hyprd/kitty.ts`                   | `hyprd-kitty-context`           | TUI     |
| Browser QA workspaces  | `hyprd/browser-qa.tsx`             | `hyprd-browser-qa`              | TUI     |
| Usage sidebar          | `usage/index.tsx`                  | `cullyn.usage-sidebar`          | TUI     |
| Lanes sidebar          | `delegate/sidebar.tsx`             | `delegate-lanes`                | TUI     |
| Sidebar scrollbar      | `opencode/sidebar-scrollbar.tsx`   | `opencode-sidebar-scrollbar`    | TUI     |
| Modified files         | `opencode/modified-files.tsx`      | `opencode-modified-files`       | TUI     |
| Markdown context       | `opencode/markdown-context.tsx`    | `opencode-markdown-context`     | TUI     |
| Media context sidebar  | `opencode/media-context/index.tsx` | `opencode-media-context`        | TUI     |
| MCP sidebar            | `opencode/mcp.tsx`                 | `opencode-mcp`                  | TUI     |
| Statusline             | `opencode/statusline.tsx`          | `opencode-statusline`           | TUI     |
| Pin model              | `opencode/pin-model.tsx`           | `opencode-pin-model`            | TUI     |

## Delegate

`delegate/index.ts` replaces the built-in `task` tool.
Calls without `lane` create one-shot children; a named lane resumes its direct child under the same parent session.
Each lane pins its agent but accepts a new `model` and `effort` on later calls.
Lane names live in child session metadata and appear in the root session's sidebar with status and context usage.
Context-limited lanes roll over to a fresh child on the next call with the same name.
`compact: true` requires an existing idle lane; it calls `session.summarize` with `auto: false` before sending that call's prompt.
Automatic compaction alone marks a child context-limited; manual compaction does not seal the lane.
Busy lanes reject new calls rather than queueing them.
`task_status` lists direct children and their lane names to recover from interrupted calls.
`task_close` closes idle named lanes; the next call with a closed name creates a fresh child.

Normal flow:

- `model` is `provider/model-id`; when omitted the child inherits the agent's pinned model or the current assistant message's model and effort.
- `effort` maps to the target model's reasoning variants.
- `unattended` defaults to true for children and rewrites permission asks to denies outside drive mode; descendants cannot become attended under an unattended parent.
- A lane resumes only when its permission envelope and execution mode still match the current policy.
- The provider must be listed in `config/opencode/delegate.json`.
- Before spawning, it waits abortably if any non-post-reset window is at >=100%, until the latest capped reset passes; stale, errored, or unknown usage proceeds un-gated.
- Children inherit external-directory rules; review leaves get their own read-only defaults.
- Content-filter-shaped errors return a normal result with `state="error"` instead of throwing.

Collab is the only attended primary and cannot be a child.
Scheme, Review, and Drive are skills loaded by the current owner, not agent names.

Only an attended primary Collab may launch or resume `build/git`, with explicit `unattended: true`.
The delegate checks the current caller, stored parent agent, and primary-session ancestry before asking permission.
Collab presents the repository/worktree, branch and refs, mutations, destructive effects, checks, and stop conditions before invocation; the full brief is included in permission metadata.
Its exact task permission is `ask`, and the request offers no reusable grant (`always: []`); existing remembered approvals can still satisfy the normal runtime gate.
The runtime does not parse or validate the human plan's intent.
The worker's named command permissions support the approved workflow without routine asks; inherited denials remain blockers, and permission patterns are guardrails rather than a shell sandbox.

Execution mode is stored in `metadata.delegate.unattended`.
The tool guard rejects file-write tools and recognized shell mutations for review, scout, source-verification, and web-verification agents.
These command checks are guardrails, not a shell sandbox.

Context governor:

- While a child is active, the delegate polls status and messages every 300 ms.
- Token pressure comes from completed assistant-step telemetry and mirrors OpenCode's overflow count: `tokens.total`, or input + output + cache read + cache write when total is absent.
- `shared/session.ts` owns `CONTEXT_PRESSURE`; its hard stop references `COMPACTION_LIMIT` directly, with no separate limits in `delegate.json`.
- At 100k (soft), the delegate appends one warning to try to finish before 150k while preserving assigned acceptance checks.
- At 150k (medium), it warns about possible degraded long-context performance and asks the child to finish soon and verify critical conclusions.
- At 200k (final), it reports the remaining context budget before the hard stop and asks for a final report, allowing only last edits already in progress or final evidence calls.
- Warnings do not abort, seal, change tools, or limit later resume; a normal child completion after any warning remains a trusted normal result.
- At 225k (hard), the delegate aborts active work, seals the session, and returns `state="context_limited"` with recoverable assistant text and durable-state advice.
- Hard-stopped and automatically compacted sessions persist `metadata.delegate.context`, receive a tail deny, appear marked in `task_status`, and trigger lane rollover.
- The pinned runtime's `prompt_async` is the supported non-aborting path: it accepts an asynchronous warning user turn while the existing runner remains active.
- A later message poll confirms that the warning was stored, but API acceptance alone cannot prove that the child consumed it.
- The delegate never retries an accepted warning request because a delayed first request could otherwise create a duplicate prompt loop.
- If the warning appears, the runner can consume it only after the model response or tool call already in progress, so the child can cross another threshold first.
- If one completed step jumps across multiple warning levels, the delegate sends only the highest reached warning and treats lower warnings as spent to avoid prompt loops.
- A warning API failure is reported as undelivered, while a stopped child whose accepted warning never appears is reported as unconfirmed.
- Telemetry is committed only at a completed model step, so the governor cannot stop an active response at an exact token or prevent the next step from starting before the poll.
- `experimental.session.compacting` can change only the compaction prompt and context; it cannot cancel compaction selectively.
- Global auto-compaction stays enabled; if an automatic child compaction part appears, the delegate aborts at the next poll and returns `context_limit: compaction` because compaction may already have started.

Unattended envelope, applied when `unattended: true` is selected and carried to descendants:

- The child envelope is composed as review defaults, the agent's whole effective ruleset, delegate denies, external-directory boundaries, and inherited unattended blockers.
- Every `ask` in that composition is rewritten to `deny` in place, so global config, parent, built-in defaults, and the selected agent's own profile are all covered by construction.
- In a drive-armed tree, the delegate skips that rewrite, and `opencode/drive.ts` approves each ask once.
- A lane stores its unrewritten envelope in `metadata.delegate.basis` and resumes when that basis still matches, so a drive toggle does not block a resume.
- Before the prompt, the delegate appends the current mode's envelope when the stored one differs, because a session update appends rules and the leading floor shadows the older ones.
- A lane without a stored basis resumes when its permissions end with either current envelope, and the delegate then stores the basis.
- The match ignores order inside a run of rules that share a permission and action and have no `*` pattern, because OpenCode lists skill directories in a different order after each restart.
- Rewriting keeps each rule's position, so a later, more specific `allow` still wins; `pacman -Q*` stays allowed even though `pacman *` asks.
- Rewriting happens before dedupe, otherwise a rewritten inherited rule survives as a tail duplicate and outranks the child's own refinement of the same permission.
- A leading `*` deny is prepended after dedupe as the floor for permissions no rule matches, since the runtime's own fallback is `ask`.
- Inheritance skips only the synthetic leading floor; appending that floor to a child's tail would outrank every allow the child needs.
- Unattended children inherit parent denials as blockers; attended children otherwise use their own effective profile.
- Children always deny `question`; an unattended child also has no permission prompts, and denied operations return as tool errors.
- The envelope applies on each child creation, with matching execution mode required on lane resume.

Practical failure diagnosis:

- `delegate provider policy missing for <provider>` → add the provider to `delegate.json`.
- `Unknown effort` → pick a variant that the target model exposes in config.
- `delegate resumed child permission envelope no longer matches` → re-brief a fresh child under the current policy.
- `delegate resumed child execution contract no longer matches` → preserve its unattended mode or use a new lane.
- `delegate lane ... is busy` → wait for the current call to finish before reusing its name.
- `delegate cannot compact lane ...` → first create an idle lane before requesting compaction.
- `context_limit: hard|compaction` → treat the recovered findings as partial, reconcile durable state when write-capable, and start a fresh narrower child.
- `child showed no activity within 120 seconds` → the model/provider failed to start producing output.
- `blocked: content_filter` → reword the brief first; switch provider only as a last resort; never resume the tainted child.

## Usage

Usage has a TUI view and a read-only server tool.
`usage/index.tsx` shows OpenAI, Anthropic, xAI, Cursor, and OpenCode headroom in the sidebar.
`usage/tool.ts` exposes `usage_status`, a primary tool that reads the same local cache without refreshing providers.

Normal flow:

- Cache files live under `${XDG_CACHE_HOME}/opencode/usage-sidebar/`, or `~/.cache/opencode/usage-sidebar/` without `XDG_CACHE_HOME`.
- Locks live under `${XDG_RUNTIME_DIR}/opencode/`, or `/tmp/opencode-${uid}/` without `XDG_RUNTIME_DIR`.
- Adapters enforce per-provider minimum fetch intervals and backoffs for errors or 429s.
- The sidebar keeps stale windows visible with a muted note instead of replacing them with an error.
- `usage_status` asks the `usage_status` permission and reports remaining percent, reset timing, and cache age; permission config decides who may call it.

Auth sources:

- OpenAI: OpenCode `auth.json` OAuth entry.
- Anthropic: fixed Claude credential directories, one for Trend and one for Cogikyo.
- xAI: Grok CLI auth at `~/.grok/auth.json`; refresh via `grok models`.
- Cursor: OpenCode `auth.json` OAuth entry.
- OpenCode Go: OpenCode `auth.json` API key under `opencode-go`.

Claude subscription requests are handled by `opencode-claude-auth` directly to Anthropic.
The usage adapter's `claude -p . --model haiku` invocation is only bounded 401 recovery; it does not route subscription requests.

Practical failure diagnosis:

- `no auth` → missing provider credentials; the note is warning-colored for OpenCode Go and error-colored for OpenAI, Anthropic, and Cursor.
- xAI auth failures use Grok CLI credentials and show warning-colored notes.
- `invalid key` → the OpenCode Go API key was rejected.
- `429` → rate-limited; wait for the backoff or the reset window.
- `stale` note → cached data is older than the provider's `staleAfterMS`; click the provider row for a manual refresh.
- `auth recovery failed` (amber) → recovery failed in the selected account's Claude directory; run `claude-auth trend` or `claude-auth cogikyo`.
- `usage_status` unavailable → delegate child permission derivation denies `experimental.primary_tools` tools unless the child agent's frontmatter explicitly allows them.

## Claude auth

[`anthropic/index.ts`](anthropic/README.md) loads two account-bound instances of a vendored, patched `opencode-claude-auth@2.2.1`.
It talks directly to Anthropic and retains upstream request formatting, with separate auth state for `anthropic` (Trend) and `anthropic-personal` (Cogikyo).
Use the linked guide for login, patch updates, verification, and rollback.

## Notifications and Kitty context

`hyprd/kitty.ts` writes which Kitty pane owns the active OpenCode session.
`hyprd/notify.ts` reads that context and sends notifications to `/tmp/hyprd.sock`.

Normal flow:

- The context file is `${XDG_RUNTIME_DIR}/opencode/kitty-context.json`, falling back to `/tmp/opencode-${uid}/kitty-context.json`.
- The directory is mode `0700` and the file is mode `0600`.
- Each session record includes its project directory and TUI generation with the existing Kitty pane identity.
- The writer prunes stale entries and dead Kitty sockets.
- The reader follows parent session IDs so subagent notifications target the pane that owns the parent session.
- Idle reminders only fire when the context is fresh.

Event types sent: `start`, `complete`, `subagent`, `idle`, `permission`, `question`, `todo-complete`, `error`.

Practical failure diagnosis:

- No notifications → confirm `hyprd` is running and `/tmp/hyprd.sock` exists.
- Notifications go to the wrong pane → check `KITTY_PID` and `KITTY_WINDOW_ID` in the TUI pane; the writer skips context without them.
- Stale context → the writer removes entries older than `STALE_CONTEXT_MS` or whose Kitty socket is gone.
- Duplicate permission/question toasts → the notify path dedupes within ~1s windows.

## Browser QA

`hyprd/browser-isolation.ts` exposes Chrome DevTools tools from one MCP subprocess per OpenCode session.
Each subprocess launches an isolated Chromium profile, so concurrent browser agents cannot list or change each other's pages.
Marked Chromium windows are assigned stable `browser-qa-<slot>` workspaces by hyprd and shown in the `Browsers` sidebar section.
When an agent session becomes idle or is deleted, the plugin closes its MCP subprocess and isolated browser.

## TUI presentation

`usage/index.tsx` owns the `sidebar_title` and `sidebar_content` slots; it deactivates `internal:sidebar-context` on load and restores it on dispose.
The other sidebar sections register `sidebar_content` with distinct orders.
`delegate/sidebar.tsx` lists active lanes in the root session sidebar.

- `opencode/code-blocks.ts` patches OpenTUI code-block rendering and registers a SQL tree-sitter parser.
- `hyprd/browser-qa.tsx` keeps one workspace subscription per plugin instance and lists marked browser workspaces before MCP.
- `opencode/input-cap.ts` caps eligible enabled-provider model input limits at `COMPACTION_LIMIT + reserved` (225k + 25k by default), using cached catalog limits when needed.
  Models that already compact at or below 225k stay unchanged.
  Load it after provider plugins that seed models, including Cursor.
- `opencode/statusline.tsx` wraps `session_prompt` with cwd, git status, and a context-pressure bar that uses OpenCode's overflow token count and effective compaction threshold.
  Pink is the last pressure tier before that threshold.
- `opencode/modified-files.tsx` lists files touched in the current session.
- `opencode/markdown-context.tsx` lists Markdown reads plus pinned `AGENTS.md` files, the current agent, skills, and slash commands. Click the close mark to stub an unpinned skill or Markdown read. Click restore on a compacted row to reload the file from disk. Click the label to open the file.
- `opencode/skill-compact.ts` stubs loaded skill bodies when a session compacts. It also uncompacts protected `AGENTS.md` / Collab reads so native prune cannot keep them stubbed.
- `opencode/compact.ts` adds the primary-only `compact` tool and appends a system nudge to primary sessions once context passes 120k and again at 200k.
  An approved call runs `session.summarize` with `auto: false` when the turn goes idle, passes the agent's brief into the compaction context, and leaves the session waiting for the user.
  A denial silences nudges until the next tier; any compaction resets the tiers. Calls are logged to `${XDG_STATE_HOME:-~/.local/state}/opencode/compact.jsonl`.
- `opencode/drive.ts` arms drive mode when the user runs `/drive` or `/drive <task>` in a top-level session, and `/drive off` disarms it; the state is in memory and clears on restart.
  The `commands/drive.md` command owns the name; the plugin replaces its text with a one-line note plus the task, and the model still takes a turn.
  In an armed tree, it approves each permission ask once, returns `question` calls to the agent, and prompts the armed session to continue after a compaction that did not continue by itself.
  It retries an approval after a network error, 5xx, or 429, and stops after any other 4xx; a non-404 stop shows an error toast because the ask then waits for the user.
- `opencode/media-context/index.tsx` lists registered images and videos and opens images in a Kitty overlay.
- `opencode/pin-model.tsx` pins the current model to `opencode.json` with `<leader>f` / `/pin`, and switches to that pin with `<leader>shift+t` / `/pinned`, and toggles reasoning between `medium` and `high` with `<leader>i`.
  The variant is stored at `join(api.state.path.state, "pin.json")`; new OpenCode windows read the pinned `model` from `opencode.json`.
- `opencode/sidebar-scrollbar.tsx` hides the sidebar scrollbar.
- `opencode/mcp.tsx` lists MCP status when a server is enabled.

Practical failure diagnosis:

- Usage sidebar missing → verify `tui.json` includes `usage/index.tsx` and `plugin_enabled.internal:sidebar-files` is `false`.
- Browser QA missing → verify `hyprd` is running and `tui.json` includes `hyprd/browser-qa.tsx`.
- Code blocks not styled → the plugin warns via toast when OpenTUI internals change.
- Media preview fails → needs `python3`, `config/xplr/bin/kitty-preview.py`, and a Kitty terminal.
- Statusline shows no context pressure → the model's context limit is not exposed or there is no assistant output yet.
- Pin model missing → restart OpenCode after `tui.json` includes `opencode/pin-model.tsx`.
- `Model picker unavailable` → OpenCode's Local context shape changed and the Solid owner walk failed.

## For agents changing plugins

### Source-of-truth boundaries

- `config/opencode/opencode.json` and `config/opencode/tui.json` are the only load surfaces.
- `hyprd/context.ts` owns the Kitty context path, schema, stale window, and Kitty socket probe.
- `usage/providers.ts` owns provider IDs, labels, and `staleAfterMS`.
- `usage/cache.ts` owns the cache file shape, cache and lock paths, lock semantics, and decoder.
- `usage/auth.ts` owns OpenCode and Claude credential paths and schemas.
- `opencode/media-context/store.ts` owns `registryPath`, registry entry validation, and disk operations; `registry.ts` owns media references and file-part IDs.
- `shared/opencode.ts` owns server-only typed readers over the v1 client, with Zod views checked against v2 SDK types.
- `shared/file.ts` owns `readJson` and atomic `writeText` / `writeJson`; `shared/error.ts` formats thrown values.
- `delegate/metadata.ts` owns the `metadata.delegate` schema.
- `shared/drive.ts` owns the in-memory armed sessions and the parent walk that `opencode/drive.ts` and the delegate read.
- `opencode/skill-parts.ts` owns skill/read tool-part compacting and persist via TUI `part.update` or server HTTP PATCH.
- `delegate/config.ts` hardcodes `DELEGATE_CONFIG_PATH` to `/home/cullyn/dotfiles/config/opencode/delegate.json`.
- Changing `hyprd/context.ts` paths or schema requires updating both `hyprd/kitty.ts` and `hyprd/notify.ts`.
- `shared/session.ts` owns `COMPACTION_LIMIT` (225k) and `COMPACTION_RESERVED` (25k); input-cap uses their sum as its default cap (250k).
- `shared/` owns session/provider metadata, colors/icons, git status parsing, and the sidebar-section wrapper; only put helpers there when more than one plugin owns the concept.

### Invariants

- Plugin-created persisted parts must have IDs starting with `prt`.
- Never derive plugin-created part IDs from `output.message.id`.
- Video files stay local-only; only images are pushed as provider file parts.
- Usage adapters must not log tokens, cookies, or local paths.
- `usage_status` is read-only and must never refresh providers or mutate chat context.
- Delegate children deny `todowrite`, `task`, and `experimental.primary_tools` tools unless the agent declares them.
- Delegate children always deny `question`, and unattended children outside drive mode carry no `ask` rule at all.
- Kitty context directory is mode `0700` and the context JSON file is mode `0600`.
- Media registry directories are mode `0700` and registry files are mode `0600`.
- Named media images are copied into the runtime cache; original source files are never renamed.

### Safe change checklist

1. Edit the source file.
2. If adding or removing a plugin, update the matching JSON load surface.
3. Keep plugin IDs stable; OpenCode may persist plugin state by ID.
4. For media-context, keep the server hook and TUI plugin IDs stable and in the correct load surfaces.
5. Run the verification commands below.
6. Restart OpenCode.

### Verification

From the repo root:

```bash
bunx --package typescript tsc --noEmit --project config/opencode/tsconfig.json
git diff --check -- config/opencode/plugins
```

From `config/opencode/`:

```bash
bunx --package typescript tsc --noEmit --project tsconfig.json
git diff --check -- plugins
```

If local dependencies help your editor/LSP, install them with `bun install --cwd config/opencode --ignore-scripts`.
