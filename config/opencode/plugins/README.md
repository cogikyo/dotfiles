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

`usage/index.tsx` is a TUI sidebar replacement loaded by `tui.json`.
It shows OpenAI, Claude, xAI, and opencode-go usage sections for every session.
Its plugin ID is `cullyn.usage-sidebar`.

`opencode/markdown-context.tsx` is a TUI sidebar content section loaded by `tui.json`.
It lists Markdown files backed by completed `read` tool calls.
Its plugin ID is `opencode-markdown-context`.

`opencode/media-context/index.tsx` is a TUI sidebar content section loaded by `tui.json`.
It lists registered media references for the current session, previews local images with Kitty, and opens videos with `xdg-open`.
Its plugin ID is `opencode-media-context`.

`opencode/statusline.tsx` is a TUI prompt wrapper loaded by `tui.json`.
It injects statusline chrome into `session_prompt` while forwarding the real prompt props/ref unchanged.
Its plugin ID is `opencode-statusline`.

`opencode/spec-title.ts` is the server-side `spec_title` tool loaded by `opencode.json`.
It renames the current root session after an existing project `.spec` Markdown file.
Its plugin ID is `opencode-spec-title`.

`delegate/index.ts` is the server-side `task` tool replacement loaded by `opencode.json`.
It routes each task call to a per-call `{model, effort}`, waits for exhausted provider windows to reset, and runs the work in a child session.
Its plugin ID is `delegate-task`.

`shared/` contains cross-plugin helpers.
Keep helpers here only when more than one plugin owns the concept.

## Load Surfaces

Server plugins go in `opencode.json`.
TUI plugins go in `tui.json`.
Restart OpenCode after changing plugin entries or source; running sessions keep the loaded plugin set.

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

## Usage Sidebar Contract

`usage/index.tsx` reads OpenCode auth from `${XDG_DATA_HOME:-~/.local/share}/opencode/auth.json`.
It expects OAuth entries with access tokens for providers it can display.

The sidebar always displays OpenAI, Claude, xAI, and opencode-go sections so usage can guide model switching before the current session provider changes.
The active provider label uses the theme primary color.
Each provider's note renders inline beside its label as a one or two word marker, colored by state: `error` red, `warn` amber, `info` muted, and a stale marker muted.
A `ProviderUsage.noteKind` of `info` or `warn` marks a windowless note as a benign state, so it stays cached and visible instead of collapsing to pending or a red error.
Notes without `noteKind` keep the legacy red error path for OpenAI and Claude, so their `no auth`, `<status>`, and `no windows` markers stay red.
A provider with zero windows still renders placeholder rows with muted `--` percents, an empty bar, and `--` reset columns so column alignment matches healthy providers.
Placeholder labels are per provider: OpenAI and Claude use `H` and `W`, OpenCode uses `H`, `W`, and `M`, while xAI uses `W` and `M`; each adapter declares its `placeholders` and the UI stamps them onto `ProviderUsage`.
A window may carry a reset period with an unknown `usedPercent`, which renders as a muted `--` percent and empty bar but keeps the real duration and exact reset columns.
Provider responses are cached per provider under `${XDG_CACHE_HOME:-~/.cache}/opencode/usage-sidebar/`.
Provider lock files live under `${XDG_RUNTIME_DIR:-/tmp/opencode-${uid}}/opencode/` so multiple OpenCode sessions share one cache without stampeding private usage endpoints.
Network refreshes are allowed at most once per provider per minute; both the one-minute UI timer and session message events request a refresh, and `shouldFetch` plus the provider lock gate the actual fetch so idle instances re-fetch instead of showing stale data forever.
When a rate-limited provider has prior data, the sidebar keeps showing stale windows with a muted note rather than replacing them with an error.

### OpenAI Usage Adapter

`usage/openai.ts` expects an OpenAI OAuth entry with an access token.

The plugin calls `https://chatgpt.com/backend-api/wham/usage` with `Authorization: Bearer <token>`.
When possible it derives `ChatGPT-Account-Id` from `openai.accountId` or the OAuth JWT payload object `https://api.openai.com/auth` field `chatgpt_account_id`.

The response is expected to expose `rate_limit.primary_window` and optionally `rate_limit.secondary_window`.
Each window may contain `remaining_percent`, `used_percent`, `reset_at`, or `reset_after_seconds`.
The UI displays used percentage; if the API only returns remaining percentage, the plugin inverts it at the boundary.

