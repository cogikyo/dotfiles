# newtab

Custom Firefox new tab page. HTTP server that reads directly from the Firefox places database to provide bookmarks, history search, and Google suggestions — all local, all instant.

## Why

Firefox's built-in new tab is slow and cluttered. Extensions that replace it can't access bookmarks/history without syncing to some external service. This just reads the SQLite database directly.

## API

- `/api/bookmarks` — all bookmarks with folders, keywords, and tags
- `/api/history?q=<query>` — history search ranked by visit count + recency
- `/api/suggest?q=<query>` — proxied Google suggestions
- `/` — serves the static frontend (index.html, app.js, style.css)

## Setup

No config file. The server picks its Firefox profile at startup by scanning `~/.mozilla/firefox` and `~/.config/mozilla/firefox` — preferring `dev-edition-default`, then any profile marked `Default=1` in `profiles.ini`. Port, static dir, and history limit are baked in as constants in `main.go`.

Set Firefox to use it:

1. `about:config` → `browser.newtabpage.enabled` = `false`
2. Install [New Tab Override](https://addons.mozilla.org/en-US/firefox/addon/new-tab-override/)
3. Set custom URL to `http://localhost:42069`

## Install

```sh
install-go.sh newtab
```

Runs on `:42069`. Started automatically via `exec-once = newtab` in `hyprland.conf`.
