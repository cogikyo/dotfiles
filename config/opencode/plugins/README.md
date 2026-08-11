# OpenCode Plugins

Plugins in `config/opencode/plugins/` are loaded directly from source by OpenCode and Bun.
`config/opencode/tsconfig.json` is only for typechecking; there is no build artifact.

OpenCode has two plugin runtimes:

- **Server plugins** live where tools and hooks run; register them in `config/opencode/opencode.json`.
- **TUI plugins** render inside the terminal UI; register them in `config/opencode/tui.json`.

Changes to plugin entries or source only take effect after an OpenCode restart.
Running sessions keep the loaded plugin set.

## Plugin map

| Feature | Entrypoint | ID | Runtime |
|---|---|---|---|
| Claude auth | `opencode-claude-auth@1.5.4` | (package) | server |
| Delegate task | `delegate/index.ts` | `delegate-task` | server |
| Git batch | `git/tool.ts` | `git-batch` | server |
| Usage status tool | `usage/tool.ts` | `usage-status` | server |
| Hyprland notifications | `hyprd/notify.ts` | `hyprd-notify` | server |
| Spec title | `opencode/spec-title.ts` | `opencode-spec-title` | server |
| Media context prompt | `opencode/media-context/prompt.ts` | `opencode-media-context-prompt` | server |
| Code blocks | `opencode/code-blocks.ts` | `opencode-code-blocks` | TUI |
| Kitty context | `hyprd/kitty.ts` | `hyprd-kitty-context` | TUI |
| Usage sidebar | `usage/index.tsx` | `cullyn.usage-sidebar` | TUI |
| Modified files | `opencode/modified-files.tsx` | `opencode-modified-files` | TUI |
| Markdown context | `opencode/markdown-context.tsx` | `opencode-markdown-context` | TUI |
| Media context sidebar | `opencode/media-context/index.tsx` | `opencode-media-context` | TUI |
| Statusline | `opencode/statusline.tsx` | `opencode-statusline` | TUI |

## Delegate

`delegate/index.ts` replaces the built-in `task` tool.
It spawns a child session for each call, optionally with a per-call `model` and `effort`.

Normal flow:

- `model` is `provider/model-id`; when omitted the child inherits the agent's pinned model or the current assistant message's model and effort.
- `effort` maps to the target model's reasoning variants.
- `task_id` resumes a direct idle child only when its agent and freshly derived permission envelope still match and it has no context-limit marker.
- `task_status` lists direct children with task IDs, agents, titles, live statuses, and persisted context-limit markers.
- Resume sparingly for the same unfinished work; never resume a context-limited child.
- The provider must be listed in `config/opencode/delegate.json`.
- `delegate.json.context` requires positive integer thresholds with `soft < medium < hard`; the explicit defaults are 120,000, 150,000, and 200,000 tokens.
- Before spawning, it waits abortably if any non-post-reset window is at >=100%, until the latest capped reset passes; stale, errored, or unknown usage proceeds un-gated.
- Children inherit parent denies and `external_directory` rules; review agents get a read-only default profile.
- Content-filter-shaped errors return a normal result with `state="error"` instead of throwing.

Context governor:

- While a child is active, the delegate polls status and messages every 300 ms.
- Token pressure comes from completed assistant-step telemetry and mirrors OpenCode's overflow count: `tokens.total`, or input + output + cache read + cache write when total is absent.
- At the soft limit, the delegate appends one warning to converge and finish soon without aborting, sealing, changing tools, or limiting later resume.
- At the medium limit, the delegate appends one warning to finish immediately, allowing only last edits already in progress or final evidence calls.
- A normal child completion after either warning remains a trusted normal result.
- At the hard limit, the delegate aborts active work, seals the session, and returns `state="context_limited"` with recoverable assistant text and durable-state advice.
- Hard-stopped and compacted sessions persist `metadata.delegate.context`, receive a tail deny, appear marked in `task_status`, and are rejected by `task_id`.
- The pinned runtime's `prompt_async` is the supported non-aborting path: it accepts an asynchronous warning user turn while the existing runner remains active.
- A later message poll confirms that the warning was stored, but API acceptance alone cannot prove that the child consumed it.
- The delegate never retries an accepted warning request because a delayed first request could otherwise create a duplicate prompt loop.
- If the warning appears, the runner can consume it only after the model response or tool call already in progress, so the child can cross another threshold first.
- If one completed step jumps across both warning levels, the delegate sends only the medium warning and treats the superseded soft warning as spent to avoid prompt loops.
- A warning API failure is reported as undelivered, while a stopped child whose accepted warning never appears is reported as unconfirmed.
- Telemetry is committed only at a completed model step, so the governor cannot stop an active response at an exact token or prevent the next step from starting before the poll.
- `experimental.session.compacting` can change only the compaction prompt and context; it cannot cancel compaction selectively.
- Global auto-compaction stays enabled; if an automatic child compaction part appears, the delegate aborts at the next poll and returns `context_limit: compaction` because compaction may already have started.

