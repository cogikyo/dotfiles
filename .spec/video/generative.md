# Generative media

Cloud models generate novel pixels and audio; local deterministic tools do everything else.
The RX 5600 XT rules out serious local diffusion, so every generative avenue is priced as an API call with retries, and accepted-clip cost runs 2–10× nominal because selection dominates.

## Video generation

| Option | Role | Cost shape | Caveat |
| --- | --- | --- | --- |
| Veo 3.1 (Gemini API) | premium cinematic b-roll, native audio, reference conditioning | ~$0.15–0.40/s + retries | expensive sampling; shot generator, not continuity |
| Runway Gen-4.5 / Aleph | video-to-video edits, insert/change objects in existing shots | credit model | best control surface; variable raw quality |
| Kling | photoreal motion alternate sampler | credits | API/regional access is a moving target |
| Pika | fast meme mechanics, effects | ~$2–4 per usable clip | lower ceiling by design |
| Open weights (Wan, LTX, Hunyuan) | hosted/rented-GPU control freedom | GPU rental per clip | CUDA-shaped locally; cloud-class VRAM needs |

Sora is discontinued and is not built on.
The working pattern: generate 5–10 second isolated shots, retain prompt/seed/reference assets in the ledger, select one, cut around it; never plan narrative as one generation.
Accepted clips freeze as immutable sources per the orchestrator's acceptance membrane; re-renders never resample the provider.
Cinematic b-roll of real-world subjects conflicts with the genome's diegetic rule; using it is an explicit genome mutation, never a default, and the owned simulation substrate covers most early needs for free.

## Self-splice humor

- The credible route is a self-shot plate against clean background, composited or video-to-video edited into a scene; face-swap tools handle simple angles only.
- Wav2Lip-class open lip-sync is non-commercial licensed; commercial APIs exist and need verification before monetized use.
- Movie frames and soundtracks remain protected regardless of transformation; Content ID triggers on seconds of recognizable material and cannot adjudicate fair use.
- The safer meme pattern recreates the visual grammar with own plates, generated sets, or licensed material, and avoids original audio.
- Current resolution: this avenue is deferred; own-plate recreation is the only admissible form if it ever activates, and the H4 hypothesis stays dormant until then.
- YouTube requires disclosure for realistic synthetic scenes or people; cloning one's own voice for VO is explicitly exempt.

## Images

- Thumbnails resolve to templated typographic composites per the genome; generators supply at most one computed or stylized element inside that locked template.
- Style frames and reference packs: art-directable generators (Midjourney taste, FLUX API automatability) with a persistent style/character reference pack in the ledger.
- Stock-first hybrid (Pexels/Pixabay APIs) trades visual singularity for legal calm on real-world subjects.
- Local diffusion on this GPU is not a pipeline; cloud or nothing.

## Voice

- ElevenLabs-class professional clone of own voice covers pickups and corrections without re-recording; audition on names and technical vocabulary before trust.
- Kokoro-class local TTS covers scratch narration for timing at zero marginal cost.
- Only own-voice cloning; anything else needs documented consent.

## Music and SFX

- Suno-class subscription covers bespoke beds with commercial terms while subscribed; no official API exists, so generation stays a manual web step.
- Suno invoices/prompts/exports and royalty-free attribution records both land in the asset ledger; royalty-free libraries remain the boring safe default.
- Foley stays real per the genome.

## Local AMD-viable ML

- Real-ESRGAN and RIFE via ncnn/Vulkan run on this GPU without CUDA: upscaling, restoration, interpolation for low-fps generated clips.
- These are the highest-confidence local ML investments and get benchmarked first.

## Open questions

- Which single video-gen API earns the first integration, if any before the simulation substrate covers early needs; Veo for quality or Runway for control.
- Per-video cloud spend envelope; without one, retry economics quietly dominate the budget.
