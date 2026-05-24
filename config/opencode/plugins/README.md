# OpenCode Plugins

These plugins are loaded directly from source by OpenCode and Bun.
`tsconfig.json` is for typechecking only; there is no build artifact checked in or loaded by config.

## Layout

`hyprd/notify.ts` is the server-side notification bridge loaded by `opencode.json`.
It listens to OpenCode events and sends notification commands to `hyprd`.
Its plugin ID is `hyprd-notify`.

`hyprd/kitty.ts` is the TUI-side pane context writer loaded by `tui.json`.
It records which Kitty pane owns the active OpenCode session.
Its plugin ID is `hyprd-kitty-context`.

`hyprd/context.ts` is the local contract between the Hyprd plugins.
Keep the context file path, schema, stale window, and Kitty socket probe here instead of duplicating them.

`openai/usage.tsx` is a TUI sidebar replacement loaded by `tui.json`.
It shows ChatGPT usage-limit windows only for OpenAI sessions.
Its plugin ID is `cullyn.openai-quota-sidebar`.

`opencode/markdown-context.tsx` is a TUI sidebar content section loaded by `tui.json`.
It lists Markdown files backed by completed `read` tool calls.
Its plugin ID is `opencode-markdown-context`.

`opencode/media-context/index.tsx` is a TUI sidebar content section loaded by `tui.json`.
It lists registered media references for the current session, previews local images with Kitty, and opens videos with `xdg-open`.
Its plugin ID is `opencode-media-context`.

`opencode/statusline.tsx` is a TUI prompt wrapper loaded by `tui.json`.
It injects statusline chrome into `session_prompt` while forwarding the real prompt props/ref unchanged.
Its plugin ID is `opencode-statusline`.

`shared/` contains cross-plugin helpers.
Keep helpers here only when more than one plugin owns the concept.

## Load Surfaces

Server plugins go in `opencode.json`.
TUI plugins go in `tui.json`.

Plugin IDs should stay stable across file moves because OpenCode may persist plugin state by ID.
The current IDs intentionally preserve the pre-reorg IDs where they existed.

## Hyprd Context Contract

`hyprd/kitty.ts` owns writes to the Kitty context file.
`hyprd/notify.ts` owns reads from it.

The context path is `${XDG_RUNTIME_DIR}/opencode/kitty-context.json` when `XDG_RUNTIME_DIR` exists.
The fallback is `/tmp/opencode-${uid}/kitty-context.json`.
The parent directory is created with mode `0700`; the JSON file is written with mode `0600`.

The JSON shape is `Record<sessionID, KittyContext>`.
`KittyContext` is `{ kitty_pid: number, kitty_window_id: number, updated_at: number }`.

Contexts older than `STALE_CONTEXT_MS` are pruned by the writer.
A context is also invalid when `/tmp/kitty-${kitty_pid}` is not a socket.

`notify.ts` follows parent session IDs through this map so subagent notifications target the pane that owns the parent OpenCode session.
Idle reminders require a fresh context because stale pane targeting is noisier than missing a reminder.

`notify.ts` sends commands to `/tmp/hyprd.sock`.
That socket is expected to be owned by the current user session.

## OpenAI Usage Contract

`openai/usage.tsx` reads OpenCode auth from `${XDG_DATA_HOME:-~/.local/share}/opencode/auth.json`.
It expects an OpenAI OAuth entry with an access token.

The plugin calls `https://chatgpt.com/backend-api/wham/usage` with `Authorization: Bearer <token>`.
When possible it derives `ChatGPT-Account-Id` from `openai.accountId` or the OAuth JWT payload object `https://api.openai.com/auth` field `chatgpt_account_id`.

The response is expected to expose `rate_limit.primary_window` and optionally `rate_limit.secondary_window`.
Each window may contain `remaining_percent`, `used_percent`, `reset_at`, or `reset_after_seconds`.
The UI displays used percentage; if the API only returns remaining percentage, the plugin inverts it at the boundary.

This endpoint and shape are private ChatGPT implementation details.
Failures should degrade to a coarse `OpenAI usage unavailable` or `Usage windows unavailable` message rather than exposing local paths or token parsing details.

The plugin deactivates `internal:sidebar-context` while active because it owns the `sidebar_title` and `sidebar_content` slots.
On dispose it reactivates that internal plugin only if this plugin deactivated it.

## Media Context Contract

Media context has both a server hook and a TUI sidebar.
`opencode/media-context/prompt.ts` is loaded by `opencode.json`; `opencode/media-context/index.tsx` is loaded by `tui.json`.

The server hook registers attached media and detected local video paths, resolves image handles and aliases into provider-safe file parts, emits local-only notices for video handles, and preserves media references in compaction context.
It also enqueues newly attached images for async sidecar naming after the session becomes idle.
Any persisted part created by the plugin and pushed into `output.parts` must use an ID beginning with `prt`.
Never derive plugin-created part IDs from `output.message.id`; old `msg_*media-context-*` rows were history poison because OpenCode rejects persisted parts whose IDs do not start with `prt`.

`opencode/media-context/registry.ts` owns the runtime registry.
The registry lives at `${XDG_RUNTIME_DIR}/opencode/media-context/<session>.json` with a `/tmp/opencode-${uid}/media-context` fallback.
It is sidecar/runtime state, not an authority to recreate visible chat history.
Registry directories are mode `0700`; registry JSON files are mode `0600`.
Registry writes use a per-session lock and atomic replace.

