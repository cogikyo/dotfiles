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

## Statusline Contract

`opencode/statusline.tsx` registers `session_prompt`.
That slot wraps `api.ui.Prompt` rather than replacing input behavior.

The slot receives `session_id`, `visible`, `disabled`, `on_submit`, and `ref`.
The wrapper forwards those to `api.ui.Prompt` as `sessionID`, `visible`, `disabled`, `onSubmit`, and `ref`.
Only `hint` is customized.
`right` belongs to OpenCode's prompt metadata line, not the preview/status row.

The statusline intentionally omits provider, model, and effort because OpenCode already renders those in prompt metadata.
It owns cwd, git status, and a Claude-style context pressure bar on the preview/status row.
OpenCode's native prompt keeps numeric context usage and command-list keybind hints on the right side of that row.

## Typechecking

Run from `config/opencode`:

```sh
node node_modules/typescript/bin/tsc --noEmit --project tsconfig.json
```

The Hyprd plugins currently keep `@ts-nocheck` because OpenCode plugin event payload types are incomplete locally.
Remove that suppression once local event and payload types exist.
