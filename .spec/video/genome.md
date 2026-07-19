# Channel genome

The channel is a vagari phenotype: a separately versioned genome (palette semantics, motion grammar, typographic voice, sonic motif) with segment types as freely varying phenotypes.
Mixed presentation modes are the thesis, memetic variation under selection pressure demonstrated on screen, so mode mixing must read as one voice rather than mush.
The genome is enforceable data, not vibes: every quantitative rule below resolves to pinned values in one token artifact, and a lint stage rejects violations at the timeline boundary.
Every project revision pins exactly one immutable genome version before lint or render.
Genome mutation requests name their exact parent version; promotion creates a child channel version and never changes an existing project or release.
Adopting a new genome version is an explicit project-revision change that invalidates dependent lint and render proposals.
Past-segment restyling produces a new candidate and release; historical releases do not change.
The release registry assigns `gen nnn` from one append-only channel counter alongside the current genome head.

## Segment taxonomy

| Type | Visual language it demands | Reference |
| --- | --- | --- |
| Talking head | motivated warm key against cool indigo room; shadows grade toward palette bg | Veritasium |
| Animated explainer | objects persist and transform; ≤6 per scene; constant-rate or manim-style rate functions, never springs | 3Blue1Brown |
| Code walkthrough | the real editor; deliberate cursor as narrator; one annotation style | CodeAesthetic |
| Terminal demo | scripted keystrokes; trimmed prompt; ≥1s held frame before the payoff | open niche |
| Screen session | honest capture; chapters; dead-ends edited for time, never erased | — |
| Meme insert | 1–3s; hard cut in and out; audio ducked; ≤1 per act | Fireship |
| Physical board | overhead or 45° rig; warm practical light; close tactile foley | Veritasium props |
| Data-viz interlude | one series, one accent; draw-on synced to narration | 3B1B-adjacent |
| 3D scene | slow dolly; matte materials plus one emissive; single light story | — |
| Stinger card | identical every time; sameness is the luxury | own invention |

## Amateur tells

| Type | Tells |
| --- | --- |
| Talking head | mixed color temperature in frame [review]; no rim separation [review]; clipped room tone [review] |
| Animated explainer | more than one easing idiom [lint]; motion during protected stillness [lint]; >6 active objects [lint] |
| Code walkthrough | cursor movement without narration [review]; code too small at feed size [review]; multiple annotation styles [lint] |
| Terminal demo | prompt noise [lint]; uniform robotic keystroke timing [review]; no held payoff frame [lint] |
| Screen session | dead-end compression without time-card [review]; unreadable UI scale [review]; cursor wander [review] |
| Meme insert | second insert in one act [lint]; unlicensed recognizable audio [review]; borrowed timing cadence [review] |
| Physical board | glare on writing [review]; hands obscure claim [review]; missing tactile foley [review] |
| Data-viz interlude | more than one accent series [lint]; unlabeled axis [review]; motion not coupled to narration [review] |
| 3D scene | unmotivated camera orbit [review]; more than one light story [review]; glossy default material [review] |
| Stinger card | template hash drift [lint]; motif deviation [review]; duration off ladder [lint] |

## Palette semantics

Accents carry Popperian meaning, held constant across footage grade, motion, plots, captions, and thumbnails:

| Semantic | vagari ramp | Meaning |
| --- | --- | --- |
| refutation | rby | the idea that dies here |
| conjecture | sun | the what-if |
| survivor | emr | earned, used sparingly |
| mutation | cyan | variation, noise, randomness |
| field | fg over drk | epistemic background, uncommitted |

A viewer learns within a few videos that red means "this dies here"; the epistemology is rendered as color.

## Token artifact

One versioned token file feeds every consumer; it carries more than hex:

- One authoritative vagari token source resolves ramp aliases once; scene code sees semantic tokens, never `cy2`/`cyn`/`cyan`-class implementation names.
- Context-keyed resolution: semantic plus surface (data-viz accent, caption highlight, thumbnail at feed size, scene on dark field) yields one exact ramp step, pre-verified against a contrast floor.
- Color-space and transfer declaration (Rec.709 versus sRGB) and grade anchors: shadow-lift target, highlight desaturation, and a skin-hue protection wedge, because a LUT is a transform and hex values alone cannot define it.
- Per-consumer derivation recipes: ASS caption color form, plot theme values, thumbnail template values.
- A non-hue carrier per semantic pair, so deuteranopia simulation cannot collapse refutation and survivor into the same signal.
- A version pin recorded by every rendered scene, so re-renders detect palette drift.

## Motion grammar

