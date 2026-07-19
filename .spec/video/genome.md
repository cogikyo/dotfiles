# Channel genome

The channel is a vagari phenotype: a stable genome (palette semantics, motion grammar, typographic voice, sonic motif) with segment types as freely varying phenotypes.
Mixed presentation modes are the thesis, memetic variation under selection pressure demonstrated on screen, so mode mixing must read as one voice rather than mush.
The genome is enforceable data, not vibes: every quantitative rule below resolves to pinned values in one token artifact, and a lint stage rejects violations at the timeline boundary.

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

Each type also carries two or three enumerable amateur tells the lint or reviewer can check: talking head — mixed color temperature in frame, no rim separation; terminal — prompt noise, uniform robotic keystroke timing; explainer — motion during silence, more than one easing idiom.

## Palette semantics

Accents carry Popperian meaning, held constant across footage grade, motion, plots, captions, and thumbnails:

| Semantic | vagari ramp | Meaning |
| --- | --- | --- |
| refutation | rby | the idea that dies here |
| conjecture | sun | the what-if |
| survivor | emr | earned, used sparingly |
| mutation | cyan | variation, noise, randomness |
| field | fg over drk | epistemic background, uncommitted |

Semantic names are the authority; scene code never uses raw ramp names, so a ramp rename touches one alias table.
A viewer learns within a few videos that red means "this dies here"; the epistemology is rendered as color.

## Token artifact

One versioned token file feeds every consumer; it carries more than hex:

- Context-keyed resolution: semantic plus surface (data-viz accent, caption highlight, thumbnail at feed size, scene on dark field) yields one exact ramp step, pre-verified against a contrast floor.
- Color-space and transfer declaration (Rec.709 versus sRGB) and grade anchors: shadow-lift target, highlight desaturation, and a skin-hue protection wedge, because a LUT is a transform and hex values alone cannot define it.
- Per-consumer derivation recipes: ASS caption color form, plot theme values, thumbnail template values.
- A version pin recorded by every rendered scene, so re-renders detect palette drift.

## Motion grammar

- One easing curve pinned as explicit control points with a shared sampled table per engine; spring physics is forbidden in scenes.
- The duration ladder is declared in frames at the project master rate (τ ≈ 200ms equivalent; steps 1τ, 2τ, 4τ); master rate is a project-level conform decision.
- Exactly two transitions: selection cut (plain hard cut, default) and mutation cut (2–4 frame chromatic displacement at pinned small amplitude, reserved for introduced variation; never a general transition).
- Modes switch between argument beats, never mid-sentence.
- Every animated element earns a beat of narration, with one named exception: a single ambient layer on title and chapter cards, amplitude-capped, whose content is a real simulation from the owned substrate, never decorative particles.
- Authored stillness (the terminal payoff beat, silence between argument beats) is marked protected so automated silence-cutting never eats it.

## Typographic and sonic voice

- Two faces with fixed roles: the personal Iosevka build for code, terminal, captions, and data; one warm display face for titles and chapter cards whose selection is an open decision and whose file gets pinned in the repo once chosen.
- One caption spec with pinned parameters: max line length (~42 chars), position and safe margins, one fg/outline pair from the token artifact, and one karaoke granularity (per-word fill in a single semantic color, or plain sentence swap — chosen once, never mixed).
- One 2–3 note sonic motif at chapter transitions; foley always real and close; music sparse; silence between beats is the premium cue.

## Recurring motifs

- Generation counter on chapter cards, continuing across videos and the channel itself; the strongest single identity idea here.
- Phylogenetic tree of ideas; pruned branches in the refutation ramp.
- Ambient simulation fields behind title cards per the motion-grammar exception.
- Entropy meter resolved to garnish: a once-per-video beat at most, never core genome, never a watermark.

## Enforcement

- A genome lint runs over the timeline before render: transitions within the allowed pair, durations on the ladder (narration-coupled and ambient exceptions declared), colors within the semantic table, object count within budget.
- The lint converts "the grammar is enforced" from a wish into a check, mirroring the citation-as-fixture discipline in the writing stage.

## Thumbnails

- Templated typographic composites, not generated art: display face, one semantic accent, one real or computed element, generation counter.
- Minimum pinned fields: safe area, type size legible at feed render (~120px wide), allowed semantic pairs at that size, one locked layout.
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
Within-video retention measures work from video one; cross-video comparisons, recognition, and conversion arms need an audience floor and stay dormant until it exists.

| Hypothesis | Claim | Refuted when | On refutation |
| --- | --- | --- | --- |
| H1 genome | semantic color + generation counter + two-transition grammar makes mixed modes read as one voice | sustained retention cliffs at mode switches relative to same-video baseline slope, once defined against real curves | thin the genome to palette + captions + cards; re-test elements separately |
| H2 lab notebook | terminal/editor segments hold retention at least as well as talking-head segments within the same video | terminal segments repeatedly underperform within-video across several videos | terminal demotes from home base to garnish; camera moves up |
| H3 field report | physical entropy props earn their production cost | prop segments show median-or-lower within-video retention, or production time stays 2× after the pipeline stabilizes | props become occasional garnish |
| H4 signal & noise | meme-density Shorts graft without corrupting long-form | dormant: activates only if the meme avenue activates, and 1–3s inserts sit near retention resolution, so evidence will be weak | drop meme inserts from long-form entirely |

## Open questions

- Display face selection.
- Karaoke granularity: per-word fill or sentence swap.
- Whether the indigo-heavy grade fights skin tones on real A-roll; a camera grade test settles it before any talking-head video, and the answer lives here, possibly as a per-mode grade exception.
