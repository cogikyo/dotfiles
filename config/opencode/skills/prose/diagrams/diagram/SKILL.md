---
name: diagram
description: Use ONLY for an already-chosen source-derived architecture diagram, boxed relationship, or annotated directory tree in documentation or a justified comment; covers the glyph alphabet, wall and layout rules, and validation with the `diagram` tool, not numbered execution workflows.
---

# Documentation diagram

## Authority

The caller decides whether a diagram, annotated tree, or comment belongs; this skill supplies construction and validation only, never encouragement to add one.
The caller owns source-specific prefixes, fences, host conventions, and width limits.
Ordinary prose and lists remain outside this skill.
Execution graphs for proposals, approval gates, delegation, and repair loops are outside this skill.

## Source facts

1. Inventory inspected source: nodes, boundaries, directed edges, and labels for relationships, or exact parent-child paths and annotations for trees.
2. Preserve source spelling and remove decorative nodes, redundant routes, and out-of-scope detail before layout.
3. Place the topology on a fixed display-cell grid with enough horizontal and vertical space to distinguish routes.

Containment means ownership, while an arrow means flow; a nested box alone must not imply an edge.
A recovery or retry route needs its triggering condition and destination from source.

## Glyph alphabet

Aside from spaces and source-derived paths or labels, geometry is limited to the eleven light box glyphs `─ │ ┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼` and the four arrowheads `▶ ◀ ▲ ▼`.
Do not substitute improvised ASCII geometry or another box-drawing family.
Decorative banner frames remain outside this skill and use the `banners` skill's established glyph-family rules.
Use the light diagram set inside a banner-bearing file and do not rewrite surrounding banners.

## Boxes and walls

Use consistent outer widths for peer boxes and consistent interior padding.
Leave at least one display cell between content and each side wall.
Embed a title in its top border with one space on each side and at least two rail cells on each side.
Center the title by display cells; if one spare cell remains, put it on the right rail.

- An outgoing edge through a right wall uses `├`; through a left wall, `┤`; through a top rail, `┴`; through a bottom rail, `┬`.
- An incoming arrow stops immediately outside an unchanged wall: `▶│`, `│◀`, `▼` above `─`, or `▲` below `─`.
- No edge connects through a title or corner; move the title into the box as an interior label when a route needs that column.

```text
              ┌──── Worker ────┐
request ─────▶│                ├────▶ result
              └────────────────┘
```

## Routes

Keep branches and merges continuous, with the correct corner or tee at every turn and no dangling stroke.
A route may end at a label with at most one separator space, as in `label ───▶` or `├── entry`.
Place edge labels next to their routes without hiding geometry, and give every route an arrowhead.
Use `┼` only when all four routes form one true connected junction; reroute unrelated crossings instead.

## Trees

Entry names and annotations are labels, and each limb is `├──` or `└──` under the first cell of its parent entry.
Keep the `│` trunk running past a nested subtree while a later sibling remains; the subtree's `└` closes only that subtree.
Use exact inspected paths and align annotations by display cells.

```text
root/
├── cmd/              # command group
│   └── serve/        # request entry
└── config/           # settings
```

## Host

When a governing host width limit exists, the caller supplies it.
Otherwise derive a budget from neighboring hosted material and favor enough room for clear geometry.
Do not inherit the `banners` skill's column target unless the layout is actually governed by that banner layout.
Never silently exceed a code-comment width constraint; fit the topology within it or report the conflict to the caller.

Apply one byte-identical indentation and comment prefix to every nonblank hosted row; the prefix may be empty for an unindented fenced layout.
Keep Markdown fence lines outside the prefixed layout region.

## Validate

Run the `diagram` tool on the bare layout as `text` until it reports no violations.
After writing the layout into its host, run it again on the hosted region with `file`, `start`, `end`, `prefix`, and `width`, and fix every violation.

The tool checks geometry, not meaning.
Compare its box sizes, traced edges, or tree paths against the source inventory, so a geometrically valid false join or missing edge still fails.
Confirm that every `┼` is an intended junction and that paths, labels, boundaries, and directions remain clear without surrounding prose.

## Render

The whole geometry alphabet is East Asian Width `Ambiguous`; the tool measures it as narrow.
The target renderer must also draw it narrow and must not apply emoji fallback to the triangle arrowheads.
Inspect this probe in the target renderer:

```text
▶│  │◀  ┌─┐
X│  │X  XXX
```

The walls beside both arrows and the right edges must occupy the same columns as their ASCII control rows.
Do not claim renderer compatibility unless this probe was observed in the target renderer.
If it does not align, report the incompatibility instead of emitting a broken layout.
