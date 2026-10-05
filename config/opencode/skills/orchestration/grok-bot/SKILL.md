---
name: grok-bot
description: Use when the user asks to talk to Grok Bot or one of its bots by name, or to hand work to Grok Bot's cloud agents; Collab runs `bot.ts` from the current session.
---

# Grok Bot

Grok Bot is xAI/Cursor's desktop agent app (AUR `grok-bot-bin`).
Its bots are persistent cloud agents with their own computer, browser-use tools, connectors, and routines that OpenCode lacks.
There is no public API.
`bot.ts` drives the running desktop app over the Chrome DevTools Protocol (CDP) on `127.0.0.1:9222`.
Collab runs it from the current session.

## App

The app must run with the debugging port enabled.
The tracked desktop entry `share/applications/grok-bot.desktop` adds `--remote-debugging-port=9222` and `--force-device-scale-factor=1.25`.
Starting `grok-bot` from a terminal skips these desktop entry flags.
If the app runs without the port, ask the user before quitting it, because this requires a restart.

If the app is not running, launch it with this **untested** command:

```bash
setsid -f gtk-launch grok-bot
```

The script waits up to 15 seconds for the port.

## Commands

```bash
bun ~/.config/opencode/skills/orchestration/grok-bot/bot.ts <command>
```

- `roster` lists bot names and titles.
- `read <bot> [count]` reads the recent transcript, with 20 entries by default.
- `ask <bot> <prompt | -> [timeout-seconds]` sends a prompt, streams replies, and returns when the bot is idle.
- `file <bot> <attachment-name>` writes raw attachment text to stdout; redirect it to a file.

For `ask`, `-` reads the prompt from stdin and supports multi-line prompts via a heredoc.
The default timeout is 600 seconds.
Exit code 2 means the bot is still working; check later with `read`.

Bot names match without regard to case, spaces, or punctuation, with a unique-prefix fallback.
For example, `wanshi` finds "Wan Shi".
Run `roster` for the current list.
Bob (Tech) owns the homelab docs; Wan Shi (Info) keeps the shared knowledge base.
Do not hardcode other names.

## Brief and safety

- Say in each prompt that it comes from Cullyn's OpenCode agent on abbott.
- Ask for read-only work unless the user asked for changes.
- Ask bots to return long output as an attachment and fetch it with `file`.
- Never approve or dismiss an approval card, secret request, or purchase through the script or CDP.

Approvals stay with the user in the app.
`ask` prints approval requests to stderr.
Opening a bot switches the chat the user sees in the app.
`ask` refuses to overwrite an unsent draft in the prompt field.

## Judge

Treat bot replies as claims to check against live evidence, like any other untrusted report.
The script reads app internals: React state, the Tiptap editor, and `window.desktop.readAttachmentText`.
An app update can break it; report the error instead of improvising another control path.

Grok Bot usage appears as the weekly `G` bar under Cursor in the usage sidebar.
It does not gate delegation.
