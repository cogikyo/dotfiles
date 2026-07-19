# Writing and pre-production

Every video begins as text the pipeline can consume: research, claims, script, and shot manifest are repo artifacts long before any pixel exists.
No open standard joins essay scripts to shot manifests; a small owned schema over Markdown is the converged pragmatic answer.

## Script artifacts

- Authoring stays Markdown prose with per-segment structured records: identity, claim references, segment type (from the genome taxonomy), visual intent, and status.
- Beat sheet carries the argument; script carries spoken language; shot manifest carries screen language; together these three artifacts are the editorial plan, joined by stable segment identities.
- Two-column AV framing (narration beside visual intent) is the mental model even when the storage is records, not tables.
- Fountain and screenplay formats stay available as interchange, never as the source of truth.

## Research and claim hygiene

- Sources live as a local corpus with snapshots; every factual segment references a claim, and every claim carries its exact supporting passage, confidence, and caveat.
- Agents draft only from the evidence corpus; claims without primary support stay explicitly provisional.
- Citations behave like test fixtures: a factual video fails review when a claim lacks its source.
- The apparatus scales with the content: it activates fully when factual segments carry the video, while argument-led videos run a light claims ledger; the threshold is a per-video judgment, never dogma.

## Voice preservation

- A voice pack (approved past scripts, style guide, banned habits, rhetorical moves) grounds drafting agents; the creator's rewrite pass is where voice actually lives.
- Editorial feedback on drafts accumulates back into the voice pack, so imitation improves rather than fossilizing bad habits.

## Narration-first timing

- VO-first is the default shape for explainer-heavy videos: lock narration, derive word timings, cut visuals to its rhythm.
- TTS scratch narration exposes pacing and board density before real recording; synthetic cadence is a known bias on prose and gets rewritten by ear.
- Shoot-first conversational takes remain the shape for talking-head-led videos, with the transcript shaping the edit afterward.
- Still-frame animatics under scratch narration are the cheap previz: numbered boards from the shot manifest, rendered to a timed reel.

## Recording aids

- QPrompt is the native Linux prompter candidate; Wayland multi-monitor behavior gets verified before it is trusted mid-session.
- Near-lens bullet prompts and memorized short blocks are the low-gear alternatives; a beam-splitter rig is the only true eyeline fix and stays optional.
- Full prose reads suit precision segments; bullet improv suits personality segments; the script marks which mode each segment wants.

## Take discipline

- Every take opens with a spoken slate (video, segment, take, pickup) and one clap; this makes transcript search, sync, and later automated selection tractable.
- A take log keyed by segment identity records take number, script revision, error type, and human keeper flags; take selection itself belongs to the edit stage.

## Open questions

- Whether research tooling stays plain files or adopts a reference manager with an export boundary.
- Prompter versus bullet improv as the default talking-head mode; this is a performance-style decision, testable per video.