This endpoint and shape are private ChatGPT implementation details.
OpenAI uses a one-minute minimum fetch interval, a one-minute transient-error backoff, and a ten-minute 429 backoff.
Failures should degrade to a coarse `unavailable`, `<status>`, `429`, or `no windows` marker rather than exposing local paths or token parsing details.

### Claude Usage Adapter

`usage/anthropic.ts` expects an Anthropic OAuth entry with an access token.

The plugin calls `https://api.anthropic.com/api/oauth/usage` with `Authorization: Bearer <token>`, `anthropic-beta: oauth-2025-04-20`, and `anthropic-version: 2023-06-01`.
The response is expected to expose `five_hour`, `seven_day`, and optional scoped weekly entries in `limits`.
Each window may contain `utilization` and `resets_at`.
Scoped weekly entries use `kind: "weekly_scoped"`, `group: "weekly"`, `percent`, `resets_at`, and `scope.model.display_name`.

The UI displays `H` for the five-hour window, `W` for the all-models weekly window, and one-letter scoped weekly model bars such as `F` for Fable.

This endpoint and shape are private Anthropic implementation details surfaced by Claude Code OAuth flows.
Claude uses a two-minute minimum fetch interval, a five-minute transient-error backoff, and a sixty-minute 429 backoff because this usage endpoint is much tighter than model inference.
Failures should degrade to a coarse `unavailable`, `<status>`, `429`, or `no windows` marker rather than exposing local paths or token parsing details.

### xAI Usage Adapter

`usage/xai.ts` reads only the Grok CLI auth at `~/.grok/auth.json`, never OpenCode's own xai OAuth.
OpenCode's refreshed xai token was rejected with 401 on the billing endpoint, while the Grok CLI token is accepted.

Before returning `no auth` or `expired`, the adapter runs `grok models` once with a short timeout and then rereads `~/.grok/auth.json`.
Set `GROK_CLI` to override the binary path; if noninteractive refresh still fails, login through Grok CLI.

That file is an object keyed by `<issuer>::<client_id>`; the adapter picks the entry whose `oidc_issuer` is `https://auth.x.ai`.
It reads `key` and `expires_at` only; it never reads or refreshes `refresh_token`.
If the file, entry, or key is missing it returns no windows with a `no auth` warn note; an expired `expires_at` returns `expired` without any network call.

When the token is fresh it polls two billing shapes in parallel with `Authorization: Bearer <key>`, `X-XAI-Token-Auth: xai-grok-cli`, `Accept: application/json`, and `User-Agent: opencode-usage`:

- `GET https://cli-chat-proxy.grok.com/v1/billing?format=usage` for the monthly credit pool (`used` / `monthlyLimit`, month `billingPeriodEnd`).
- `GET https://cli-chat-proxy.grok.com/v1/billing?format=credits` for the unified weekly period (`currentPeriod.end`) and optional `creditUsagePercent`.

A real monthly ratio becomes an `M` window; a real weekly `creditUsagePercent` becomes a `W` window.
When credits exposes only the weekly reset, `W` still renders with a muted `--` percent rather than inventing a weekly burn from monthly used.
Placeholder labels are `W` and `M` so a missing fetch still keeps column alignment.
xAI uses a five-minute minimum fetch interval, a five-minute transient-error backoff, and a fifteen-minute 429 backoff.

### opencode-go Usage Adapter

`usage/opencode-go.ts` has no API-key usage route upstream.
It reads only Firefox's `auth` cookie for `opencode.ai` from `cookies.sqlite` and replays it to `https://opencode.ai` console routes.
The cookie is never logged, cached, serialized, or sent to any other origin/provider.

The adapter first checks `/auth/status`, resolves the active workspace via `/auth`, discovers the SolidStart `queryLiteSubscription` server reference from the workspace `/usage` or `/go` route assets, then posts `[workspaceID]` to `/_server`.
The server function id is cached in memory for a short TTL because it is build/hash dependent.
Responses may be plain JSON or SolidStart/Seroval JavaScript chunks; the adapter extracts only the expected rolling, weekly, and monthly usage fields and does not evaluate remote code.

The adapter displays as `OpenCode` (its id stays `opencode-go` and the cache path is unchanged) and renders `H`, `W`, and `M` windows when available.
If Firefox is signed out or the browser session is expired, it shows the short warn note `sign in`.
`sign in` is a browser-session problem; there is still no API-key usage route.
Private console route drift degrades to `unavailable` or `no usage` rather than exposing local paths or cookie details.

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

