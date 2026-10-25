# ewwd

System utilities daemon for eww statusbar integration. Uses a provider-based architecture where each provider monitors a system resource and pushes updates to subscribers over a Unix socket.

## Commands

```bash
ewwd                     # start daemon (foreground, auto-opens eww windows)
ewwd open                # reload eww config and reopen configured windows
ewwd status              # check if running
ewwd status --json       # full state dump
```

When opening, ewwd selects `windows.full` or `windows.compact` from `cmds/config/ewwd.yaml` using [hyprd's display selection](../hyprd/README.md#query-and-subscribe), falling back to `full` if hyprd is unavailable.
With `--no-open`, startup opens widgets only if hyprd reports `hidden=false`.
Hyprd requests `ewwd relayout` when the layout or monitor changes; it restarts eww only while widgets are open.
`ewwd close` restarts eww empty.

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
ewwd action audio toggle_mute <sink|source> # toggle mute
ewwd action audio cycle_device <sink|source>
ewwd action audio change_volume sink up   # adjust ±10
ewwd action audio reset_volume both       # reset preset volumes
ewwd action bluetooth toggle              # connect or disconnect headphones
ewwd action bluetooth reconnect           # explicitly reconnect headphones

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

# Weather
ewwd action weather refresh               # refetch unless fetched <10m ago
```

The speaker and microphone controls use left-click to toggle mute, middle-click to cycle devices, right-click to open `pulsemixer`, and scroll to adjust volume.
Cycling skips disconnected ports and output monitors, and keeps each device's volume and mute state.
When the tracked headphones' wear state changes, the default sink follows: putting them on selects their Bluetooth sink once it appears, and taking them off selects `audio.fallback_sink`.
Unknown wear state and disconnects leave the default sink alone, and cycling the sink by hand cancels a pending switch to the headphones.
In `pulsemixer`, F1 selects outputs and F2 selects inputs; both controls open its default output tab.

## Providers

| Provider  | Source             | Data                                       |
| --------- | ------------------ | ------------------------------------------ |
| audio     | WirePlumber        | default sink/source volume, mute, identity |
| bluetooth | BlueZ D-Bus        | tracked headphone connection and battery   |
| music     | Spotify            | playback, Canvas, history, queue           |
| network   | /proc/net/dev      | upload/download speeds                     |
| date      | time               | time, date, clockface icons, weeks alive   |
| clock     | time               | wall-aligned hour, minute, second          |
| computer  | procfs/sysfs       | RAM use, NVMe Composite temperature        |
| battery   | sysfs power_supply | charge, AC state, time to empty or full    |
| cycle-5   | time               | wall-aligned scalar display cycle          |
| cycle-6   | time               | wall-aligned scalar display cycle          |
| weather   | OpenWeatherMap API | temperature, conditions, moon phase, wind  |
| timer     | internal           | countdown timer and alarm                  |

Each provider implements the `providers.Provider` interface and runs in its own goroutine. Providers that support user interaction also implement `providers.ActionProvider`.

### Music

Playback state comes from Spotify through playerctl; a hidden Spotify Connect observer supplies the active device's history and queue without appearing as a player.
Canvas and Connect access tokens use `sp_dc`, and Connect client tokens also need `sp_t` from Firefox; log into `open.spotify.com` in Firefox to refresh the cookies.

- `canvas_path`: local MP4 path for the current track's Canvas, or an empty string when unavailable.
- `history`: up to five previous tracks, newest first.
- `queue`: up to five upcoming tracks in playback order.

Each history or queue entry has `title`, `artist`, and `art_url` fields.
Canvas frames publish on a separate `canvas` topic as `{"frame": "<path>"}`, with an empty path when no Canvas plays, so frame ticks never republish `music`.

### Weather

The location comes from the public IP through ip-api.com.
How often ewwd looks it up depends on `form` in `/etc/dctl/machine`.
A desktop, or a machine with a missing or invalid record, looks it up once at startup and falls back to `LAT LON` in `~/.local/.location`.

On a laptop, each poll and each `refresh` looks up the location before fetching.
While a NetworkManager VPN connection is active, the lookup is skipped because it would return the VPN server's city.
Each successful lookup is saved to `~/.cache/ewwd/location`.
A skipped or failed lookup keeps the last known location; with none yet, as after a restart, ewwd reads `LAT LON` from `~/.cache/ewwd/location`, then from `~/.local/.location`.
`refresh` runs in the background and does nothing while another fetch runs or when the last successful fetch is under 10 minutes old.

## Configuration

`../config/ewwd.yaml` contains provider settings, API keys, and poll intervals.

`ewwd` also reads the canonical tracked Bluetooth address from `hyprd.yaml`.

## Structure

```
ewwd/
├── daemon.go            # lifecycle, provider coordination, command handler
├── main.go              # CLI entry, command routing to daemon socket
├── providers/           # audio, clock, computer, cycles, music, and other state sources
└── state.go             # generic thread-safe state store
```
