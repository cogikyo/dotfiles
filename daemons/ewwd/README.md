# ewwd

System utilities daemon for eww statusbar integration. Uses a provider-based architecture where each provider monitors a system resource and pushes updates to subscribers over a Unix socket.

## Commands

```bash
ewwd                     # start daemon (foreground)
ewwd status              # check if running
ewwd status --json       # full state dump
```

### Query and subscribe

```bash
ewwd query               # all state as JSON
ewwd query gpu           # specific provider

ewwd subscribe gpu audio music  # stream events (for eww deflisten)
```

eww integration:

```yuck
(deflisten ewwd `ewwd subscribe gpu audio music date weather`)
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
ewwd action music toggle                  # play/pause
ewwd action music next                    # next track
ewwd action music previous                # previous track
ewwd action music volume up [0.05]        # adjust volume
ewwd action music seek up                 # seek forward 10s

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
| gpu        | sysfs               | AMD GPU busy%, VRAM, memory clock         |
| audio      | PulseAudio          | sink/source volumes with offset           |
| music      | D-Bus (Spotify)     | playback status, track info, album art    |
| network    | /proc/net/dev       | upload/download speeds                    |
| date       | time                | time, date, clockface icons, weeks alive  |
| weather    | OpenWeatherMap API  | temperature, conditions, moon phase, wind |
| timer      | internal            | countdown timer and alarm                 |

Each provider implements the `providers.Provider` interface and runs in its own goroutine. Providers that support user interaction also implement `providers.ActionProvider`.

## Configuration

`../configs/ewwd.yaml` — provider settings, API keys, poll intervals

`ewwd` reads its config from the repo-level daemon config directory and uses that to initialize providers, set polling cadence, and wire provider-specific options.

## Structure

```
ewwd/
├── daemon.go            # lifecycle, provider coordination, command handler
├── main.go              # CLI entry, command routing to daemon socket
├── providers/           # gpu, audio, music, network, date, weather, timer
└── state.go             # generic thread-safe state store
```
