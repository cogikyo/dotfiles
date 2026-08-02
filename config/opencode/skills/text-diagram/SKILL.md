---
name: text-diagram
description: Use ONLY when the calling agent has decided that a fixed-width diagram or annotated directory tree belongs in repository documentation or a justified code comment; constructs and validates source-derived Unicode layouts with display-width-safe geometry.
---

# Text diagram

## Authority

The calling agent retains authority over whether a diagram, annotated tree, or comment belongs.
Use this skill only after that agent decides to create or modify a fixed-width diagram or annotated directory tree.
This skill owns construction and validation mechanics and must never encourage either representation or a comment.
The caller also owns source-specific comment prefixes, Markdown fences, host conventions, and governing width limits.
Simple non-diagram prose and lists remain outside this skill.

## Construct

1. Derive the layout from inspected source.
2. For a relationship diagram, list every node, group or boundary, edge, direction, and label before drawing.
3. For an annotated tree, list exact parent-child paths and every source-derived annotation before layout.
4. Preserve exact source spelling in paths, titles, and labels.
5. Simplify the topology before layout by removing decorative nodes, redundant routes, and detail outside the caller's scope.
6. Place the remaining topology on a fixed display-cell grid with generous horizontal and vertical spacing.

Use consistent outer widths for peer boxes and consistent interior padding.
Leave at least one display cell between content and each side wall.
Embed a title in its top border with one space on each side of the exact title and horizontal rails outside those spaces.
Keep at least two horizontal rail cells on each side.
Center the title by display cells; if one spare cell remains, put it on the right rail.

## Geometry alphabet

Aside from spaces and source-derived paths or labels, geometry is limited to the eleven light single-line box glyphs in the mask table and the four arrowheads `▶`, `◀`, `▲`, and `▼`.
Do not substitute improvised ASCII geometry or another box-drawing family.
Decorative banner frames remain outside this skill and retain `scribe/comment`'s established glyph-family rules.
Use the light diagram set inside a banner-bearing file and do not rewrite surrounding banners.

## Renderer precondition

The entire geometry alphabet has the Unicode East Asian Width property `Ambiguous`, while `wcwidth` measures it as narrow.
The target terminal or renderer must use narrow ambiguous width and must not apply emoji fallback to the triangle arrowheads.
Keep `wcwidth` as the layout measurement, then inspect this rendered probe:

```text
▶│  │◀  ┌─┐
X│  │X  XXX
```

The walls beside both arrows and the right edges must occupy the same columns as their ASCII control rows.
Do not claim renderer compatibility unless this probe was observed in the target renderer.
If it does not align, report the incompatibility instead of emitting a broken layout.

## Python canvas and mutation

Measure paths, labels, and rows with `wcwidth.wcwidth` and `wcwidth.wcswidth`, never bytes, rune counts, code-point counts, or `len` for visible geometry.
Reject source text with a negative display width.
Track every cell occupied by a wide path or label so later drawing cannot overlap its continuation cell.

The governed unit is every multiline diagram or annotated directory tree selected by the caller; an ordinary prose line containing an inline arrow glyph is outside the unit.
Python-only mutation applies to the complete governed unit.
Use a temporary Python canvas and validator under `/tmp/opencode`.
Represent every geometry glyph in temporary Python source with a `\uXXXX` escape so file-writing tools never mutate a literal geometry glyph in the script.
Python must perform both generation and line-level insertion or replacement in the host file.
Never use Edit, Write, `apply_patch`, or shell text mutation on governed lines containing box-drawing, arrows, multi-width, or aligned content.

Create a unique script path and retain the path returned by:

```bash
mktemp /tmp/opencode/text-diagram.XXXXXX.py
```

Run that exact script with:

```bash
uv run --with wcwidth python /tmp/opencode/text-diagram.<id>.py
```

Remove the returned script path after validation, including after a failed generation or host mutation.
Do not add a renderer or generic canvas script to the repository.

## Connection masks

Represent each stroke cell by the exact set of directions it connects: north (`N`), east (`E`), south (`S`), and west (`W`).
Render line geometry with these masks:

| Mask | Glyph |
| --- | --- |
| `E+W` | `─` |
| `N+S` | `│` |
| `E+S` | `┌` |
| `S+W` | `┐` |
| `N+E` | `└` |
| `N+W` | `┘` |
| `N+E+S` | `├` |
| `N+S+W` | `┤` |
| `E+S+W` | `┬` |
| `N+E+W` | `┴` |
| `N+E+S+W` | `┼` |

Every occupied mask direction must reciprocate unless the opaque-span rule below terminates that one direction.

### Opaque text spans

A title, edge label, annotated-tree entry name or comment, and route-originating label is opaque text rather than route geometry.
Register each opaque span as exact display cells before validating masks.
At each route-facing edge, one separator space belongs to that opaque span when a separator is present.

When a stroke direction lacks a reciprocal route neighbor, inspect the next cell in that direction.
Permit the direction to terminate if that cell is registered opaque text, or if it is the span's registered separator and the following cell is registered text from the same span.
Grant at most one opaque termination direction to a stroke cell.
This makes title rails, `label ───▶`, and tree limbs such as `├── entry` valid without treating text as geometry.
A stroke direction that faces empty space without a registered opaque span or an adjacent terminal arrow tail remains dangling and must fail.
Do not use empty cells to bridge a route to distant text or an arrow.

