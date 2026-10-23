---
name: prose
description: Use when writing or editing prose that lands in a file or gets published, such as docs, agent prompts, skills, comments, commit messages, or PR bodies, including when Collab writes directly; owns style, evidence, structure, and editing procedure, with documentation and diagram subskills.
---

# Prose

This skill owns universal prose style and the writing procedure.
It does not change the owner's write scope or authority.

## Compose

Read the subskills the task needs; they combine freely:

- `docs` (`prose/docs/SKILL.md`) for READMEs, guides, usage notes, and durable repository documentation.
- `diagram` (`prose/diagrams/diagram/SKILL.md`) for an architecture diagram or annotated tree that has already earned its place.
- `workflow` (`prose/diagrams/workflow/SKILL.md`) for numbered workflow graphs in proposals and runs.

Attention-friendly presentation keeps a document's necessary explanation and safety context.
User-facing product text belongs to `copy` (`design/copy/SKILL.md`), under the `design` skills.

## Procedure

Identify the intended reader, their task or question, and the source that supports the answer.
Inspect relevant code, config, commands, or supplied evidence before making technical claims; neighboring writing can inform vocabulary and voice but cannot override source truth.
Verify exact names, options, constraints, and examples, and expose missing evidence instead of filling it with expectations.

Choose content and structure for the reader's need, then remove repetition before polishing sentences.
Lead with the answer or result; readers often read the opening and the summary first, then jump to the parts they need.
Keep a qualification with the claim it limits, and preserve a causal explanation when removing it would invite a wrong inference.
Read the result as a new reader: can they understand the claim, make the intended decision, or complete the task without guessing?
Report source conflicts and unverified claims rather than hiding them with confident wording.

## Style

Put one sentence on each source line in comments and Markdown prose, unless two very short sentences fit comfortably on one line.
Keep related sentence lines adjacent so they render as one paragraph; do not manually wrap them or insert a blank line merely because a sentence ended.
Use blank lines between real Markdown blocks, including paragraphs, headings, lists, callouts, and fences.

Keep each paragraph about one topic, with adjacent sentence lines when it needs development.
Prefer concise, complete sentences over dense paragraphs or a series of extremely short sentences.
Name headings for their content, not generic labels such as Overview or Details.
For in-repo comments and docs, prefer ASD-STE100: common words, one meaning per word, one term per idea, short sentences, and active voice.

## Slop catalogue

Cut the rhetorical move, not just its stock phrasing:

- **Cataphoric teasers / setup-payoff:** "Here's the part nobody tells you...", "Here's what most people get wrong...", "Here's where it gets interesting..."
- **Negative parallelism / fake contrast:** "it's not X, it's Y", "the question is not X, it is Y", "not just X, but Y", "No X. No Y. Just Z"; frequent ", not" is a warning sign.
- **Hypophora:** a question asked only so the next sentence can answer it.
- **Throat-clearing:** "It is worth noting", "To be clear", "Make no mistake", "The reality is", "Absolutely", "Great question", "Let's dive in".
- **Significance inflation:** "This underscores/highlights/signals", "quietly becoming", "sits at the intersection", "a new era".
- **Tidy cadence:** stacked contrasts, rule of three, mirrored clauses, and landing sentences that restate the point.
- **Corporate filler:** delve, leverage, unlock, seamless, robust, landscape, ecosystem, pivotal, transformative.
- **Em dashes:** use rarely as attention signals.

If a sentence performs suspense, fake contrast, or depth, drop the frame and keep the claim.
Use an analogy only when a reader is struggling to understand in a learning context; do not decorate ordinary explanations, reports, or patches with one.
When an analogy earns its place, prefer biology, mathematics, physics, or systems.

## Paragraphs and lists

Paragraphs explain connected reasoning, lists enumerate peers, and nesting encodes dependence.
Choose the Markdown block that matches the relationship between its ideas.
Write clear sentences first, then shape them into a list.

- Use a short paragraph, usually two to four sentences, for connected explanation, causal reasoning, framing, or synthesis.
- Use a flat list for peer items that answer the same question and can be scanned independently.
- Use nested bullets when a child explains, qualifies, exemplifies, or operationalizes its parent.
  - The parent states the main idea, and the child depends on it for context.
- Use a numbered list only when sequence, priority, or dependency matters.
- Give a conceptual section a paragraph when readers need to understand why its ideas connect.
- Let a self-explanatory catalog, checklist, or rule family remain list-only.

**Shape each bullet to stay whole.**

- Give each bullet one short, full sentence that fits on one rendered line with room to spare.
  - The marker, indent, and any headline use up width.
- The usual miss is subtle: a bullet that spills one to three words onto the next line.
- Nest as punctuation: split where a comma or semicolon would go, and move the rest to a child.
- Write children as short, full sentences, not clipped fragments.
- Move sustained reasoning into a nearby paragraph instead of stretching a bullet.
- An occasional wrap is fine; when several bullets wrap, shorten or nest them.

**Emphasize what a scanning reader should land on.**

- Bold the key term or the action in an item, never a whole sentence.
- Mix plain bullets with `**Headline:** context` bullets.
  - A bold item can stand alone when it is simply important.
- When a section holds several groups, open each with a bold line instead of a deeper heading.

**Don't overuse lists.**

- A list needs at least two peers; a single item is a sentence.
- Keep a chain of causes in a paragraph, since bullets drop the "because" and "so" between steps.
- If each bullet only makes sense after the one before it, the block is a paragraph.
- Introduce a list with a sentence saying what its items answer, unless the heading does.
- Stop at two nesting levels; deeper structure wants a heading, a split, or a table.
- Do not stack one-sentence paragraphs to imitate a list.

## Tables

Use tables only for compact comparisons across stable columns.
Write short phrases in cells and keep each body cell on one rendered line at the expected reading width.
Keep source rows within the repository's display-width limit.
Shorten labels, remove columns, or choose another structure when cells wrap.
Put qualifications and explanations in prose below the table.

## Code blocks

- Fence content only when it needs literal formatting, copyable input, or syntax highlighting.
- Fence every multi-line snippet, command transcript, or aligned mapping whose spacing must survive.
- Tag each fence with the most specific language, such as `bash`, `go`, `json`, or `diff`.
- Keep ordinary prose, lists, findings, and path lists out of `text` fences.
  - Reserve `text` for diagrams, raw terminal output, and deliberately unhighlighted artifacts.
