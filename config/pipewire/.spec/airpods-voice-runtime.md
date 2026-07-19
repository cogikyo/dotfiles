# AirPods voice runtime

The audio session publishes one stable `AirPods Voice` source backed by a pinned DeepFilterNet processor, while preserving raw capture and uninterrupted A2DP/AAC playback through AirPods Max 2 connection changes.

## Transport integrity

LibrePods keeps high-resolution capture enabled and its software gain control disabled.
Capture starts and stops without setting the Bluetooth card profile to `off`, pausing media, selecting HFP, or changing the active A2DP/AAC codec.

The transport-reset setting remains disabled.
A capture-cycle preflight proves that the proprietary microphone uplink remains reliable without resetting A2DP.
Failure of that preflight requires repair in the LibrePods capture lifecycle rather than weakening the playback invariant.

The pinned LibrePods package recipe, patch source set, expected package release, and installer health check remain internally consistent before the voice processor enters the packages step.
Every package input required by the recipe is copied into its isolated build directory.

## Current transport evidence

On 2026-07-19, the repository template and private runtime settings disabled `a2dp_reset`.
Four one-second `AirPodsHiRes` captures produced valid non-silent 64 kHz mono PCM while the Bluetooth profile and codec remained A2DP/AAC before, during, and after capture.
A separate user-observed two-second capture under continuous music produced no audible pause, dropout, or stutter at either capture edge.
Disconnect and reconnect restored `AirPodsHiRes` under a new object identifier and a retry captured successfully without resetting A2DP.
The first reconnect attempt exceeded the paging timeout, so the two-second recovery target and ten-cycle reliability requirement remain unqualified.
The installed LibrePods package release still differs from the current recipe, so complete package health remains pending.

## Suppressor package

The packages step owns an x86-64 package for the DeepFilterNet `v0.5.6` GNU/Linux LADSPA release artifact.
The package verifies a repository-recorded SHA-256 digest, installs the self-contained plugin under its canonical library name, and installs license material from the matching source tag.

The package exposes the `deep_filter_mono` label and embedded low-latency model without downloading models or code at runtime.
The plugin accepts 48 kHz mono audio and reports its processing latency to the host.

Current AUR wrappers do not own this dependency because their sources, versions, checksums, or generated configurations are not sufficiently reproducible.
The runtime does not install RNNoise, EasyEffects, NoiseTorch, WebRTC APM, or generic ONNX hosting as fallback layers.

Package health verifies the exact package release, artifact digest, plugin discovery, label, architecture, license, linked libraries, and successful loading by a disposable filter host.
An incompatible or missing plugin fails explicitly.

## Stable source

A dedicated supervised filter-chain client publishes `AirPods Voice` from the PipeWire user session.
The client starts after PipeWire, shares PipeWire's lifecycle, and restarts on process failure.

The processed source remains visible under one stable node name while the filter client and PipeWire server live, including when `AirPodsHiRes` is absent.
PipeWire-server or filter-client restart may change object serials but preserves the name applications select.

The capture stream targets only the unique `AirPodsHiRes` node name.
It forbids fallback, lingers while the target is missing, remains passive while idle, and allows WirePlumber to reconnect it when that named source returns.
Numeric object identifiers never carry identity.

Missing raw capture produces silence.
It never links a laptop, webcam, Scarlett, monitor, or unrelated Bluetooth source into the trusted processed source.

Bluetooth disconnect removes only the ephemeral raw-to-filter link.
Reconnect creates exactly one new link without creating another processed source or restarting the filter client.
Normal idle suspension remains enabled because suspension does not destroy the node or prevent relinking.

## Graph shape

The graph converts the 64 kHz mono raw stream once to the session's 48 kHz mono format.
Playback never enters this graph.

The graph contains these ordered concepts:

1. A high-pass stage near `75 Hz`.
2. One `deep_filter_mono` suppressor with a provisional `12 dB` attenuation limit.
3. A deterministic level stage whose provisional gain preserves at least `3 dB` projected-speech headroom.
4. A final limiter at `-3 dBFS`.

Every DeepFilterNet control other than attenuation remains at its pinned release default until corpus qualification changes it explicitly.

The graph contains no acoustic echo canceller, gate, adaptive gain, voice-triggered mute, presence EQ, compressor, or second suppressor.
A narrow hum notch and a gentle compressor are supported by the graph boundary only after qualification produces evidence for them.

Failure to load any required stage fails the processed source visibly rather than publishing raw audio under the `AirPods Voice` name.
The raw `AirPodsHiRes` source remains independently selectable for diagnosis and comparison.

## Installation ownership

The packages step installs the pinned suppressor package and its declared host dependencies.
The system step installs and enables the supervised filter client and declarative graph.

The runtime does not set the global default source or change application configuration.
Qualification and rollout remain independent concerns, so installing the runtime cannot silently redirect a microphone.

Runtime health verifies service state, stable source name, mono 48 kHz format, provisional attenuation, reported latency, missing-target silence, and the absence of fallback links.
Diagnostics expose service health, raw-link state, selected attenuation, format, latency, DSP load, and xrun count without logging audio or Apple identity material.

## Operational acceptance

- Starting with the headphones absent publishes one silent `AirPods Voice` source with no capture link.
- Connecting the headphones creates exactly one link from `AirPodsHiRes` and makes capture usable without manual intervention.
- Disconnecting removes the link while preserving the processed source node and service.
- Ten disconnect and reconnect cycles preserve one source, resume capture within two seconds, and require no restart.
- Waiting beyond the idle suspend timeout does not prevent capture or later relinking.
- Active capture leaves playback on A2DP/AAC and causes no audible pause at capture start or stop.
- The graph adds no more than `60 ms` between simultaneous raw and processed taps.
- A 15-minute duplex soak produces no xruns at the configured graph quantum.
- Filter-client failure is visible, restarts under supervision, and never routes an unintended microphone into the processed name.
