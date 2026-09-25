# Claude accounts

The local entrypoint creates two instances of `opencode-claude-auth@2.2.1` with fixed credential directories.
The dependency, Bun lockfile, and versioned patch live in `config/opencode/`.
The patch changes the published JavaScript directly; there is no upstream build step.

| Account | Provider | Claude directory |
| --- | --- | --- |
| Trend | `anthropic` | `~/.claude` |
| Cogikyo | `anthropic-personal` | `~/.claude-cogikyo` |

Credentials stay outside the repository.
The auth plugin and usage adapters share the account definitions in `accounts.ts`.
The `claude-auth` function in `config/zsh/zshrc` must use the same directories.

## Login and selection

Open a new zsh shell after changing the function, then authenticate the account you need:

```zsh
claude-auth trend
claude-auth cogikyo
```

Each command opens Claude Code's subscription login and displays auth status afterward.
Choose the intended browser account and check the resulting identity; the directory name does not enforce which account you select.
Existing Trend credentials need no migration or new login if they still work.

Restart the OpenCode server and TUI after changing the plugin or config.
Select `anthropic/claude-opus-5-5` or `anthropic-personal/claude-opus-5-5` in the model picker or delegation request.
The personal provider also exposes `claude-fable-5-1`.
Its model capabilities and limits in `opencode.json` were copied from the local model catalog and need review when adding models.

The sidebar labels are **Anthropic [Trend]** and **Anthropic [Cogikyo]**.
Usage caches and delegate quota checks use distinct provider IDs.
The existing delegate gate is provider-wide: it checks every reported window, including model-scoped windows.
There is no automatic quota rollover; selecting personal for work sends that task's context through the personal account.

## Refresh and failure behavior

Each plugin instance owns its credential cache, active account, and excluded beta flags.
File-backed refresh identities include the credential directory, so account cooldowns and refresh locks stay separate.
Auth-store updates go through OpenCode's auth API using the selected provider ID and are serialized within this plugin's process.
Upstream request formatting and response transformations remain in the dependency.

Fixed profiles never borrow another account's credentials or invoke the auth plugin's Claude CLI fallback.
Missing credentials still register an OAuth placeholder so requests fail with the account's login command instead of using an environment API key.
The provider config's `oauth-required` API key is a non-secret failure sentinel, not a usable credential.

The usage adapter retains bounded Claude CLI recovery after a 401, with the chosen config directory and common auth-token environment overrides cleared.
That recovery can make a small Haiku request.
Its cooldown is independent for each account.
Do not run manual login concurrently with active requests for that account: the plugin's refresh lock does not establish coordination with Claude Code's own login or refresh.

## Install and update

Use Bun: other package managers do not apply `patchedDependencies`.
From `config/opencode/`, install the recorded version and patch with:

```bash
bun install --frozen-lockfile --ignore-scripts
```

For an upstream update, work in a temporary copy of this package first, outside the live symlinked config.
Install the new exact version with scripts disabled, use `bun patch` to port only the account changes, then record it with `bun patch --commit`.
Keep request compatibility code close to upstream, and inspect changes even when the old patch applies cleanly.
Move the reviewed dependency, lockfile, and patch changes back together only after checks pass.

Cheap checks from `config/opencode/`:

```bash
./node_modules/.bin/tsc --noEmit --project tsconfig.json
./node_modules/.bin/oxlint plugins/anthropic plugins/usage/anthropic.ts plugins/usage/auth.ts plugins/usage/providers.ts plugins/usage/adapters.ts
node --check node_modules/opencode-claude-auth/dist/index.js
node --check node_modules/opencode-claude-auth/dist/credentials.js
node --check node_modules/opencode-claude-auth/dist/keychain.js
node --check node_modules/opencode-claude-auth/dist/betas.js
zsh -n ../zsh/zshrc
git diff --check
```

After installation and restart, live verification needs one approved request through each provider and confirmation that the two usage meters report the intended accounts.
Syntax checks do not establish OAuth or concurrent-refresh correctness.

To roll back an update, restore the previous dependency version, lockfile, and patch together, reinstall with Bun, and restart OpenCode.
Keep the previous working files until live verification succeeds.
Credential directories do not need to be removed or copied during rollback.
