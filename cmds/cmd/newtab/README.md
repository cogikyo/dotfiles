# newtab

Custom Firefox new tab page.

`newtab` is a local HTTP server that reads Firefox's SQLite database `places.sqlite` directly for bookmarks and history.
It also proxies Google suggestions for the search box.

## API

- `/api/bookmarks` — all bookmarks with folders, keywords, and tags
- `/api/history?q=<query>` — history search ranked by visit count + recency
- `/api/suggest?q=<query>` — proxied Google suggestions
- `/` — serves the static frontend from `cmds/cmd/newtab/`

## Setup

No config file.

At startup, the server reads `profiles.ini` under `~/.mozilla/firefox` and `~/.config/mozilla/firefox` to find `places.sqlite`.
It prefers a profile named `dev-edition-default`, then uses the first profile with an existing database.
Run Firefox once before starting newtab so that the database exists.

Port, static directory, and history limit are constants in `main.go`.

Set Firefox to use it:

1. `about:config` → `browser.newtabpage.enabled` = `false`
2. Install [New Tab Override](https://addons.mozilla.org/en-US/firefox/addon/new-tab-override/)
3. Set custom URL to `http://localhost:42069`

## Install

The dctl ISO installs the binary into `~/.local/bin/`.
For an attended source update, run as your normal user:

```sh
dctl update --only newtab
```

This rebuilds newtab and restarts it only if its binary changed.
The tracked user unit is `config/systemd/user/newtab.service` at the repo root.
`dctl setup home` links the user unit directory and its `.wants` links without starting or restarting the service.

`newtab` listens on all interfaces at `:42069` and serves static files from `~/dotfiles/cmds/cmd/newtab/`.
Treat the bookmarks and history API as private; the server has no authentication.
