# Editorial project

One project model owns what a video is while it is being made: the semantic plan, the concrete timeline, and the typed proposals that move between them.
Sibling concerns (writing, edit intelligence, composition, orchestration) reference this model; none defines its own competing notion of segments, timelines, or proposals.

## Artifact graph

- The plan and timeline are authoritative editorial revisions.
- A project revision is an immutable manifest over exact plan hash, timeline hash, accepted asset references, conformed asset references, and genome version.
- Measurements are immutable adapter outputs; they seed proposals and never become editorial truth by themselves.
- Source assets are immutable accepted inputs registered through the ledger.
- Conformed assets, clips, render plans, previews, and exports are reproducible derivatives with declared inputs.
- A proposal is a candidate patch against one exact base revision and dependency set.
- A release pins one promoted project revision, render manifests, output hashes, captions, attribution, disclosure flags, and publish metadata.
- A publication is an external-effect record attached to a release.
- Analytics evidence is append-only observation that may seed future proposals and never mutates a release.

## Editorial artifacts

- The plan is semantic: beats, narration, claims, segment records with stable identities, segment type from the genome taxonomy, visual intent, status.
- The timeline is concrete: selected media ranges, tracks, timing, transitions, scene references, markers.
- Segment identity is semantic only; timeline regions, source ranges, capture takes, caption cues, chapter markers, and analytics windows have their own identities and reference segments explicitly.
- Splits and merges preserve lineage instead of pretending one identifier can mean every downstream artifact.
- A segment revision changes meaning; a take swap changes timeline selection.

## Proposal lifecycle

- A proposal is a typed candidate change (cut set, take selection, overlay set, caption projection, genome mutation request) with declared inputs, dependency set, and exact base revision.
- Any producer emits proposals: an agent stage, a human review note, an analytics join.
- Editorial authors may directly create a new authoritative revision during stage 0 text editing.
- Promotion is a decision made by a human or supervising agent; the orchestrator applies it mechanically only when the base revision still matches.
- Proposal states are draft, valid, invalidated, promoted, rejected, and failed-to-apply.
- A current revision change invalidates proposals whose declared dependencies no longer match: transcript alignment, source asset, conformed variant, timeline region, or genome version.
- Promotion records actor, reviewed base revision, resulting revision, and reason.

## Asset ledger

- One ledger owns asset identity and provenance: capture checksums, generative prompts and invoices, license and attribution records, disclosure-relevant flags.
- Capture, generative, and edit stages submit records to it; publish derives disclosure and attribution from it; nothing else keeps a private provenance store.
- Asset status is separate from immutability: acquired candidate, verified source, accepted editorial source, rejected, unavailable, derived artifact.
- The verified source → accepted editorial source transition is an explicit recorded decision with actor, candidate hash, reason, and target project revision.
- Adapters may acquire, probe, verify, and propose; they never self-accept an asset for editorial use.
- Accepted editorial sources are eligible for timeline reference; rejected candidates still retain provenance when legal, cost, or disclosure records matter.

## Caption ownership chain

- Source transcript truth lives in corrected, time-anchored transcript tokens keyed to conformed source media.
- The timeline owns program time; caption cues are a projection from retained source words through approved timeline regions into program frames.
- A correction preserves token identity only when the same token keeps the same conformed-source frame interval.
- Splits, merges, insertions, deletions, and interval changes create replacement timed-transcript revisions with successor mappings where applicable.
- Ordinary edit boundaries retain whole token intervals; intentional intra-token repairs explicitly suppress or split the caption projection.
- Style tokens live in the genome; rendered presentation belongs to the renderer; the accessibility sidecar is a publish export.
- Re-alignment failure blocks derived captions and cut proposals until a new corrected transcript revision exists.

## Open questions

- Schema shape: OTIO-native timeline from day one versus a lean owned schema with OTIO export; settled by counting which stage 0–2 operations OTIO expresses natively versus which become adapter patches.
- How little plan structure video one can carry: minimal segment records first, the full claim apparatus only when the content demands it.
