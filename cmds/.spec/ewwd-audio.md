# Event-driven audio and headphone state

The ewwd audio surface reflects PipeWire and BlueZ state as events arrive, so volume controls remain smooth under rapid input and the AirPods Max status is compact, truthful, and immediately actionable.

## Audio authority

PipeWire and WirePlumber own default sink and source volume, mute, and identity.

The `audio` topic publishes independent availability for sink and source alongside each available device's raw percentage, display name, and mute state.
Unavailable devices never masquerade as zero-volume devices, and the widget gates every value on its corresponding availability field.

Volume is the raw percentage reported by WirePlumber.
The legacy source offset and dead zone do not exist.
Zero means zero for both source and sink, and either device may exceed unity only when its configured maximum permits it.

Sink and source scroll actions apply the same configured relative step through WirePlumber's native relative operation.
Each operation carries the target device's configured ceiling, performs no read-before-write, and lets the subsequent event establish displayed truth.
Rapid independent actions accumulate without predicting or publishing synthetic volume.
The action grammar consists of relative volume, toggle mute, and reset volume operations over `sink`, `source`, or the supported combined reset target.
An action addresses the current default device at execution time, rejects an unavailable target, and never mutates the published snapshot directly.

Mute actions change mute state rather than writing volume zero, so unmute restores the previous level.
The widget derives muted glyphs and tooltips from explicit mute state rather than inferring mute from volume.

The existing volume preset remains available through an action named for resetting volume rather than selecting a default device.
Its sink, source, and combined targets apply the configured preset percentages.

Device display names prefer the PipeWire node description, then nickname, then stable node name.
Configured aliases are keyed by stable node name and override that fallback chain.

## Audio event stream

A long-lived PulseAudio compatibility subscription reports sink, source, and server changes.
Relevant bursts debounce into one serialized refresh of both default devices, and only a changed snapshot is published.

The subscription process and every refresh inherit daemon cancellation and have bounded execution.
One refresh owner mutates the last snapshot, preventing subscription, recovery, and action feedback from racing each other.

Subscription exit moves unavailable devices to explicit unavailable state, reconnects with bounded backoff, and performs a full refresh after recovery.
An audio server restart, default-device replacement, or device disappearance therefore converges without restarting ewwd.

## Headphone authority

BlueZ owns connection state, device identity, and standard battery state for the tracked AirPods Max.
The existing hyprd Bluetooth device setting is the canonical address, and the shared configuration model exposes it to ewwd without another literal in ewwd configuration or the widget.

The `bluetooth` topic has this contract:

| Field | Meaning |
|---|---|
| `status` | One of `unknown`, `disconnected`, `connecting`, or `connected` |
| `name` | BlueZ device name when known |
| `battery_present` | Whether the percentage is authoritative |
| `battery_percent` | Aggregate headphone percentage, meaningful only when present |

`unknown` means BlueZ truth is temporarily unavailable.
`disconnected` means BlueZ authoritatively reports the tracked device disconnected.
Battery and name are cleared when their interfaces disappear, and stale values never survive a service restart or disconnect.

The provider installs signal matches before taking its initial managed-object snapshot.
It consumes device and battery property changes, object-manager interface additions and removals, and BlueZ bus-owner changes.
Address matching is case-insensitive.
When BlueZ reappears, the provider reinstalls its matches, rescans managed objects, and republishes recovered truth.
Signals and snapshots are serialized within the current BlueZ bus-owner generation, and work from a replaced owner is discarded.

The standard battery interface is capability-dependent and does not require BlueZ experimental mode on the installed BlueZ generation.
Connection-only operation is normal when neither the device nor a registered battery provider exposes that interface.

## Connection actions

The Bluetooth provider exposes toggle and reconnect actions through BlueZ D-Bus, so the widget never invokes a shell command or handles a hardware address.

Left click toggles the authoritative connection state.
Middle click performs an explicit reconnect for recovery from a degraded link.

Toggle disconnects when currently connected and requests connection otherwise.
Connect and reconnect publish `connecting` immediately, while disconnect remains connected until BlueZ confirms disconnection.

Only one connection operation is active at a time, and overlapping toggle or reconnect requests are rejected rather than interleaved.
The operation belongs to the current BlueZ owner generation and has a bounded deadline.
Its transient resolves on the next authoritative connection change, an explicit operation failure, or deadline expiry followed by rescan.
Replies and signals from replaced owners cannot resolve a newer operation.
The transient is never allowed to persist indefinitely.

## Headphone presentation

The headphone unit follows the existing compact-at-rest and detail-on-hover grammar.

| State | Resting presentation | Detail |
|---|---|---|
| Unknown | Dim unknown glyph and `––` | Tooltip says Bluetooth is unavailable |
| Disconnected | Dim disconnected Bluetooth glyph | Tooltip says AirPods Max is disconnected |
| Connecting | Orange connecting Bluetooth glyph and `––` | Tooltip says AirPods Max is connecting |
| Connected without battery | Unknown-battery glyph and `––` | Hover reveals the device name |
| Connected with battery | Battery-level glyph and permanent `NN%` | Hover reveals the device name; tooltip repeats exact state |

Known battery uses the installed Material Design Nerd Font battery ladder in ten-percent steps, including the `󰁽` family rather than a new progress ring.
The glyph and percentage share a battery-specific color ramp: healthy charge remains quiet blue, 20–39% becomes warm, and charge below 20% becomes ruby and uses the alert glyph.
Unknown battery is visually distinct from empty battery.

Disconnect collapses battery detail immediately.
Hover expansion uses the existing slide-and-fade motion, while glyph and color changes use the existing color transition without blinking or pulsing.

The old terminal launch tied to the removed mixer is deleted unless it has a deliberate replacement.
No unused click binding remains after the migration.

## Operational acceptance

- Rapid sink and source scrolling settles on the authoritative final value without delayed catch-up or offset jumps.
- External volume, mute, and default-device changes appear within a fraction of a second under normal load.
- Sink and source honor independent ceilings, represent zero honestly, and restore prior volume after unmute.
- Missing audio tools or server state renders unavailable rather than plausible zeroes and recovers without daemon restart.
- Audio actions reject unavailable targets and apply to whichever default device is authoritative when the action executes.
- A device appearing after ewwd startup is discovered without restarting ewwd.
- Bluetooth service restart clears stale battery, passes through unknown, re-enumerates the tracked device, and recovers connection state.
- Battery interface appearance, update, removal, and absence each produce the corresponding truthful widget state.
- Reconnect feedback appears immediately and always resolves to authoritative state or timeout recovery.
- Overlapping connection actions cannot create an invalid pending state or let an old BlueZ owner overwrite recovered truth.
- The resting connected state shows a Nerd Font battery level plus percentage, and low charge becomes progressively more salient.
- No tracked hardware address exists in ewwd configuration, widget markup, or action commands.
