# Edit intelligence

The editing pipeline turns raw media into a reviewed timeline through inspectable stages: media → measurements → proposals → timeline → render.
Agents propose and humans (or supervising agents) promote; no stage holds opaque state.
The ecosystem is converging on exactly this shape: ASR word timings → structured edit proposals → deterministic renderers, with blind silence-cutting abandoned in favor of proposal-and-review.

## Transcription

| Tool | Strength | Caveat |
| --- | --- | --- |
| WhisperX | forced alignment, word timestamps, diarization, JSON out | alignment models per language; diarization imperfect |
| faster-whisper | mature CPU int8 baseline, easy serialization | GPU path is CUDA-shaped |
| whisper.cpp | CPU/Vulkan/ROCm local, native CLI | word timing marked experimental |

Raw ASR output is never the root: the corrected transcript (human or agent-audited, revision-identified) is, and captions, cut proposals, and b-roll search derive from it; revision changes flow through the editorial proposal lifecycle.

## Rough cuts

- auto-editor is the mature loudness/motion cutter with exports to editor formats; it detects signal conditions, never editorial quality.
- FFmpeg silence/scene primitives plus an owned cut-list layer is the lowest-dependency equivalent.
- Filler-word removal derives from transcript rules over word JSON; proposed remove-ranges merge and pad before becoming cuts.
- Authored stillness and protected regions from the plan are immune to silence proposals; the genome's payoff beats are content, not dead air.
- Best-take selection is rank-and-review: transcript-to-script match and audio-quality measures rank candidates, and the human keeper flag decides; fully automatic selection stays research residue.

## Structure and sync

- PySceneDetect and FFmpeg detectors (scene, black, freeze) provide shot measurements that feed proposals.
- Camera scratch audio aligns to the clean Scarlett track by waveform cross-correlation; drift on long takes is inspected, not assumed away.
- One clap slate per take remains the cheap insurance.

## Captions

- The corrected transcript renders through an authored ASS template (karaoke timing tags) and burns via libass, keeping caption style versioned with the genome.
- Styled caption quality is a template-authoring problem, not a tooling gap.

## Audio post

- Two-pass loudnorm with measured JSON gives reproducible loudness; a versioned starting chain (high-pass, EQ, compression, de-ess, limiter) suits the SM57 without pretending to fix room or placement.
- DeepFilterNet class denoise is an optional pass whose artifacts get auditioned, never trusted blind.
- Music ducking is sidechain compression keyed by the narration track.

## Semantic b-roll

- B-roll assets carry captions and embeddings; transcript windows retrieve candidates; the agent emits a proposed overlay list.
- Editorial appropriateness stays the weak link; proposals stay proposals.

## Agent surfaces to watch

| Project | Why it matters | Status |
| --- | --- | --- |
| Descript API/MCP | most mature product surface for transcript editing | cloud state; not an owned source of truth |
| VibeFrame | agent-harness contract: JSON, dry-run, cost caps | very new |
| FireRed OpenStoryline | local MCP, rough-cut skills, semantic media | new, model/API heavy |
| Video Jungle MCP | multimodal search with OTIO export | hosted vendor |

These are idea sources and comparison points; none is a substrate to build on yet.

## Open questions

- Where the human review gate lives per stage (cut proposals vs assembled timeline vs preview render), and how many gates gen 0 actually needs.
- Whether diarization matters at all for a single-voice channel or gets dropped from the pipeline.