### Arrow masks

An arrow's tip directions are `▶=E`, `◀=W`, `▲=N`, and `▼=S`; its tail is the opposite direction.
The validator classifies an arrow as inline when route geometry continues in its tip direction and that neighbor connects back to the arrow.
Inline `▶` and `◀` use `E+W`; inline `▲` and `▼` use `N+S`.
Otherwise the arrow is terminal and uses only its tail direction: `▶=W`, `◀=E`, `▲=S`, and `▼=N`.
A terminal tip is intentionally unconnected and does not require reciprocation.
Permit a terminal tip to face only its target wall, an opaque label, or the diagram boundary; reject other incompatible route geometry.
Every occupied mask direction, including a terminal arrow tail, must reciprocate except one stroke direction intentionally terminated by an opaque span.

### Wall attachments

- An outgoing edge through a right wall uses `├`; an outgoing edge through a left wall uses `┤`.
- An outgoing edge through a top rail uses `┴`; an outgoing edge through a bottom rail uses `┬`.
- An incoming horizontal arrow terminates immediately outside an unchanged side wall, as `▶│` from the left or `│◀` from the right.
- An incoming top arrow places terminal `▼` immediately above an unchanged `─` rail cell.
- An incoming bottom arrow places terminal `▲` immediately below an unchanged `─` rail cell.
- No edge connects through a title or corner.

Keep branches and merges continuous, with the correct corner or tee at every turn and no dangling stroke.
Place edge labels next to their routes without hiding geometry, and make every edge's direction unambiguous.
Use `┼` only when all four routes form one true connected junction.
Reroute unrelated crossing paths instead of drawing a false `┼` connection.

## Host the layout

Generate and validate the bare layout before adding a fence, indentation, or comment prefix.
When a governing host width limit exists, the caller supplies it.
Otherwise derive a budget from neighboring hosted material and favor enough room for clear geometry.
Do not inherit `scribe/comment`'s banner column target unless the layout is actually governed by that banner layout.
Do not impose a globally small README width.
Never silently exceed a code-comment width constraint; fit the topology within it or report the conflict to the caller.

Use Python to add the bare layout to the host file.
Apply one byte-identical indentation and comment prefix to every nonblank hosted row; the prefix may be empty for an unindented fenced layout.
A blank comment separator may contain the right-trimmed prefix; normalize it to an empty bare row and exclude it from connectivity checks.
An empty row inside a Markdown fence is valid and normalizes the same way.
Keep Markdown fence lines outside the prefixed layout region.
Strip or normalize prefixes before hosted display-width and connectivity checks.
Reject negative display widths in source labels and stripped bare content.
Do not reject a raw hosted row only because a tab in its prefix has negative display width.

## Examples

The title is centered in display cells, the incoming arrow terminates at the left wall, and the outgoing edge passes through the right-wall tee:

```text
              ┌──── Worker ────┐
request ─────▶│                ├────▶ result
              └────────────────┘
```

The input splits through titled nodes and their outgoing edges form one connected merge:

```text
                       ┌───── Fast ─────┐
                  ┌───▶│                ├────┐
                  │    └────────────────┘    │
input ─────▶──────┤                          ├──────▶ output
                  │    ┌───── Safe ─────┐    │
                  └───▶│                ├────┘
                       └────────────────┘
```

The entry names and source-style comments are opaque spans; the limbs and parent-child continuation remain connected:

```text
root/
├── cmd/          # command group
│   └── serve/    # leaf command
└── config/       # settings
```

## Validate

The temporary Python validator must:

- Reject negative display widths in source text and stripped bare rows.
- Confirm that `wcwidth` measures every geometry glyph as one cell under its narrow-width assumption.
- Detect wide-path or label continuation-cell overlap.
- Register every opaque text span and its one route-facing separator cell before mask validation.
- Check each box's row width, interior padding, title spacing, minimum rails, and display-cell centering.
- Exercise the odd-spare title case and require the extra rail cell on the right.
- Derive every line, corner, tee, and junction mask from neighboring cells.
- Classify each arrow from geometry in its tip direction and apply its inline or terminal mask.
- Require reciprocity for every occupied mask direction, including terminal tails.
- Exempt only a terminal tip and one registered opaque-span direction on a stroke cell.
- Reject a stroke facing unregistered empty space as dangling.
- Reject corner or title attachments and check horizontal and vertical wall semantics.
- Check annotated-tree parent-child continuation, row widths, limb junctions, entry spans, and annotation spans.
- Enforce the caller-supplied limit or derived host budget on the bare layout.
- Require byte-identical prefixes on nonblank hosted rows.
- Normalize permitted blank separators and exclude them from connectivity checks.
- Strip or normalize prefixes before repeating display-width and connectivity checks.

Read the bare layout back visually before host mutation.
Read the complete hosted region back after mutation and inspect it in the target renderer.
During visual read-back, confirm that every `┼` is an intended connected junction.
Confirm that paths, labels, boundaries, edges, and directions match source truth and remain clear without surrounding prose.
Remove the temporary script after all checks or after any failure.