Unattended envelope, applied when Drive appears anywhere in the parent's session ancestry:

- The child envelope is composed as review defaults, the agent's whole effective ruleset, the delegate denies, then inherited parent rules.
- Every `ask` in that composition is rewritten to `deny` in place, so global config, parent, built-in defaults, and the selected agent's own profile are all covered by construction.
- Rewriting keeps each rule's position, so a later, more specific `allow` still wins; `pacman -Q*` stays allowed even though `pacman *` asks.
- Rewriting happens before dedupe, otherwise a rewritten inherited rule survives as a tail duplicate and outranks the child's own refinement of the same permission.
- A leading `*` deny is prepended after dedupe as the floor for permissions no rule matches, since the runtime's own fallback is `ask`.
- Inheritance skips only that synthetic leading floor, and only when the parent is itself under Drive lineage; a floor is positional, so appending one to a child's tail would outrank every allow the child needs.
- Every other parent rule is inherited unchanged, including a catch-all the parent's own profile declares later, and a non-Drive parent is inherited exactly as before.
- Net effect: a child anywhere under Drive can never surface a `question` or a permission prompt, and a denied operation returns to the child as an ordinary tool error for the parent to judge.
- The same derivation runs for a mode child, so `Drive → Collab → leaf` carries the policy to every depth.

Practical failure diagnosis:

- `delegate provider policy missing for <provider>` → add the provider to `delegate.json`.
- `Unknown effort` → pick a variant that the target model exposes in config.
- `delegate resumed child permission envelope no longer matches` → re-brief a fresh child under the current policy.
- `delegate refuses context-limited child session` → start a fresh narrower child; never reuse that task ID.
- `context_limit: hard|compaction` → treat the recovered findings as partial, reconcile durable state when write-capable, and start a fresh narrower child.
- `child showed no activity within 120 seconds` → the model/provider failed to start producing output.
- `blocked: content_filter` → reword the brief first; switch provider only as a last resort; never resume the tainted child.

## Git batch

`git/tool.ts` exposes `git_batch` for bounded read-only Git inspection without shell composition.
It accepts an ordered list of structured `merge-base`, `log`, and `diff` operations and runs `/usr/bin/git` directly in the session worktree.

- The argv grammar allows only the comparison forms documented by the tool; callers cannot supply a cwd, global Git options, pathspecs, config, or executable paths.
- Git runs sequentially with clean defensive environment variables, disabled pagers, hooks, fsmonitor, external diff, textconv, replacement objects, optional locks, and lazy fetches.
- Each result preserves stdout, stderr, and exit code; ordinary diagnostic exits continue the batch, while abort, timeout, spawn failure, or output overflow stops it explicitly.
- The tool allows at most eight operations, eight arguments per operation, 15 seconds per operation, 60 seconds per batch, 512 KiB per operation, and 2 MiB per batch.
- Global permission denies `git_batch`; only Review and the four Git agents allow it.

## Usage

Usage has a TUI view and a read-only server tool.
`usage/index.tsx` shows OpenAI, Claude, xAI, and OpenCode headroom in the sidebar.
`usage/tool.ts` exposes `usage_status`, a primary tool that reads the same local cache without refreshing providers.

Normal flow:

- Cache files live under `${XDG_CACHE_HOME}/opencode/usage-sidebar/`, or `~/.cache/opencode/usage-sidebar/` without `XDG_CACHE_HOME`.
- Locks live under `${XDG_RUNTIME_DIR}/opencode/`, or `/tmp/opencode-${uid}/` without `XDG_RUNTIME_DIR`.
- Adapters enforce per-provider minimum fetch intervals and backoffs for errors or 429s.
- The sidebar keeps stale windows visible with a muted note instead of replacing them with an error.
- `usage_status` asks the `usage_status` permission and reports remaining percent, reset timing, and cache age; permission config decides who may call it.

Auth sources:

- OpenAI: OpenCode `auth.json` OAuth entry.
- Claude: OpenCode `auth.json` OAuth entry.
- xAI: Grok CLI auth at `~/.grok/auth.json`; refresh via `grok models`.
- OpenCode: Firefox `auth` cookie for `opencode.ai` from `cookies.sqlite`.

Claude subscription requests are handled by `opencode-claude-auth` directly to Anthropic.
The usage adapter's `claude -p . --model haiku` invocation is only bounded 401 recovery; it does not route subscription requests.

Practical failure diagnosis:

- `no auth` (red) → missing or non-OAuth provider credentials; xAI uses Grok CLI auth, not OpenCode's xai OAuth.
- `sign in` (amber) → OpenCode Firefox session expired.
- `429` → rate-limited; wait for the backoff or the reset window.
- `stale` note → cached data is older than the provider's `staleAfterMS`; click the provider row for a manual refresh.
- `auth recovery failed` (amber) → recovery checks `$CLAUDE_CONFIG_DIR` when set, then the XDG Claude config and legacy `~/.claude`; otherwise the 401 is unrecoverable from here.
- `usage_status` unavailable → delegate child permission derivation denies `experimental.primary_tools` tools unless the child agent's frontmatter explicitly allows them.

