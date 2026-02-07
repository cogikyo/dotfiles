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

ewwd subscribe gpu audio brightness music  # stream events (for eww deflisten)
```

eww integration:

```yuck
(deflisten ewwd `ewwd subscribe gpu audio brightness music date weather`)
(label :text {ewwd?.audio?.sink?.volume ?: "?"})
```

### Actions

Triggered by eww button clicks and scroll events.

```bash
# Brightness
ewwd action brightness reset              # set to 100%
ewwd action brightness night              # set to 40%
ewwd action brightness adjust up          # increase by 10%

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
| brightness | sysfs               | screen brightness percentage              |
| music      | D-Bus (Spotify)     | playback status, track info, album art    |
| network    | /proc/net/dev       | upload/download speeds                    |
| date       | time                | time, date, clockface icons, weeks alive  |
| weather    | OpenWeatherMap API  | temperature, conditions, moon phase, wind |
| timer      | internal            | countdown timer and alarm                 |

Each provider implements the `providers.Provider` interface and runs in its own goroutine. Providers that support user interaction also implement `providers.ActionProvider`.

## Configuration

`~/.config/ewwd/config.yaml` — provider settings, API keys, poll intervals

## Structure

```
ewwd/
├── main.go              # CLI entry, command routing to daemon socket
├── daemon.go            # lifecycle, provider coordination, command handler
├── state.go             # generic thread-safe state store
├── config/              # YAML config loading
└── providers/           # gpu, audio, brightness, music, network, date, weather, timer
```
