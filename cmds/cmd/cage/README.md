# cage

Linux command sandbox for OpenCode read-only shells, installed in `~/.local/bin/` by dctl with the other commands.
Requires unprivileged user namespaces and Landlock ABI 9 or newer; missing support fails closed.

## Usage

```sh
cage [--hide <path>]... <scratch-dir> -- <command> [args...]
```

The scratch path must be clean and absolute, and must not resolve to `/`.
Cage creates `tmp/` and `cache/` beneath it and sets `TMPDIR`, `XDG_CACHE_HOME`, and `OPENCODE_SCRATCH`.
Invoke a shell explicitly when the command needs shell syntax.

## Policy layers

- New user, mount, network, PID, and IPC namespaces isolate the command; all mounts are read-only except scratch, with a fresh `/proc`.
- Landlock denies reads beneath `--hide` paths and limits `/dev` access to `null`, `zero`, `full`, `random`, and `urandom`.
- Scratch permits file and directory creation and in-place writes, but denies deletion, rename, truncation, and symlink creation; `tmp/` and `cache/` permit those operations.
- Landlock denies Unix-socket connects and scopes signals and abstract sockets to the sandbox.
- The command has zero capabilities, `no_new_privs`, a zero core-size limit, and only inherited file descriptors 0–2.
- A PID 1 supervisor reaps children; its exit kills remaining processes, including when the caller disappears.

## Caller trust and limits

Cage trusts the caller's scratch path; it does not assign sessions or enforce ownership of that path.
The [guard plugin](../../../config/opencode/plugins/README.md#guard) supplies `~/.local/state/opencode/scratch/<session-id>` and the hide list from `guard/paths.ts`.

- Shells are offline: cold `uv` or `pip` downloads and authenticated `gh` or `pier` calls do not work.
- Scratch-root files remain overwritable in place, so they are not immutable evidence; tools that truncate or atomically replace files must use `tmp/` or `cache/`.
- Secret protection depends on the caller's hide inventory; unlisted secrets remain readable, and missing hide paths are skipped.
- Hidden entry names can remain visible in ancestor directory listings; `/` itself is unlistable, and new entries in hidden paths' ancestors remain unreadable after the policy walk.
- There is no syscall filter or disk quota.
- Core dumps are disabled, but crash metadata can still reach the system journal.
