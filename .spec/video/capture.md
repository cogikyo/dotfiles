# Capture and ingest

Recording produces immutable, well-named, checksummed source media with clean audio and predictable sync; everything downstream depends on this stage being boring.

## Camera A-roll

- Internal SD recording is the primary path: best quality, fewest Linux variables, at the cost of offload and sync discipline.
- The M50 II's 4K is 23.98/25p only, heavily cropped (16mm becomes ~41mm-equivalent), and loses Dual Pixel AF; 1080p is the practical talking-head mode with the Sigma wide open around f/2–2.8.
- Manual exposure, shutter near 1/50–1/60, fixed white balance; face/eye AF trusted only after a real test.
- Clean HDMI through a UVC capture card is the live path for one-button OBS sessions; it requires dummy-battery power, info-display off, and a 60–90 minute thermal soak test before trust.
- Cheap generic UVC dongles are a hardware lottery; a Linux-explicit card (Magewell class) is the low-risk buy if the live path earns its keep.
- gphoto2 tethering suits stills and control experiments, never production video.

## Screen capture

- OBS with PipeWire portal capture is the session control plane: scenes per content type (coding, terminal demo, face, screen+face), camera as V4L2 source, isolated audio tracks.
- wl-screenrec covers lean high-fps single-output capture with DMA-BUF and VAAPI when composition is unnecessary; it fails on transformed monitors with Radeon VAAPI, a live constraint on this GPU.
- Hyprland gotchas: exactly one portal backend owns screencast; monitor bit depth must match; fractional scaling changes captured dimensions; HDR capture stays off, SDR/Rec.709 first.
- Code content captures native resolution at 30fps unless motion demands more; 4K60 is tested as a full chain before being promised.

## Audio

- SM57 close-miked at 5–10cm with pop filter, null aimed at the machine; placement and room beat gear purchases.
- Scarlett Solo records 48kHz/24-bit as an isolated track; its 56dB gain gets a real level test before any inline-preamp purchase.
- Redundancy is a second recorder path plus camera scratch audio, one clap slate per take; 32-bit float cannot recover analog clipping and is not a strategy.

## Encoding and intermediates

- VAAPI h264/hevc for screen-capture acquisition; RDNA1 has no AV1 encode, so AV1 delivery is an offline SVT-AV1 job benchmarked before adoption.
- Camera originals stay immutable; ingest creates proxies for review and composition later creates project-scoped conformed variants for edit/render use.
- Mezzanine only where an editor demands it; screen masters usually stay h264/hevc.

## Session orchestration

- obs-websocket (bundled since OBS 28) exposes scene switching and record control; a Go client drives recording sessions as part of the orchestrator.
- The fallback shape is independent CLI recorders plus the camera SD, trading convenience for failure isolation.

## Ingest hygiene

- One project layout distinguishes originals, proxies, conformed variants, edit artifacts, exports, and notes; originals never move or mutate.
- Offload renames to date-slug-source-index, records checksums in the asset ledger, and verifies; the card is freed only after two verified copies exist on independent storage.
- ffprobe validation at ingest catches codec/framerate/channel surprises when they are cheap.

## Machine verification shortlist

- vainfo plus VAAPI encode benchmarks at target resolutions on this exact Mesa/FFmpeg stack.
- M50 II on-device: clean-feed HDMI behavior, internal-recording duration and file-splitting limits, dummy-battery power, thermal soak.
- SM57 spoken-level measurement at normal distance before buying gain.

## Open questions

- Whether the live HDMI/OBS path is worth a capture-card purchase now, or SD-first delays it until multi-source sessions demand one-button capture.
- Recording resolution policy: 1080p everywhere for pipeline speed, or 4K screen masters for punch-in freedom.
