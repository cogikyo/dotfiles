# Composition engines

One engine layer turns editorial intent into rendered pixels; this maps the credible candidates and the architecture they imply.
Code remains the source of truth, renderers run headless, and opaque GUI project state is only a disposable review projection.
The editorial project model owns what timelines mean; this concern owns how they lower to pixels.

## Two-layer shape

- The concrete timeline lowers into a generated, non-editable assembly render plan; OpenTimelineIO is the reference data model and interchange escape hatch, never a renderer.
- Vocabulary: a segment is editorial (plan and timeline); a clip is rendered media that a segment references.
- Renderer-specific scene modules (React composition, manim scene, browser page, Blender script, terminal tape, simulation) own visual richness; the timeline references them as opaque clips with props and declared alpha mode.
- Assembly rendering, audio mux, and final encode belong to FFmpeg regardless of which scene engines win.

## Assembly IR v0 bounds

The owned IR earns "small" only by exclusion; v0 names its operations and refuses the rest:

- v0 carries: integer destination frame ranges, source frame ranges, project canvas, layer order, narration and music audio lanes, genome-approved visual cuts, audio-only crossfades, markers, caption projection references, and per-clip alpha declaration.
- Layer order is explicit and boring: base footage or full-canvas scene, full-canvas alpha overlays, captions, then encode metadata.
- Positioned overlays render upstream as full-canvas alpha clips; v0 has no transform language.
- The mutation cut lowers as a short full-frame transition clip generated from outgoing and incoming frames; it is never a general glitch effect.
- v0 refuses: speed ramps, nested sequences, keyframed transforms, per-clip grading, multicam switching; wanting one is a signal to re-plan the video, not to grow the IR silently.
- Anything beyond v0 lives in a scene module or waits; a "small IR" without this fence is a multi-month editor engine in disguise.

## Conform contract

Hybrid media (23.98/25p camera, 30/60 screen, browser sRGB, camera Rec.709 limited range) is where first exports break, so one project-wide contract governs:

- Conform is the admission membrane for acquired and rendered media candidates.
- Every assembly clip references a specific conformed artifact hash and frame space.
- Admitted clips already match project canvas, pixel aspect, master rate, color transfer/range, alpha convention, and audio specification.
- Assembly only composites, cuts, and muxes admitted clips; it never rescales, retimes, or converts color implicitly.
- One master frame rate per project, chosen at plan time; immutable originals stay untouched, and project-scoped conformed variants normalize VFR and mixed-rate sources there.
- Color: every source declares range and transfer; conversion happens at conform, never implicitly mid-filtergraph.
- Alpha: every scene clip declares straight or premultiplied; assembly refuses undeclared alpha.
- Audio: one sample rate, loudness target, and true-peak ceiling per project; narration is the master editorial timing track.
- Source-to-project frame mappings are artifacts, because transcript projection, sync, and VFR normalization all depend on them.

## Option map

| Engine | Best at | Health signal | Agent fit | Trade-off |
| --- | --- | --- | --- | --- |
| Remotion | web motion graphics, captions, templates | very active, weekly releases | excellent; ships agent skills; JSON props | license verified free for solo creators (2026-07); Chromium render overhead; React code less diffable than data |
| Owned IR → FFmpeg | assembly cuts, overlays, subtitles, audio | FFmpeg eternal | perfect: owned diffable schema | complexity fenced only by the v0 bounds above |
| MLT / melt XML | multitrack NLE-grade timelines, headless | active (7.40, 2026-06) | XML directly generatable | niche knowledge; version portability untested |
| manim CE + voiceover plugin | mathematical visual poetry, narration-timed | active (0.20.x, plugin 2026-06) | Python scenes very LLM-friendly | slow renders; not general editing |
| Motion Canvas | narration-synced vector explainers | stale releases since 2024 (checked 2026-07) | TS generator scenes readable | editor-centric authoring; weak footage assembly |
| Revideo | code-first 2D scenes, MIT | newer, active-looking (checked 2026-07) | headless render API | small ecosystem, little production history |
| Editly | declarative JSON assembly cuts | prerelease cadence, risky deps (checked 2026-07) | JSON5 spec directly generatable | maintenance concentration |
| Blender bpy | 3D and procedural scenes | active | scripts generatable | binary .blend state; own sibling concern |

Rejected without residue: moviepy/PyAV (imperative, slow pixel path) and GStreamer Editing Services (heavy GObject boundary from Go); nothing on the horizon revives them.

## Determinism contract

- Renderer versions, fonts, browser/Node/Python builds, FFmpeg build, color configuration, seeds, and input hashes are pinned per project and participate in cache identity.
- Visual determinism is the requirement; byte-identical encodes are not.
- If a render differs from an approved output beyond the drift threshold, it becomes a new candidate artifact requiring promotion instead of replacing the prior clip silently.
- VAAPI accelerates final encode; most filters stay CPU-bound. GPU-everywhere is a conjecture to falsify, never a design premise.

## Leading conjecture

- Hybrid: the bounded owned IR lowers to FFmpeg for assembly; a browser engine renders motion graphics; manim renders mathematical scenes.
- Remotion versus raw frame-step is the field's most reversible decision: spike the same 30-second genome segment both ways and pick; do not agonize.
- Cloud template services stay out: reproducibility and asset locality lose too much.

## Open questions

- Whether MLT earns the seat between owned IR and raw FFmpeg, or gets skipped entirely.
- Master frame rate for the first project.
