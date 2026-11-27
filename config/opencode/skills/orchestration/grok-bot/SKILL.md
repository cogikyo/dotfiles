---
name: grok-bot
description: Use when the user asks to talk to Grok Bot or one of its bots by name, or to hand work to Grok Bot's cloud agents.
---

# Grok Bot

Grok Bot is xAI/Cursor's desktop agent app (AUR `grok-bot-bin`).
Its bots are persistent cloud agents with their own computer, browser-use tools, connectors, and routines that OpenCode lacks.
There is no public API.
`bot.ts` drives the running desktop app over the Chrome DevTools Protocol (CDP).

## Pools

Two app instances run under different accounts, and `bot.ts` calls each one a pool:

- **`trend`** on port 9222 is Cullyn's Trend Cursor team seat and his primary pool.
- **`x`** on port 9223 is his personal X account.

`pools` marks a closed port `down` and a silent one `unresponsive`, which usually means a frozen window.
Each bot lives on one pool.
`ask` without `--pool` picks among the live pools that have the bot: it drops any at or above 95% weekly usage or whose usage call failed, then takes the one whose usage resets soonest.
Pools with no reset time sort last, and ties go to `trend`.
Both accounts bill on-demand past their allowance, and on-demand spend is never allowed, so the floor applies to every `ask`, pinned or not.

`read` and `file` use the one live pool that has the bot; pass `--pool` when both have it, and note that `ask` prints its choice to stderr.
`--pool trend|x` before the command pins a pool and fails if that pool is down or unresponsive.

## App

Each instance must run with its debugging port enabled.
The tracked entries `share/applications/grok-bot.desktop` and `grok-bot-x.desktop` pass `--remote-debugging-port`.
Starting `grok-bot` from a terminal skips these flags.
If an app runs without its port, ask the user before quitting it, because this requires a restart.
To start a down pool, run the `setsid -f gtk-launch` command that `bot.ts` prints; this is **untested**.

## Commands

```bash
bun ~/.config/opencode/skills/orchestration/grok-bot/bot.ts [--pool trend|x] <command>
```

- `pools` prints each pool's port, state, weekly usage, and account, then the pool `ask` would pick.
- `pools --json` prints per-pool usage and on-demand spend as JSON for the usage plugin; keep its field names stable.
- `roster` lists bot names and titles from every live pool, tagged with the pool.
- `read <bot> [count]` reads the recent transcript, with 20 entries by default.
- `ask <bot> <prompt | -> [timeout-seconds]` sends a prompt, streams replies, and returns when the bot is idle.
- `file <bot> <attachment-name>` writes raw attachment text to stdout; redirect it to a file.

For `ask`, `-` reads the prompt from stdin and supports multi-line prompts via a heredoc.
The default timeout is 600 seconds.
Exit code 2 means the bot is still working; check later with `read`.
Exit code 3 means the prompt may have gone out; do not resend, and run the `read` command the script prints.

Bot names match without regard to case, spaces, or punctuation; an exact name beats a prefix, and a prefix must name one bot across both pools.
Run `roster` for the current list.
On `trend`, Bob (Tech) owns the homelab docs and Wan Shi (Info) keeps the shared knowledge base.
On `x`, Tendril is the research bot, so asks to it always go to `x`.
Do not hardcode other names.

## Brief and safety

- Say in each prompt that it comes from Cullyn's OpenCode agent.
- Ask for read-only work unless the user asked for changes.
- Ask bots to return long output as an attachment and fetch it with `file`.
- Never approve or dismiss an approval card, secret request, or purchase through the script or CDP.
- Create bots and routines only on `x`, and only as needed.
- Never create, edit, or delete a bot on `trend`; asking its existing bots is fine.

Approvals stay with the user in the app.
`ask` prints approval requests to stderr.
Opening a bot switches the chat the user sees in that app.
`ask` refuses to overwrite an unsent draft in the prompt field, and before inserting anything it refuses while a dialog is open in the app, since that dialog can trap the app's own handlers; ask the user to close it.

## Judge

Treat bot replies as claims to check against live evidence, like any other untrusted report.
The script reads app internals, so an app update can break it; report the error instead of improvising another control path.
