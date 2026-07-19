# Composition engines

One engine layer turns editorial intent into rendered pixels; this maps the credible candidates and the architecture they imply.
The field converges on code as source of truth, headless renderers, and JSON at orchestration boundaries; opaque GUI project state and generic fluent FFmpeg wrappers are being abandoned.
The editorial project model owns what timelines mean; this concern owns how they lower to pixels.

## Two-layer shape

- The concrete timeline lowers through a bounded assembly IR to renderers; OpenTimelineIO is the reference data model and interchange escape hatch, never a renderer.
- Vocabulary: a segment is editorial (plan and timeline); a clip is rendered media that a segment references.
- Renderer-specific scene modules (React composition, manim scene, browser page, Blender script, terminal tape, simulation) own visual richness; the timeline references them as opaque clips with props and declared alpha mode.
- Assembly rendering, audio mux, and final encode belong to FFmpeg regardless of which scene engines win.

## Assembly IR v0 bounds

The owned IR earns "small" only by exclusion; v0 names its operations and refuses the rest:

- v0 carries: one video track, one overlay track, narration and music audio tracks, hard cuts, one crossfade type, markers, per-clip alpha declaration.
- v0 refuses: speed ramps, nested sequences, keyframed transforms, per-clip grading, multicam switching; wanting one is a signal to re-plan the video, not to grow the IR silently.
- Anything beyond v0 lives in a scene module or waits; a "small IR" without this fence is a multi-month editor engine in disguise.

## Conform contract

Hybrid media (23.98/25p camera, 30/60 screen, browser sRGB, camera Rec.709 limited range) is where first exports break, so one project-wide contract governs:

- One master frame rate per project, chosen at plan time; all sources conform at ingest, and VFR screen captures normalize there.
- Color: every source declares range and transfer; conversion happens at conform, never implicitly mid-filtergraph.
- Alpha: every scene clip declares straight or premultiplied; assembly refuses undeclared alpha.
- Audio: one loudness target and true-peak ceiling per project; narration is the master timing track.
- The genome's duration ladder pins to the master rate in frames.

## Option map

| Engine | Best at | Health signal | Agent fit | Trade-off |
| --- | --- | --- | --- | --- |
| Remotion | web motion graphics, captions, templates | very active, weekly releases | excellent; ships agent skills; JSON props | license verified free for solo creators (2026-07); Chromium render overhead; React code less diffable than data |
| Owned IR → FFmpeg | assembly cuts, overlays, subtitles, audio | FFmpeg eternal | perfect: owned diffable schema | complexity fenced only by the v0 bounds above |
| MLT / melt XML | multitrack NLE-grade timelines, headless | active (7.40, 2026-06) | XML directly generatable | niche knowledge; version portability untested |
| manim CE + voiceover plugin | mathematical visual poetry, narration-timed | active (0.20.x, plugin 2026-06) | Python scenes very LLM-friendly | slow renders; not general editing |
| Motion Canvas | narration-synced vector explainers | stale releases since 2024 | TS generator scenes readable | editor-centric authoring; weak footage assembly |
| Revideo | code-first 2D scenes, MIT | newer, active-looking | headless render API | small ecosystem, little production history |
| Editly | declarative JSON assembly cuts | prerelease cadence, risky deps | JSON5 spec directly generatable | maintenance concentration |
| Blender bpy | 3D and procedural scenes | active | scripts generatable | binary .blend state; own sibling concern |

Rejected without residue: moviepy/PyAV (imperative, slow pixel path) and GStreamer Editing Services (heavy GObject boundary from Go); nothing on the horizon revives them.

## Determinism contract

- Renderer versions, fonts, browser/Node/Python builds, FFmpeg build, color configuration, seeds, and input hashes are pinned per project and participate in cache identity.
- Visual determinism is the requirement; byte-identical encodes are not.
- VAAPI accelerates final encode; most filters stay CPU-bound. GPU-everywhere is a conjecture to falsify, never a design premise.

## Leading conjecture

- Hybrid: the bounded owned IR lowers to FFmpeg for assembly; a browser engine renders motion graphics; manim renders mathematical scenes.
- Remotion versus raw frame-step is the field's most reversible decision: spike the same 30-second genome segment both ways and pick; do not agonize.
- Cloud template services stay out: reproducibility and asset locality lose too much.

## Open questions

- Whether MLT earns the seat between owned IR and raw FFmpeg, or gets skipped entirely.
- Master frame rate for the first project.
