# AirPods voice rollout

Applications consistently use the qualified AirPods voice source without duplicate processing, while existing Scarlett recording behavior and explicit raw-source access remain intact.

## Default policy

`AirPods Voice` becomes the static user default only after its processor has a frozen qualified configuration.
No Bluetooth event hook, polling loop, per-session mode, or automatic source switch rewrites that default.

The stable source stays present and silent when the headphones are absent.
Using Scarlett, a laptop microphone, or raw `AirPodsHiRes` remains an explicit user choice.

Applications select the stable node name rather than a PipeWire object serial.
Applications pinned directly to `AirPods Voice` retain that identity across Bluetooth reconnects and ordinary service restarts.

The system does not claim fail-silent privacy during the brief interval where the filter client or PipeWire server itself is absent.
WirePlumber may choose another available default during that interval, so service health is visible and relevant applications prefer the named source over an abstract default.

## Application matrix

Each microphone application receives one explicit disposition:

| Disposition | Meaning |
|---|---|
| Processed | The application uses `AirPods Voice` with its own suppression, automatic gain, and echo cancellation disabled or minimized |
| Compatible | Unavoidable application processing remains enabled and passes the double-processing checks |
| Raw-owned | The application uses `AirPodsHiRes` because its unavoidable processor performs better as the sole suppressor |
| Unsupported | Neither source meets intelligibility and artifact requirements, so the application is excluded from the quality guarantee |

Zoom, Slack, the primary browser call path, and the user's ordinary recording path each receive a disposition before rollout completes.
No per-application daemon or automatic source-switching rule implements these choices.

Applications with unavoidable processing compare an application recording against a simultaneous direct processed-source capture.
A transcription-confidence drop greater than `0.03`, pause-floor change greater than `4 dB`, audible pumping, clipped whispers, or consistently worse blind preference fails the processed disposition.

Failure selects the raw-owned disposition only when the application's processing passes the same intelligibility and listening checks on raw input.
Failure of both paths leaves the application unsupported rather than adding another system suppressor.

## Existing recording ownership

OBS keeps its explicit Scarlett source and all Scarlett-only filters unchanged.
The system never replaces Scarlett with an AirPods source in existing scenes.

A future OBS source using `AirPods Voice` omits duplicate noise suppression and gating.
A future OBS source using raw `AirPodsHiRes` owns its entire processing chain explicitly and does not alter the system default processor.

## User feedback

The selected microphone name, processor health, and raw-link availability remain inspectable through standard audio controls.
An absent AirPods link appears as an idle trusted source rather than fabricated audio or an unnamed fallback.

The interface does not expose suppressor-strength scrolling, automatic room modes, or live gain knobs.
The frozen configuration changes through deliberate qualification rather than incidental UI state.

## Operational acceptance

- Zoom, Slack, the primary browser call path, and the ordinary recording path use their documented disposition after relaunch.
- Whisper, normal speech, silence, typing, and speech-over-typing remain within qualification thresholds in every processed or compatible application.
- A raw-owned application performs better with its own processor than the system-processed input in blind comparison.
- Bluetooth disconnect and reconnect do not invalidate applications pinned to the stable source name.
- Default playback remains A2DP/AAC throughout calls and recordings.
- Existing Scarlett OBS scenes produce unchanged audio and retain their existing source identifiers and filters.
- Processor or PipeWire failure is visible and never masquerades raw input as `AirPods Voice`.