- One easing curve pinned as explicit control points with a shared sampled table per engine; linear is allowed only for continuous processes such as draw-ons and plot sweeps; spring physics is forbidden in scenes.
- The duration ladder is declared in frames at the project master rate (`τ = round(0.2 × fps)`; steps 1τ, 2τ, 4τ); holds and protected stillness are regions, not moves.
- Exactly two transitions: selection cut (plain hard cut, default) and mutation cut (2–4 frame chromatic displacement at pinned small amplitude, reserved for introduced variation; never a general transition).
- Picture modes switch at argument-beat or narration-clause boundaries; audio cuts only at segment boundaries unless the timeline declares a pickup repair.
- Screen-session dead-ends compress as jump cut plus time-card; speed ramps stay forbidden.
- Every animated element earns a beat of narration, with one named exception: a single ambient layer on title and chapter cards, amplitude-capped, whose content is a real simulation from the owned substrate, never decorative particles.
- Ambient layers stay in the field color family and never use semantic accents unless the card itself carries that semantic claim.
- Authored stillness (the terminal payoff beat, silence between argument beats) is marked protected so automated silence-cutting never eats it.

## Typographic and sonic voice

- Two faces with fixed roles: the personal Iosevka build for code, terminal, captions, and data; one warm display face for titles and chapter cards whose selection is an open decision and whose file gets pinned in the repo once chosen.
- One caption spec with pinned parameters: max line length (~42 chars), position and safe margins, fg/outline pair from the token artifact, and neutral per-word karaoke emphasis as an fg→bright step.
- Karaoke animation is instantaneous at word onset because word timing is narration-coupled and too fast to carry the easing curve honestly.
- One 2–3 note sonic motif pins interval set, duration on the τ ladder, and level relative to narration; it fires on chapter cards and stingers, never over protected stillness.
- Mutation cuts are silent by default; any sonic accent for them is a pinned one-shot in the token artifact, not an editor's ad-lib.
- Foley always uses real close materials (keys, board, paper, room-handling); music is sparse; silence between beats is the premium cue.

## Recurring motifs

- Generation counter on chapter cards and thumbnails as `gen nnn`; it is monotonic per channel, never reused, and gaps persist when a video is removed.
- Phylogenetic tree of ideas; pruned branches in the refutation ramp.
- Ambient simulation fields behind title cards per the motion-grammar exception.
- Entropy meter resolved to garnish: a once-per-video beat at most, never core genome, never a watermark.

## Enforcement

- A genome lint runs over the timeline before render: transitions within the allowed pair, durations on the ladder (narration-coupled and ambient exceptions declared), colors within the semantic table, object count within budget, and machine-checkable amateur tells.
- Scene manifests distinguish assertions the lint can prove from visual claims that require human review.

## Thumbnails

- Templated typographic composites, not generated art: display face, one semantic accent, one real or computed element, generation counter.
- Minimum pinned fields: safe area, type size legible at feed render (~120px wide), badge exclusion zone, allowed semantic pairs at that size, one locked layout.
- Experiments may vary title phrasing, semantic accent choice, and element crop; layout, display face, counter, and contrast floor stay fixed.
- The anti-face bet is explicit: typographic thumbnails forgo the highest-CTR category in exchange for stronger channel memory, and Test & Compare is the falsifier once available.
- The generated-art thumbnail look is a negative signal to exactly the audience this channel wants.

## Risks

1. Audio neglect kills credibility faster than any visual flaw and is the cheapest premium fix; it comes first.
2. Generative imagery is allowed only where artifacts are diegetic (owned simulations, procedural textures), never as illustration of real-world subjects; cloud b-roll of real subjects is a genome violation unless an explicit mutation admits it.
3. Cheap AI motion destroys the duration ladder; the lint holds the line.
4. Borrowed Fireship cadence reads as costume; borrow the confidence, never the timing.
5. Attempting all segment types in video one ships video never; start with three, add one per video.
6. The thumbnail spec locks before video one or the genome never reaches the browse page where selection actually happens.
7. Motifs decay with overuse; rarity is what makes them signatures.

## Style hypotheses

The genome runs unfalsified for roughly its first three videos by construction (H1 needs mixed modes plus retention data); that runway is a committed conjecture, named here so nobody mistakes it for evidence.
Measurability constraints (retention resolution, audience floor, and exploratory small-channel limits) are owned by the publishing analytics loop.

| Hypothesis | Claim | Refuted when | On refutation |
| --- | --- | --- | --- |
| H1 genome | semantic color + generation counter + two-transition grammar makes mixed modes read as one voice | sustained retention cliffs at mode switches relative to same-video baseline slope, once defined against real curves | thin the genome to palette + captions + cards; re-test elements separately |
| H2 lab notebook | terminal/editor segments hold retention at least as well as talking-head segments within the same video | terminal segments repeatedly underperform within-video across several videos | terminal demotes from home base to garnish; camera moves up |
| H3 field report | physical entropy props earn their production cost | prop segments show median-or-lower within-video retention, or production time stays 2× after the pipeline stabilizes | props become occasional garnish |
| H4 signal & noise | meme-density Shorts graft without corrupting long-form | dormant: activates only if the meme avenue activates; evidence weak per publish's resolution limits | drop meme inserts from long-form entirely |

## Open questions

- Display face selection.
- Whether the indigo-heavy grade fights skin tones on real A-roll; a camera grade test settles it before any talking-head video, and the answer lives here, possibly as a per-mode grade exception.
- Mutation-cut spike candidate: chromatic split, cyan ramp perturbation, or two-frame double exposure.
