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

Update `../configs/newtab.yaml` for your machine. The optional `../configs/newtab.local.yaml` file can hold untracked overrides.

```yaml
port: ":42069"
firefox_db: ".mozilla/firefox/YOUR_PROFILE/places.sqlite"
static_dir: "dotfiles/daemons/newtab"
history_limit: 15
```

Set Firefox to use it:

1. `about:config` → `browser.newtabpage.enabled` = `false`
2. Install [New Tab Override](https://addons.mozilla.org/en-US/firefox/addon/new-tab-override/)
3. Set custom URL to `http://localhost:42069`

## Install

```sh
install-go.sh newtab
```

Runs on `:42069`. Started automatically via `exec-once = newtab` in `hyprland.conf`.
