# Editorial project

One project model owns what a video is while it is being made: the semantic plan, the concrete timeline, and the typed proposals that move between them.
Sibling concerns (writing, edit intelligence, composition, orchestration) reference this model; none defines its own competing notion of segments, timelines, or proposals.
This spec exists because three specs previously claimed editorial ownership and a take swap had no authoritative propagation path.

## Two artifacts, one identity space

- The plan is semantic: beats, narration, claims, segment records with stable identities, segment type from the genome taxonomy, visual intent, status.
- The timeline is concrete: selected media ranges, tracks, timing, transitions, scene references, markers.
- Stable segment identity is the join: a plan segment maps to timeline regions, takes, captions, chapters, and analytics windows.
- A segment revision or take swap propagates through identity, never through duplication across artifacts.

## Proposal lifecycle

- A proposal is a typed candidate change (cut set, take selection, overlay set, caption timing, genome mutation) with declared inputs and a revision identity.
- Any producer emits proposals: an agent stage, a human review note, an analytics join.
- Promotion is a decision made by a human or supervising agent; the orchestrator executes it mechanically and never defines a proposal's meaning.
- A proposal invalidates when any declared input changes: a new transcript revision, re-ingested media, a mutated genome.

## Asset ledger

- One ledger owns asset identity and provenance: capture checksums, generative prompts and invoices, license and attribution records, disclosure-relevant flags.
- Capture, generative, and edit stages populate it; publish derives disclosure and attribution from it; nothing else keeps a private provenance store.

## Caption ownership chain

- Timing truth lives in the corrected transcript; style tokens live in the genome; rendered presentation belongs to the renderer; the accessibility sidecar is a publish export.
- Four responsibilities joined by segment identity; there is no single overloaded caption artifact.

## Open questions

- Schema shape: OTIO-native timeline from day one versus a lean owned schema with OTIO export; settled by counting which gen 0–2 operations OTIO expresses natively versus which become adapter patches.
- How little plan structure video one can carry: minimal segment records first, the full claim apparatus only when the content demands it.
