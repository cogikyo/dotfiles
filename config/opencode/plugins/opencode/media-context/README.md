# Media Context plugin

Registers local image file parts and local video paths for reuse in an opencode session.

- Handles are per-session timestamps: `@HH_MM_SS`, with `_2`, `_3`, and so on when multiple media files register in the same second.
- Newly attached images can also receive async model-generated aliases such as `@whiteboard-sketch`.
- Image aliases are limited to 3 words, slugged to lowercase safe characters, and collision-suffixed when needed.
- The sidebar lists named images as `I <generated-name>` and unnamed images as `I <timestamp-handle>`.
- Videos display `V <local-basename>` with handle fallback and are never model-named or pasted into provider context.
- Clicking an image row opens the local image in the Kitty overlay and clears it on close, session changes, and plugin disposal.
- Clicking a video row opens the local path with `xdg-open`.
- Typing a known image handle or alias later attaches only provider-safe image file parts.
- Typing a known video handle stays local-only and emits a notice instead of attaching the video.
- Compaction context includes available media handles or aliases whose backing files still exist.
- Attached and pasted image file parts are registered as images.
- Clipboard `data:image/...;base64,...` URLs are materialized on send up to 2 MiB; oversized data images are skipped.
- Absolute local video paths in submitted user text are detected for `.mp4`, `.mov`, `.mkv`, `.webm`, `.avi`, and `.m4v`.
- Video paths must exist as files under `/home/cullyn/`, `/tmp/`, or `$XDG_RUNTIME_DIR/`; both the typed path and realpath must stay under those roots.

Image naming is configured on the server plugin tuple in `opencode.json`:

```json
[
  "file:///home/cullyn/dotfiles/config/opencode/plugins/opencode/media-context/prompt.ts",
  { "imageNames": { "enabled": true } }
]
```

The naming job is sidecar-only.
It queues only images registered by the `chat.message` send path and starts naming asynchronously after registration.
It reuses the current OpenCode prompt model and auth by creating a temporary OpenCode session, prompting it with the image, then deleting that session in a best-effort `finally`.
If the current prompt model is unavailable, the plugin falls back to the configured default OpenCode model when OpenCode exposes it to the config hook.
It does not read `OPENAI_API_KEY`, parse OpenCode auth files, or call provider APIs directly.
If the provider/model lacks image support or the temporary prompt fails, naming warns once for that image and leaves timestamp handles intact.
OpenCode docs do not currently expose a true non-persistent completion API, so temp session rows and stats are deleted best-effort but true non-persistence depends on OpenCode API support.

Named image files are copied into the media-context runtime cache under the generated filename.
Original source files are not renamed.
Timestamp handles remain stable fallback references.

Registry reads are capped at 256 KiB and 200 entries.
Registry directories are mode `0700`; registry JSON files are mode `0600`.
Registry writes use a per-session lock and atomic replace.

Restart OpenCode after changing this plugin or its `opencode.json` tuple options.
