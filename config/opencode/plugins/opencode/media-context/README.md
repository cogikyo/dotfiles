# Media Context plugin

Registers local image file parts and local video paths for reuse in an opencode session.

- Handles are per-session timestamps: `@HH_MM_SS`, with `_2`, `_3`, and so on when multiple media files register in the same second.
- The sidebar lists current-session images as `I <timestamp-handle>` and videos as `V <local-basename>` with handle fallback.
- Clicking an image row opens the local image in the Kitty overlay and clears it on close, session changes, and plugin disposal.
- Clicking a video row opens the local path with `xdg-open`.
- Typing a known handle later attaches only provider-safe media URLs; local files remain sidebar/local-open references.
- Compaction context includes available media handles whose backing files still exist.
- Attached and pasted image file parts are registered as images.
- Clipboard `data:image/...;base64,...` URLs are materialized on send up to 2 MiB; oversized data images are skipped.
- Absolute local video paths in submitted user text are detected for `.mp4`, `.mov`, `.mkv`, `.webm`, `.avi`, and `.m4v`.
- Video paths must exist as files under `/home/cullyn/`, `/tmp/`, or `$XDG_RUNTIME_DIR/`; both the typed path and realpath must stay under those roots.

The plugin has no model renaming, naming aliases, or legacy ordinal handle schemes.

Registry reads are capped at 256 KiB and 200 entries.
Registry directories are mode `0700`; registry JSON files are mode `0600`.
Registry writes use a per-session lock and atomic replace.
