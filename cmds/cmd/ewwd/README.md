# ewwd

System utilities daemon for eww statusbar integration. Uses a provider-based architecture where each provider monitors a system resource and pushes updates to subscribers over a Unix socket.

## Commands

```bash
ewwd                     # start daemon (foreground, auto-opens eww windows)
ewwd open                # reload eww config and reopen configured windows
ewwd status              # check if running
ewwd status --json       # full state dump
```

### Query and subscribe

```bash
ewwd query               # all state as JSON
ewwd query audio         # specific provider

ewwd subscribe audio music  # stream events (for eww deflisten)
```

eww integration:

```yuck
(deflisten ewwd `ewwd subscribe audio music date weather`)
(label :text {ewwd?.audio?.sink?.volume ?: "?"})
```

### Actions

Triggered by eww button clicks and scroll events.

```bash
# Audio
ewwd action audio mute <sink|source>      # mute device
ewwd action audio change_volume sink up   # adjust ±10
ewwd action audio set_default both        # preset volumes

# Music (Spotify)
ewwd action music play                    # start playback
ewwd action music pause                   # pause playback
ewwd action music toggle                  # play/pause
ewwd action music next                    # next track
ewwd action music previous                # previous track
ewwd action music volume up [0.05]        # increase volume
ewwd action music volume down [0.05]      # decrease volume
ewwd action music seek up                 # seek forward 10s
ewwd action music seek down               # seek backward 10s

# Timer/Alarm
ewwd action timer timer start             # start countdown
ewwd action timer timer reset             # stop and reset to 01:30
ewwd action timer timer up <minutes>      # add minutes
ewwd action timer alarm start             # start alarm countdown
ewwd action timer alarm reset             # stop and reset to +6 hours
ewwd action timer alarm up <minutes>      # add minutes
```

## Providers

| Provider   | Source              | Data                                      |
|------------|---------------------|-------------------------------------------|
| audio      | PulseAudio          | sink/source volumes with offset           |
| music      | D-Bus (Spotify)     | playback status, track info, album art    |
| network    | /proc/net/dev       | upload/download speeds                    |
| date       | time                | time, date, clockface icons, weeks alive  |
| weather    | OpenWeatherMap API  | temperature, conditions, moon phase, wind |
| timer      | internal            | countdown timer and alarm                 |

Each provider implements the `providers.Provider` interface and runs in its own goroutine. Providers that support user interaction also implement `providers.ActionProvider`.

## Configuration

`../config/ewwd.yaml` — provider settings, API keys, poll intervals.

`ewwd` reads its config from `cmds/config/` and uses it to initialize providers, set polling cadence, and wire provider-specific options.

## Structure

```
ewwd/
├── daemon.go            # lifecycle, provider coordination, command handler
├── main.go              # CLI entry, command routing to daemon socket
├── providers/           # audio, music, network, date, weather, timer
└── state.go             # generic thread-safe state store
```
