# newtab

Custom Firefox new tab page.

`newtab` is a local HTTP server that reads Firefox `places.sqlite` for bookmarks and history.
It also proxies Google suggestions for the search box.

## Why

Firefox's built-in new tab is slow and cluttered.
Replacement extensions usually need sync or broad browser APIs to reach bookmarks and history.

This keeps the page local and reads Firefox's SQLite database directly.

## API

- `/api/bookmarks` — all bookmarks with folders, keywords, and tags
- `/api/history?q=<query>` — history search ranked by visit count + recency
- `/api/suggest?q=<query>` — proxied Google suggestions
- `/` — serves the static frontend from `cmds/cmd/newtab/`

## Setup

No config file.

At startup, the server scans `~/.mozilla/firefox` and `~/.config/mozilla/firefox` for `places.sqlite`.
It prefers `dev-edition-default`, then falls back to the first readable profile.

Port, static directory, and history limit are constants in `main.go`.

Set Firefox to use it:

1. `about:config` → `browser.newtabpage.enabled` = `false`
2. Install [New Tab Override](https://addons.mozilla.org/en-US/firefox/addon/new-tab-override/)
3. Set custom URL to `http://localhost:42069`

## Install

```sh
go build -o ~/.local/bin/newtab ./cmd/newtab
```

Run from `cmds/`, or use `./install.sh go` from the repo root to build all commands.

`newtab` listens on `:42069`.
It can run as a user service with `cmds/cmd/newtab/newtab.service`.