Media files get stable per-session timestamp handles in the exact form `@HH_MM_SS`.
Newly attached images may additionally get generated aliases in the exact form `@lowercase-slug`.
If multiple media files register in the same second, later handles append `_2`, `_3`, and so on.
Generated aliases are capped at 3 words, sanitized to safe lowercase filename characters, and collision-suffixed.
Missing backing files are hidden from the sidebar and cannot resolve, but registry entries are retained so timestamp handles are not reused while the runtime registry exists.

`opencode/media-context/index.tsx` lists current-session media reconciled against currently loaded session messages.
Registry-only entries are hidden on restore when persisted messages are unavailable, empty, or stale.
The sidebar can discover media from loaded persisted file parts when the TUI exposes them.
On event-driven refresh, the TUI may backfill attached media and detected video paths from the most recent 100 current-session messages, capped at 20 registrations per refresh.
It does not scan old sessions or full history.
Image rows display `I <generated-name>` when a model alias exists and `I <timestamp-handle>` otherwise; video rows display `V <local-basename>` with handle fallback.
Clicking an image row opens a borderless full-screen OpenTUI backdrop while Kitty draws the image preview out-of-band; clicking a video row opens the file with `xdg-open`.

Local image previews are attempted for `file://` URLs and any existing `source.path`, including clipboard backing paths.
Unsupported `http(s):`, internal, relative, and malformed URLs are not registered because they cannot be dereferenced later as local files.
Video `source.path` and `file://` registrations use the same local-file guard as sidebar discovery.
Detected local video paths are registration-only unless they are later referenced by handle; even then, videos stay local-only and are not pushed as provider file parts.
Videos are never model-named.

The plugin reuses `config/xplr/bin/kitty-preview.py` via `python3` with argv-only spawning for image previews.
Kitty overlay coordinates use a centered terminal rectangle under the backdrop.
It clears the Kitty overlay on backdrop close, session navigation, session commands, slot cleanup, and plugin disposal.
Backing local files must remain available for handles to resolve after compaction.

Image naming is configured through the `opencode.json` server plugin tuple under `imageNames`.
The tuple can disable naming with `{ "imageNames": { "enabled": false } }`.
Naming reuses the current OpenCode prompt model and auth by creating a temporary session, prompting it with the image, then deleting that session best-effort.
If the prompt model is unavailable, the plugin falls back to the configured default OpenCode model when exposed by the config hook.
It does not read `OPENAI_API_KEY`, parse provider auth files, or call provider APIs directly.
Naming failures are swallowed and leave timestamp handles intact.
Named images are copied into the media-context runtime cache under the generated filename; original source files are not renamed.
OpenCode does not currently expose a true non-persistent completion API, so temp session/stat persistence depends on best-effort deletion and future API support.

## Statusline Contract

`opencode/statusline.tsx` registers `session_prompt`.
That slot wraps `api.ui.Prompt` rather than replacing input behavior.

The slot receives `session_id`, `visible`, `disabled`, `on_submit`, and `ref`.
The wrapper forwards those to `api.ui.Prompt` as `sessionID`, `visible`, `disabled`, `onSubmit`, and `ref`.
Only `hint` is customized.
`right` belongs to OpenCode's prompt metadata line, not the preview/status row.

The statusline intentionally omits provider, model, and effort because OpenCode already renders those in prompt metadata.
It owns cwd, git status, and a Claude-style context pressure bar on the preview/status row.
The pressure bar normalizes against a local `150K` auto-compaction window instead of the model's advertised max context.
OpenCode's native prompt keeps numeric context usage and command-list keybind hints on the right side of that row.

## Markdown Context Contract

`opencode/markdown-context.tsx` is a trust surface, not a recommender.
It must only show Markdown files that have hard evidence in the current session state.

Read entries require a completed `read` tool part whose input contains a `.md`, `.mdx`, or `.markdown` path.
Do not add inferred, discovered, grep-only, or path-proximity entries to this plugin.

The global `~/.config/opencode/AGENTS.md` file is intentionally omitted because it is always loaded for this setup.
The sidebar section is hidden until at least one Markdown read exists.

Compacted read entries are shown with a red `C` marker when OpenCode marks the completed tool part that way.
Fresh read entries use source markers: green `R` for `README.md`, blue `A` for `AGENTS.md`, orange `O` for core orchestration files as `Master`, `Manager`, or `Worker`, cyan `S` for `SKILL.md`, yellow `I` for uppercase pointer docs like `GO.md` or `DATABASE.md`, and muted `M` for generic Markdown.
`S` labels show the parent skill directory, such as `commit` or `scribe`.
Carrier files omit their filename in labels because the marker carries that information.

## Typechecking

Plugins are loaded directly from source by OpenCode and Bun, so the usual check is an ephemeral Bun run with no local install.

From the repo root:

```bash
bunx --package typescript tsc --noEmit --project config/opencode/tsconfig.json
git diff --check -- config/opencode/plugins
```

From `config/opencode`:

```bash
bunx --package typescript tsc --noEmit --project tsconfig.json
git diff --check -- plugins
```

If local dependencies are useful for editor/LSP state, install them with Bun and keep scripts disabled:

```bash
bun install --cwd config/opencode --ignore-scripts
```

`npx --yes --package typescript tsc --noEmit --project config/opencode/tsconfig.json` is a fallback from the repo root.
Use `--package typescript`; plain `npx tsc` can resolve the unrelated `tsc` package.

The Hyprd plugins currently keep `@ts-nocheck` because OpenCode plugin event payload types are incomplete locally.
Remove that suppression once local event and payload types exist.