## Claude auth

`opencode-claude-auth` is the server plugin that owns Claude subscription requests.

- It talks directly to Anthropic, owns credential refresh, and wraps subscription requests so nothing else touches Anthropic's billing shape.
- The usage adapter's `claude -p . --model haiku` fallback is bounded 401 recovery only: it reads `$CLAUDE_CONFIG_DIR` when set, otherwise `${XDG_CONFIG_HOME:-~/.config}/claude` and legacy `~/.claude`, then retries the usage fetch once after a successful refresh.

## Notifications and Kitty context

`hyprd/kitty.ts` writes which Kitty pane owns the active OpenCode session.
`hyprd/notify.ts` reads that context and sends notifications to `/tmp/hyprd.sock`.

Normal flow:

- The context file is `${XDG_RUNTIME_DIR}/opencode/kitty-context.json`, falling back to `/tmp/opencode-${uid}/kitty-context.json`.
- The directory is mode `0700` and the file is mode `0600`.
- The writer prunes stale entries and dead Kitty sockets.
- The reader follows parent session IDs so subagent notifications target the pane that owns the parent session.
- Idle reminders only fire when the context is fresh.

Event types sent: `start`, `complete`, `subagent`, `idle`, `permission`, `question`, `todo-complete`, `error`.

Practical failure diagnosis:

- No notifications → confirm `hyprd` is running and `/tmp/hyprd.sock` exists.
- Notifications go to the wrong pane → check `KITTY_PID` and `KITTY_WINDOW_ID` in the TUI pane; the writer skips context without them.
- Stale context → the writer removes entries older than `STALE_CONTEXT_MS` or whose Kitty socket is gone.
- Duplicate permission/question toasts → the notify path dedupes within ~1s windows.

## TUI presentation

`usage/index.tsx` owns the `sidebar_title` and `sidebar_content` slots; it deactivates `internal:sidebar-context` on load and restores it on dispose.
The other sidebar sections register `sidebar_content` with distinct orders.

- `opencode/code-blocks.ts` patches OpenTUI code-block rendering and registers a SQL tree-sitter parser.
- `opencode/statusline.tsx` wraps `session_prompt` with cwd, git status, and a context-pressure bar.
- `opencode/modified-files.tsx` lists files touched in the current session.
- `opencode/markdown-context.tsx` lists Markdown files backed by completed `read` tool calls.
- `opencode/media-context/index.tsx` lists registered images and videos and opens images in a Kitty overlay.

Practical failure diagnosis:

- Usage sidebar missing → verify `tui.json` includes `usage/index.tsx` and `plugin_enabled.internal:sidebar-files` is `false`.
- Code blocks not styled → the plugin warns via toast when OpenTUI internals change.
- Media preview fails → needs `python3`, `config/xplr/bin/kitty-preview.py`, and a Kitty terminal.
- Statusline shows no context pressure → the model's context limit is not exposed or there is no assistant output yet.

## For agents changing plugins

### Source-of-truth boundaries

- `config/opencode/opencode.json` and `config/opencode/tui.json` are the only load surfaces.
- `hyprd/context.ts` owns the Kitty context path, schema, stale window, and Kitty socket probe.
- `usage/providers.ts` owns provider IDs, labels, and `staleAfterMS`.
- `usage/cache.ts` owns the cache file shape, lock semantics, and decoder.
- `usage/auth.ts` owns path resolution for auth, cache, and runtime directories.
- `opencode/media-context/registry.ts` owns media registry paths, handle/alias patterns, and file-part ID rules.
- `git/tool.ts` owns the `git_batch` argv grammar, worktree binding, process hardening, and resource bounds.
- `delegate/config.ts` hardcodes `DELEGATE_CONFIG_PATH` to `/home/cullyn/dotfiles/config/opencode/delegate.json`.
- Changing `hyprd/context.ts` paths or schema requires updating both `hyprd/kitty.ts` and `hyprd/notify.ts`.
- `shared/` owns session/provider metadata, colors/icons, git status parsing, and the sidebar-section wrapper; only put helpers there when more than one plugin owns the concept.

### Invariants

- Plugin-created persisted parts must have IDs starting with `prt`.
- Never derive plugin-created part IDs from `output.message.id`.
- Video files stay local-only; only images are pushed as provider file parts.
- Usage adapters must not log tokens, cookies, or local paths.
- `usage_status` is read-only and must never refresh providers or mutate chat context.
- `git_batch` remains read-only, shell-free, worktree-bound, and limited to its exact operation grammar.
- Delegate children deny `todowrite`, `task`, and `experimental.primary_tools` tools unless the agent declares them.
- Delegate children always deny `question`, and children under Drive lineage additionally carry no `ask` rule at all.
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
