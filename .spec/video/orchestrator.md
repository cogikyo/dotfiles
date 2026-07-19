# Orchestrator

A Go package and CLI (name open) is the execution layer: it runs the build graph, stores and applies proposals, invokes adapters, and enforces guardrails.
It executes editorial meaning but does not define it; the editorial project model owns semantics, and the genome lint holds style law at the timeline boundary.

## Execution mechanics

- A video project is a directory of text artifacts plus immutable media references; editorial mutations are file diffs and Git owns text history.
- Media is content-addressed; derived artifacts declare their inputs, including adapter versions and environment fingerprints, so re-rendering from retained assets is always possible.
- Incremental caching (unchanged segments skip re-render) is a stage 2 payoff; stage 0 only records the hashes that make it possible later.
- Cloud acquisition passes an acceptance membrane: accepted outputs freeze as immutable sources with ledger provenance, and re-renders never resample providers.
- Cloud adapters carry dry-run modes and per-video spend caps.
- Irreversible or costly operations require explicit authority: paid generation, capture control, credentialed upload, privacy changes, and publication write idempotency records before execution.
- An external-action record exists before submission and carries immutable intent, bounded approval envelope, authorizer, cost/privacy limits, provider idempotency key, and target project, release, or capture session.
- External-action states are authorized, submitted, confirmed, and uncertain; retries reuse the same key, and uncertain actions require reconciliation before any new submission.
- Ambiguous provider status fails closed: the orchestrator records the state and asks for review instead of spending, publishing, or deleting.

## Capability boundary

Adapters are pinned-version subprocess boundaries declaring capabilities rather than a fixed topology, because the composition engine is still undecided:

| Capability | Examples |
| --- | --- |
| produce scene clip | browser frame-step, remotion, manim, blender, terminal renderer, simulation |
| assemble timeline | FFmpeg lowering, MLT lowering |
| measure media | ASR, scene detection, loudness analysis, sync correlation |
| transform audio | mastering chain, denoise pass |
| acquire asset | generative cloud calls |
| produce review projection | preview render, web preview, Blender VSE project |
| control capture | obs-websocket sessions |

## Agent surface

- CLI-first: every operation is a command with JSON in and out, so agents and humans share one interface and OpenCode needs no bespoke plugin to start.
- Proposals target exact base revisions per the editorial lifecycle; the orchestrator stores, lint-validates, applies promoted proposals, and invalidates them mechanically.
- Direct text edits are registered as editorial revisions per the editorial lifecycle.
- Every stage re-runs from declared inputs; the pipeline is resumable by construction.

## Review loop

- Stage 0's editor is text: the human edits the timeline artifact directly (nvim is the stage 0 NLE), and the loop is edit → preview re-render in under a minute → next note.
- Review notes reference timecodes and become typed proposals; promotion re-renders only affected spans.
- Review surfaces are disposable projections: mpv plus cue sheet first, a web preview as a strong candidate, Blender VSE optional.
- Annotations made in any projection round-trip as proposals, or that surface is disqualified; a review surface that swallows human judgment does not exist.

## Open questions

- Home: dotfiles command workspace versus standalone repo.
- Name.
- Buy-versus-build posture toward VibeFrame and OpenStoryline: cannibalize ideas only, or adopt pieces.
- Python sidecar policy for ML edges: pinned uv-run scripts per call, or a long-lived sidecar service.
