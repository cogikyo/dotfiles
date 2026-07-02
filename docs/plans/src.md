# `src` — source inspection cache: build spec

Status: implemented in this session, commit pending.
Post-restart `verify/source` dry run and user `sudo pacman -S devtools` are still pending.
Companion: `verify/source.md` agent rewiring in slice 4.

Objective: replace ad-hoc `/tmp/opencode` clones with a persistent, machine-owned inspection cache and a `src` CLI, then wire agent permissions to it.

Decisions locked:

- Cache lives at `~/.cache/src`.
- Standalone `src` binary in `cmds/`.
- `/tmp` clone fallback removed.
- `devtools` added to packages.
- scout/architect get read-only subcommands.

## Cache layout

- Root: `${XDG_CACHE_HOME:-$HOME/.cache}/src`.
- Entry: `<host>/<path...>/<repo>@<ref>`; repo strips `.git`.
- Tags verbatim; sha pins 12-hex; default branch is `@default`.
- Detached HEAD checkouts.
- Go modules are never cloned (GOMODCACHE tier via the go tool).
- Pinned entries are immutable; `@default` refreshes only with `-u`.

## CLI

Stdlib only, no new deps.
Paths on stdout, diagnostics on stderr.

- `src find <query> [-C dir] [-a]` — never touches the network.
  - Tiers: Go module via `go list -m -f '{{.Dir}}'` (explicit error if no go.mod context); node_modules walk-up; bare repo name in `~/repos` then cache; absolute path via `pacman -Qo`.
  - `-a` prints all tiers; a miss exits 1 and lists the tiers tried.
- `src get <spec> [-u]` — spec forms: URL | `host/org/repo` shorthand | Go `module@version` via `go mod download -json` | `npm:<pkg>[@ver]` via registry `repository.url` (no tarball fallback) | `arch:<pkg>` via `https://gitlab.archlinux.org/archlinux/packaging/packages/<pkg>.git`.
  - Git tier: `git clone --filter=blob:none --depth 1 --single-branch [--branch <ref>]`; sha pins via init + `fetch --depth 1 <sha>` + checkout FETCH_HEAD.
  - `-u` only refreshes `@default` (fetch+reset, or delete+reclone); `-u` on a pin is an error.
  - A failed pinned ref never falls back; a partial clone removes the dest and propagates the error.
- `src ls` — entries with ref, size, mtime age; empty prints "no entries" and exits 0.
- `src prune [--older-than Nd] [--all] [entry...]` — default 60d by dir mtime, includes pins; `--all` wipes the root; an unknown entry name is an error.

## Error contract

- Non-zero exit plus one-line actionable stderr.
- No silent tier fallthrough.
- No network in `find`.
- No auto-refresh.
- No ref fallback.
- No partial-state repair.

## Slices with verification

1. find + ls: `cmds/cmd/src/main.go`, `cmds/internal/src/{src,find,ls}.go`.
   - Verify: gofmt/vet/build; `src find golang.org/x/sys` from `cmds/` hits GOMODCACHE; a miss exits 1; `src find opencode` → `~/repos/opencode`; `src ls` empty-ok.
2. get: `internal/src/{get,resolve}.go`.
   - Verify: `github.com/junegunn/fzf@v0.60.0` clones then the second run is instant; detached HEAD + blob:none config present; `golang.org/x/text@v0.14.0` → GOMODCACHE with no clone; `arch:jq` has a PKGBUILD; `@badref` errors with no fallback and no dest dir.
3. prune: `internal/src/prune.go`.
   - Verify: fake old entry pruned, fresh entry kept, unknown entry errors.
4. Agent wiring.
   - Precondition: resolve `repo_clone`/`repo_overview` provenance (`rg -ln "repo_clone|repo_overview" ~/repos/opencode --iglob '!*.md'`; also check `~/.local/share/opencode`); dead keys → remove from `verify/source.md` only.
   - Edit `config/opencode/agents/verify/source.md`: swap the four `/tmp` clone ask-patterns for `src find *`: allow, `src ls`: allow, `src get *`: ask, `src prune*`: deny.
   - Delete the line-40 TODO; rewrite the discovery ladder disk-before-network.
   - Guardrails name `~/.cache/src`, `~/.go/pkg/mod`, and `~/repos` as sanctioned read scopes.
   - Report contract: "Temp clone path..." → "Cache entry path used".
   - Add `src find *` and `src ls` allows to `review/scout.md` and `plan/architect.md`.
   - Verify with the rg checks, then a post-restart dry verify/source task.
5. Packaging: `devtools` → `etc/packages.lst` (alphabetical); `cmds/README.md` commands list + tree.
   - Verify: rg checks plus `./install.sh --check`; the user runs the pacman install themselves.

## Traps

- GOPATH is `~/.go`, so GOMODCACHE is `~/.go/pkg/mod`; always read `go env GOMODCACHE`.
- The module cache is 0444 read-only.
- Path escaping is handled by the go tool; never reimplement it.
- Keep `internal/src` stdlib-self-contained; no `internal/dctl` reuse.
- `config/opencode` edits are live via symlink; restart needed for slice 4.
- `etc/repos.toml` machinery is out of scope.
