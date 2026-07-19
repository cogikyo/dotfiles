# Orchestrator

A Go package and CLI (name open) is the execution layer: it runs the build graph, stores and executes proposals, invokes adapters, and enforces guardrails.
It executes editorial meaning but does not define it; the editorial project model owns semantics, and the genome lint holds style law at the timeline boundary.

## Execution mechanics

- A video project is a directory of text artifacts plus immutable media; every mutation is a file diff and Git owns history.
- Media is content-addressed; derived artifacts declare their inputs, including adapter versions and environment fingerprints, so re-rendering from retained assets is always possible.
- Incremental caching (unchanged segments skip re-render) is a gen 2 payoff; gen 0 only records the hashes that make it possible later.
- Cloud acquisition passes an acceptance membrane: accepted outputs freeze as immutable sources with ledger provenance, and re-renders never resample providers.
- Cloud adapters carry dry-run modes and per-video spend caps.

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
- Proposals precede mutations per the editorial lifecycle; the orchestrator stores, lint-validates, applies promoted proposals, and invalidates them mechanically.
- Every stage re-runs from declared inputs; the pipeline is resumable by construction.

## Review loop

- Gen 0's editor is text: the human edits the timeline artifact directly (nvim is the gen 0 NLE), and the loop is edit → preview re-render in under a minute → next note.
- Review notes reference timecodes and become typed proposals; promotion re-renders only affected spans.
- Review surfaces are disposable projections: mpv plus cue sheet first, a web preview as a strong candidate (the newtab pattern already proves the shape), Blender VSE optional.
- Annotations made in any projection round-trip as proposals, or that surface is disqualified; a review surface that swallows human judgment does not exist.

## Open questions

- Home: dotfiles command workspace versus standalone repo.
- Name.
- Buy-versus-build posture toward VibeFrame and OpenStoryline: cannibalize ideas only, or adopt pieces.
- Python sidecar policy for ML edges: pinned uv-run scripts per call, or a long-lived sidecar service.
