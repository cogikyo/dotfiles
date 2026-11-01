# cage

Linux command sandbox for OpenCode read-only shells, installed in `~/.local/bin/` by dctl with the other commands.
Requires unprivileged user namespaces and Landlock ABI 9 or newer; missing support fails closed.

## Usage

```sh
cage [--hide <path>]... <scratch-dir> -- <command> [args...]
```

The scratch path must be clean and absolute, and must not resolve to `/`.
Cage creates `tmp/` and `cache/` beneath it and sets `TMPDIR`, `XDG_CACHE_HOME`, and `OPENCODE_SCRATCH`.
When `go` is on `PATH`, it also creates `cage/go-build/` under the caller's original `XDG_CACHE_HOME` (default `~/.cache`) and sets `GOCACHE` to it.
Cage trusts that base but refuses to start if `cage/` or `cage/go-build/` is a symlink or not a directory.
Invoke a shell explicitly when the command needs shell syntax.

## Policy layers

- New user, mount, network, PID, and IPC namespaces isolate the command; all mounts are read-only except scratch and the Go build cache, with a fresh `/proc`.
- Landlock denies reads beneath `--hide` paths and limits `/dev` access to `null`, `zero`, `full`, `random`, and `urandom`.
- Scratch permits file and directory creation and in-place writes, but denies deletion, rename, truncation, and symlink creation; `tmp/` and `cache/` permit those operations.
  The Go build cache permits reads, file and directory creation, writes, deletion, and truncation, but no symlink creation or cross-directory links or renames.
- Landlock denies Unix-socket connects and scopes signals and abstract sockets to the sandbox.
- The command has zero capabilities, `no_new_privs`, a zero core-size limit, and only inherited file descriptors 0–2.
- A PID 1 supervisor reaps children; its exit kills remaining processes, including when the caller disappears.

## Caller trust and limits

Cage trusts the caller's scratch path; it does not assign sessions or enforce ownership of that path.
The [guard plugin](../../../config/opencode/plugins/README.md#guard) supplies `~/.local/state/opencode/scratch/<session-id>` and the hide list from `guard/paths.ts`.

- Shells are offline: cold `uv` or `pip` downloads and authenticated `gh` or `pier` calls do not work.
  With `go` on `PATH`, cage sets `GOPROXY=file://<GOMODCACHE>/cache/download` from the host's `go env GOMODCACHE`, so modules resolve from its read-only download cache.
  If that lookup fails, cage warns and sets `GOPROXY=off`; sumdb settings are unchanged.
- All cages share the Go build cache, so one caged command can plant entries a later caged build trusts; the host's own build cache is untouched.
- Scratch-root files remain overwritable in place, so they are not immutable evidence; tools that truncate or atomically replace files must use `tmp/` or `cache/`.
- Secret protection depends on the caller's hide inventory; unlisted secrets remain readable, and missing hide paths are skipped.
- Hidden entry names can remain visible in ancestor directory listings; `/` itself is unlistable, and new entries in hidden paths' ancestors remain unreadable after the policy walk.
- There is no syscall filter or disk quota.
- Core dumps are disabled, but crash metadata can still reach the system journal.
