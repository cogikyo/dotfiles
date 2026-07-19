# AirPods voice qualification

A private, repeatable listening and measurement harness selects the least destructive AirPods voice suppression and level settings that remove hum and keyboard noise while preserving whispered speech.

## Private corpus

The corpus lives in private durable user state outside the repository and temporary directories.
It is never uploaded, committed, or consumed by a remote speech service.

The raw source remains untouched throughout capture.
Every take records source identity, sample format, gain state, processing state, timestamp, content label, and a content hash in a manifest.

The corpus contains:

| Material | Contract |
|---|---|
| Room tone | At least 60 seconds with no speech or deliberate movement |
| Hum | At least 30 seconds under the condition where the user hears the steady tone |
| Keyboard | At least 30 seconds of representative typing and mouse use without speech |
| Speech over keyboard | At least 30 seconds of soft and normal speech during representative typing |
| Whisper, soft, normal, projected | Three independent takes per register from a fixed sentence set |
| Duplex | Familiar A2DP playback during whisper, normal speech, and silence |

Short Dunst prompts disappear before each take starts, and each material class records independently so notification timing cannot corrupt labels.
The user explicitly agrees before every recording session.

Spoken takes split into a two-thirds tuning set and untouched one-third confirmation set before processing or listening.
The confirmation set receives one evaluation pass and never feeds parameter changes.

## Measurement boundary

Suppressor measurements compare each candidate directly with attenuation-zero bypass before downstream gain or limiting.
The same raw samples and sample alignment feed every candidate.

Output-level, clipping, and latency measurements occur after the complete chain.
This separation prevents gain from masquerading as suppression.

Local transcription uses one pinned `small.en` Whisper model revision in CPU integer mode.
The manifest records model identity, decoder settings, normalized reference text, average per-word probability, and word error rate.

Spectral analysis identifies the microphone's useful bandwidth and whether the reported hum is a narrow stable line, harmonics, or broadband noise.
A narrow notch enters before suppression only for a repeatable tonal peak that survives the high-pass stage.
The notch remains narrow enough that gain-matched speech is not audibly colored.

## Suppressor selection

DeepFilterNet candidates use attenuation-zero bypass, `12 dB`, `18 dB`, and `24 dB`.
Every other plugin control remains at the pinned release default.
Unlimited attenuation is forbidden.

The lowest attenuation passing every requirement on the tuning set wins.
That winner receives one evaluation on the confirmation set.
Failure on confirmation leaves the processor unqualified rather than reopening tuning against the holdout.

| Property | Acceptance |
|---|---|
| Whisper energy | No more than `2 dB` loss against bypass |
| Whisper recognition | Average word confidence drops by at most `0.03`, and word error rises by at most five percentage points |
| Normal-speech transparency | RMS differs by at most `1 dB` against bypass |
| Room-noise reduction | Mean energy falls by at least `10 dB` |
| Keyboard-only reduction | Mean energy falls by at least `10 dB`, and peak energy falls by at least `8 dB` |
| Speech over keyboard | Recognition stays within the whisper limits while blind comparison clearly favors the processed noise balance |
| Clean speech | Processed speech is not judged less natural in more than three of twelve blind comparisons |
| Noisy speech | Processed speech is preferred in at least eight of twelve blind comparisons |

If no DeepFilterNet attenuation passes, no candidate becomes qualified.
The stable source remains available for diagnosis but does not become the default.
A fresh successor concern may evaluate RNNoise or another single suppressor against the same contract.

The evaluator never stacks suppressors or adds a gate, adaptive gain, acoustic echo cancellation, or automatic room-dependent tuning to rescue a failed candidate.
Preserving whisper outranks eliminating every keyboard transient.

## Level selection

Fixed makeup gain derives from the reacquired projected register rather than historical temporary captures.
The selected gain places the hottest projected transient at or below `-4.5 dBFS` before the limiter.
The limiter ceiling remains `-3 dBFS` and reduces the worst accepted transient by no more than `1.5 dB`.

A compressor enters only when the fixed-gain result leaves whisper mean level below `-34 dBFS` or the user judges whisper loudness inadequate in at least eight of twelve blind trials.

The compressor uses a ratio at or below `2:1`, an attack between `5` and `10 ms`, a release between `150` and `200 ms`, and makeup gain at or below `4 dB`.
It receives one bounded tuning pass and must preserve every suppressor requirement on both corpus partitions.

Presence EQ remains absent unless a blind comparison identifies a repeatable intelligibility defect after suppression and level qualification.
Subjective preference without a named defect does not add an EQ stage.

## Runtime qualification

The complete winner adds no more than `60 ms` between simultaneous raw and processed taps.
It produces no xruns during a 15-minute duplex soak at the configured graph quantum.

DSP cost is sampled from the named filter nodes' `B/Q` values under the same idle and duplex workloads.
The 95th-percentile named-node load remains below `20%` over the soak rather than using whole-graph load as the denominator.

Active capture preserves the same A2DP/AAC card profile and codec before, during, and after use.
Familiar music remains subjectively unchanged, and microphone activation causes no playback pause.

Qualification freezes the attenuation, notch decision, level gain, limiter, and optional compressor after one winner passes the tuning set, confirmation set, duplex soak, and blind listening requirements.
Further changes require a call-participant complaint, a source/package revision, or a measured runtime failure.
