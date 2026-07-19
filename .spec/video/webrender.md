# Web-native segments

The models are strongest at HTML/CSS/SVG/canvas, so browser-rendered artifacts become first-class video segments.
Everything here renders frame-indexed and deterministic: real-time screen capture of a browser is never a production asset.

## Frame-step contract

- A page exposes a seek function: given t (or frame/fps), it renders that exact state; the renderer steps virtual time, screenshots lossless frames, and encodes via FFmpeg at a declared framerate.
- Playwright's clock API overrides timers, animation frames, and performance time, making this far less fragile than the older timecut monkey-patching lineage.
- CSS transitions/animations are the known determinism hazard; scenes derive state from injected time, and CSS animation is banned in scenes outright.
- Clock control stabilizes logic, not rasterization: fonts, compositing, and GPU output can still drift across Chromium upgrades; golden frames detect drift rather than create determinism, and lossless frame storage gets budgeted per segment.
- Reproducibility pins: Chromium revision, fonts, viewport and scale factor, locale, seeds, bundled assets.

## Engine choices

| Route | Role | Trade-off |
| --- | --- | --- |
| Remotion | productized frame-exact web rendering, parallel workers | composition discipline required; license verified free for solo creators |
| Raw Playwright frame-step | preserves arbitrary generated HTML/CSS/SVG as-is | owned capture contract to maintain |
| WebCodecs in-browser encode | canvas/WebGL scenes with exact timestamps | low-level; needs muxer; not a DOM solution |
| three.js manual time | deterministic 3D segments in browser | assets, shaders, seeds all pinned |
| Lottie / ThorVG | portable vector interchange, native raster | AE-shaped subset; poor target for arbitrary generated SVG |

## Terminal and code segments

- VHS renders scripted terminal demos from checked-in tapes with explicit geometry, theme, font, and framerate; it owns the terminal-demo segment type.
- VHS emits MP4/WebM/GIF/PNG frames with no documented alpha path, so terminal segments composite as opaque panels until a local test proves otherwise.
- asciinema plus agg captures semantic session streams for replay-derived renders when a real session matters.
- An owned cast-to-frames renderer through the channel theme is the compounding alternative: casts are structured text with timing, so a genome mutation can render new candidates for past terminal segments; a candidate once the niche proves out.
- Code walkthroughs (CodeAesthetic style) have no mature standalone tool, and the honest cost lives in the unowned layer: stable line identity across structural edits, token geometry under wrapping and ligatures, camera planning; this is a mini-engine, not a weekend script.
- Stage 0 ships scripted real-editor capture with post zooms; the owned animator exists only after a timed spike beats capture on hours per output minute.

## Slides and typeset stills

- Markdown/typst/slidev sources render to deterministic pages or stills; narration is the master timeline and visuals cut to it.
- Typst covers equations, diagrams, and title cards as crisp stills animated by the compositor.

## Compositing handoff

- Segments export with alpha (ProRes 4444 or VP9 with alpha) so they layer over camera footage downstream.
- A segment renderer emits a candidate described by data: identity, entry artifact, props, dimensions, fps, frame count, alpha mode, and target timeline slot; conform admits it before assembly references it.
- Audio is composed separately; browser segments render silent.

## Open questions

- Whether the code-walkthrough animator becomes an owned mini-engine early, given it is the channel's most distinctive segment type; the timed-spike gate above bounds it.
- How much of the vagari CSS/design system gets packaged as a reusable scene library for agents to compose from.
