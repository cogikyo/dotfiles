# LibrePods metadata service

LibrePods exposes its proprietary AirPods Max state through a stable user-session interface, and ewwd enriches the existing headphone unit with noise control, wear, and device-native battery data without weakening BlueZ-only operation.

## Runtime ownership

LibrePods is the sole authority for Apple AAP state and commands.
Its tray, settings UI, and metadata interface consume one shared runtime snapshot rather than maintaining parallel interpretations of device state.

BlueZ remains authoritative for whether the configured headphones are connected.
ewwd accepts LibrePods enrichment only while BlueZ reports that tracked device connected, and it clears enrichment immediately on disconnect or LibrePods loss.

LibrePods neither exports identity keys nor logs raw private protocol material.
The existing redaction of identity-key material remains an invariant of every interface and diagnostic path.
The normalized public device address identifies which managed device a snapshot describes, allowing ewwd to compare runtime identity with its canonical tracked address without duplicating configuration.

## Session interface

LibrePods owns a versioned session-bus service with an initial property snapshot and change signals.
The interface is useful in headless mode and does not depend on the tray or settings window being visible.
LibrePods owns the durable wire definition of this surface, and ewwd mirrors it in exactly one translation boundary rather than spreading bus details through its domain or widget code.

Every snapshot and change carries a device-session generation that changes whenever the LibrePods process owner or active AAP device session changes.
Interface version describes wire compatibility, while session generation describes the freshness of runtime state.

The snapshot represents capability separately from current value so unsupported, unknown, false, and off remain distinct.
It exposes these concepts when the connected model supports them:

| Concept | Values |
|---|---|
| Interface version | A monotonic compatibility version |
| Session generation | An opaque identity for the current service owner and AAP device session |
| Device identity | The normalized public address of the managed device |
| Device readiness | Initializing, ready, disconnected, or failed |
| Device-native battery | Aggregate percentage, validity, and charging status when available |
| Noise control | Off, noise cancellation, transparency, or adaptive |
| Wear state | Unknown, worn, or not worn |
| Conversation awareness | Supported and enabled state |
| Personalized volume | Supported and enabled state |
| High-resolution microphone | Supported and enabled state |

State changes emit only after the shared LibrePods snapshot changes.
A new client can obtain complete current truth without waiting for a future signal.
The initial snapshot and later changes belong to one session generation, so a client can discard a late snapshot or signal from an obsolete session.

The service exposes capability-checked commands for selecting and cycling noise-control modes.
Cycling traverses only modes supported by the connected headphones and never silently substitutes a different mode.
Commands report explicit rejection when the device, feature, or protocol session is unavailable.
Each command is correlated with the device-session generation and its requested target, and reported state from that same generation remains the final authority.

## ewwd merge

ewwd subscribes before reading the initial LibrePods snapshot, watches service ownership, and reconnects after service replacement.
An incompatible interface version is rejected explicitly and leaves the BlueZ-only headphone topic intact.
It accepts only the current unique bus sender, discards snapshots completed after an owner change, and serializes snapshot and signal application within the matching session generation.

The BlueZ connection state always gates enrichment.
The LibrePods device identity must match ewwd's canonical tracked address before any metadata or command result is accepted.
A live valid LibrePods aggregate battery takes precedence over BlueZ battery because it comes from the device protocol; current BlueZ battery is the fallback whenever LibrePods battery is absent.
Service loss therefore removes proprietary fields and falls back to BlueZ battery without fabricating a transient zero.

The enriched headphone topic adds optional noise-control, wear, conversation-awareness, personalized-volume, and high-resolution-microphone fields.
Absent fields remain absent rather than taking plausible defaults.

Noise-control actions pass through ewwd to LibrePods, preserving one widget command surface.
The widget does not call LibrePods directly and does not need to know which process supplied a displayed field.
The translation boundary validates wire version, sender, device identity, and session generation once before producing ewwd's internal headphone state.

## Metadata presentation

The connected resting unit remains the battery glyph and permanent percentage, so richer metadata does not add permanent bar clutter.

Hover reveals the device name followed by one compact noise-control chip: `ANC`, `Trans`, `Adapt`, or `Off`.
The chip uses the existing accent transition and appears only while its value is authoritative.

Normal worn state adds no visual noise.
Not-worn state adds a dim wear indicator on hover because it explains paused playback or an apparently idle connection.
Unknown wear state renders nothing.

Conversation awareness, personalized volume, and high-resolution microphone state live in the tooltip as diagnostic prose.
They enter the hover row only if later use shows that one of them deserves glanceable status.

Scrolling the headphone unit cycles supported noise-control modes.
The requested chip turns orange until LibrePods confirms the new mode or the request times out, then reconciles to reported truth.
One noise-control command is in flight at a time.
Rapid additional scrolls update the latest desired mode, and any superseded confirmation cannot clear or mislabel the newer pending target.
Pending work is discarded on session-generation change, disconnect, rejection, or bounded timeout.
Connection toggle and reconnect interactions remain unchanged.

## Failure containment

LibrePods startup, shutdown, restart, protocol failure, and incompatible version never remove BlueZ connection control or standard battery display.
Stale proprietary metadata disappears as soon as service ownership is lost or the device disconnects.

The session interface remains responsive while the settings window is open, closed, or minimized.
Slow device operations do not block property reads or unrelated state signals.

High-resolution microphone state is observable but not toggled from the bar.
Its existing audio-profile behavior remains LibrePods-owned and cannot silently rewrite ewwd's connection or volume model.

## Operational acceptance

- A fresh client receives complete current metadata before the next AirPods event.
- Noise-control, wear, battery, conversation-awareness, personalized-volume, and microphone changes emit one coherent update each.
- LibrePods restart clears enrichment, preserves BlueZ controls, reconnects the client, and repopulates current metadata.
- An incompatible interface version produces an explicit diagnostic and clean BlueZ-only behavior.
- A snapshot for any device other than the configured AirPods Max is rejected without disturbing BlueZ state.
- Device disconnect removes battery and proprietary state without leaving stale hover chips or tooltip values.
- Device-native battery replaces BlueZ battery only while valid and falls back cleanly when unavailable.
- Scroll cycles only supported noise-control modes and shows pending feedback until authoritative confirmation or timeout.
- Headless LibrePods publishes the same metadata and accepts the same commands as tray mode.
- Old service owners, device sessions, and command confirmations cannot overwrite newer enrichment or pending state.
- No identity key or raw private frame leaks through the session interface or ordinary logs.