## Spec Title Contract

`spec_title` is a server-only tool loaded by `opencode.json`.
It renames the current root session and does nothing else.
There is no TUI surface, no ledger, no persisted state, and no automation; nothing fires on session events.

The tool takes two args.
`path` is a project-relative or absolute `.spec/*.md` path.
`title` is exactly four ALL-CAPS or hyphenated words separated by single ASCII spaces, at most 28 characters total.

Title validation rejects leading, trailing, repeated, Unicode, tab, and newline whitespace.
Each word is `[A-Z0-9]` with optional internal hyphens, so `SPEC-TITLE FOUR WORD NAME` is accepted.

Path validation lexically rejects paths outside the project before resolving them with `realpath`.
It then requires the canonical target to be an existing regular `.md` file inside this project with a `.spec` path segment.
Paths that escape through symlinks or lack a `.spec` segment are rejected.

The tool only runs in root sessions; a session with a `parentID` is rejected.
On success it updates the session title through the OpenCode client and returns `session titled <title>`.

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
Traversal aliases and existing filesystem aliases share one canonical row; the latest completed read supplies the path, label, and compacted state.

The global OpenCode config-root `AGENTS.md` file is intentionally omitted because it is always loaded for this setup.
The sidebar section is hidden until at least one Markdown read exists.

Compacted read entries are shown with a red `C` marker when OpenCode marks the completed tool part that way.
Fresh read entries use source markers: green `R` for `README.md`, blue `󰯉` for `AGENTS.md`, yellow `I` for uppercase pointer docs like `GO.md` or `DATABASE.md`, and muted `M` for generic Markdown.
Markdown files directly in the OpenCode config root are doctrine, except existing `README.md` and `AGENTS.md` kinds.
Fresh doctrine entries use the yellow `icons.doctrine` scroll glyph and the extensionless basename.
Any read under a `.spec` path segment takes precedence over filename kinds and renders with the cyan spec glyph (`icons.spec`), regardless of the file's name.
Spec labels remove the `.spec` segment and render the owning directory plus the extensionless path below `.spec`.
Carrier files omit their filename in labels because the marker carries that information.

## Delegate Contract

The `task` tool accepts optional `model`, `effort`, and `task_id` args beyond the upstream trio.
`model` takes `provider/model-id`; when omitted the child uses the agent's model or inherits the current assistant message's model and effort.
`effort` maps to the target model's reasoning variants; an invalid effort errors listing the valid efforts.
An unknown `subagent_type` errors listing the known agents.
`task_id` resumes an existing child session instead of creating one.
Drive parents reject `task_id` resumes so a stale child cannot keep older prompt-shaped permissions; re-brief a fresh child instead.

The card description gets exactly one trailing `· effort` suffix.
Any trailing known-effort suffixes are stripped before appending, so `task_id` resumes never accumulate `· high · high`.
This display is a local description suffix until upstream TUI renders `message.variant`.

`ContentFilterError` and refusal-shaped errors (a tolerant classifier on error name and message) return a normal delegate result instead of throwing.
That result has `state="error"` and its output carries `blocked: content_filter`, the `child_session_id`, and re-brief advice.
There is no auto-retry and no taint persistence.
The policy: never resume a refusal-tainted child; reword the brief first, switch provider as a last resort.

`config/opencode/delegate.json` is the provider allowlist for delegation.
Delegating to a provider missing from `providers` errors; extend `delegate.json` deliberately.
Delegate reads the usage-sidebar cache under `${XDG_CACHE_HOME:-~/.cache}/opencode/usage-sidebar/` before spawning.
When any usage window is at 100%, the tool waits abortably for the latest reset and then proceeds.
There is no maximum wait and no stale-cache cutoff.
Metadata notes are prepended to the child's output in brackets.

Children inherit the parent session's deny rules and all `external_directory` rules.
Review leaves also get an explicit read-only profile: read/search/web/LSP are allowed, root-path grep is denied, and edit, bash, task, todowrite, and question are denied unless that leaf declares its own permission rules.
When the parent session is Drive, inherited `ask` rules are converted to child-session denies so unattended review cannot surface a TUI approval prompt.
`todowrite` and `task` are denied unless the agent's own permissions declare them, and every `experimental.primary_tools` tool is denied.

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
