# Video pipeline mission

AI-orchestrated video production lives inside OpenCode: agents plan, edit, animate, and render while the human records, reviews, and decides taste.
The channel communicates cullyn's ideas (systems, memetics, entropy, epistemology) in a self-owned style built from zeroth principles, with no conventional GUI editor at the center of gravity.
The spec packet defines the artifact membranes and style laws that let implementation agents build one coherent pipeline instead of a pile of video scripts.

## Terminal state

- A video moves from idea to publishable file through inspectable text artifacts: script, shot manifest, transcript, cut proposals, timeline, render recipe.
- The script is the replicator: the video is its most expensive phenotype, and the same script may also ship as essay, audio, or explorable page.
- Every render re-renders from committed sources plus retained immutable assets; cloud generations are acquisitions frozen at acceptance, never re-derivable.
- Automation converges toward three human surfaces (performance, proposal review, taste); today the human also writes, lights, verifies ingest, corrects transcripts, and operates Studio, and the specs treat that honestly as the reduction target.

## Constraints

- Arch + Hyprland; capture must be Wayland-native.
- AMD RX 5600 XT: VAAPI h264/hevc encode, no hardware AV1, no credible local diffusion; Blender HIP Cycles works on RDNA1 but without ray-tracing acceleration or GPU denoise.
- Canon EOS M50 II + Sigma 16mm f/1.4; SM57 through Scarlett Solo; sensor limits and recording policy live in capture.
- Go is the preferred language for owned tooling; pinned Python sidecars are acceptable at ML edges.
- Personal channel: learning and expression outrank throughput; no commercialization requirement.

## Decision axes

| Axis | Poles | Current lean |
| --- | --- | --- |
| Composition engine | web/code renderer ↔ owned IR→FFmpeg ↔ Blender | hybrid; spike per composition |
| Review surface | text timeline + preview renders ↔ Blender VSE ↔ NLE import | text + preview at stage 0 |
| Pixel origin | captured ↔ programmatic/simulation ↔ generative cloud | simulation-first for non-camera pixels |
| Autonomy | propose-and-approve ↔ unattended | propose-and-approve first |

## Capability stages

Each stage has an exit criterion; a stage without one is decoration.

- stage −1 session conductor: scene arming, slates, take log via capture control. Exit: a recorded session lands ingested, verified, and named with zero manual file handling.
- stage 0 assisted assembly: automated rough cut, sync, captions, loudness; the human edits the timeline artifact as text and re-renders previews in under a minute. Exit: one published video.
- stage 1 proposed assembly: agent emits a full timeline proposal from script plus transcript; human review is note → proposal → re-render. Exit: human corrections per video fall below a threshold set from stage 0 experience.
- stage 2 generated segments: motion-graphic and simulation segments from script beats in genome style; incremental render caching earns its keep here, not earlier. Exit: a generated segment ships unretouched.
- stage 3 unattended draft: full draft with one review pass; entered only with correction data accumulated from stages 1–2.

## Walking skeleton conjecture

- The first video is camera-free and reflexive: voiceover, terminal demo, data-viz interlude, chapter and stinger cards, on a topic where the pipeline itself is the subject.
- This exercises writing → webrender → editing → publish end-to-end with zero new engine risk, cannot be topically wrong, and is itself the terminal-niche probe.
- The camera enters around video three, after a dedicated grade test settles the indigo-versus-skin-tone question.
- Named counterargument: if parasocial trust is the real growth thesis, the camera moves earlier and the grade test moves before video one.

## Open questions

- Budgets, which are genuine decisions and currently absent: tooling hours before video one, hours per published minute, cloud spend envelope per video.
- Multi-phenotype distribution now or after the first videos.
- Self-modifying channel: whether the analytics→genome-mutation loop gets built at video one or the channel stays styled-only until data justifies it.

On-device verification tasks (VAAPI benchmarks, camera soak, mic levels, portal behavior, upload privacy test) live in their owning specs.
