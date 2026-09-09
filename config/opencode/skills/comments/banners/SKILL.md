---
name: banners
description: Use with comments for structural code or config banners, subsection labels, and external-document blocks; covers admission, hierarchy, glyph families, and display-cell alignment.
---

# Banners

Read `comments` for scope and source-fidelity rules.
Banners provide navigation in code and config.
Add or retain one only when it helps a reader locate a meaningful section that ordinary declarations or spacing do not make clear.
Existing headers, monolithic configuration, files around 300 lines, and deliberately long functions are reasons to inspect navigation, not automatic triggers.
Do not box every helper or add visual weight to a small, obvious file.

## Layout

- Reserve three-row major-section boxes for top-level file structure: top border, labeled body, bottom border.
- Use one-row subsection labels with exactly one blank line above and none below, attached to the code they introduce.
- Label phases inside a long function only when it remains monolithic for a real reason.
- Use an external-document block when a URL needs durable context: opening marker, URL row, indented reason for consulting it, closing marker.

Adapt the comment prefix to the language and target visual column 100 unless a deliberate local design provides a better boundary.
Preserve a coherent local glyph family; repair broken width, hierarchy, attachment, or glyph integrity rather than copying the defect.
Without a coherent local family, use the following canonical construction, expressed as Unicode code points to keep the grammar unambiguous:

- Major boxes use U+256D and U+256E at the top corners, U+2570 and U+256F at the bottom corners, U+2500 rails, and U+2502 body walls.
- Subsection rows start with U+251C and U+2500, then a space, label, space, U+2500 rail, and U+2524 at the right boundary.
- External-document blocks use U+2553 to open, U+2551 before the URL and context rows, and U+2559 to close; indent the context two spaces and prefix it with U+2014 and a space.

Use a space after the comment prefix and inside box walls around the label, extending boxes and subsection rows to the chosen display-cell boundary.
Report the reason for any deliberate change of family or width.
Relationship diagrams have a different grammar; use the diagram skill `docs` only when the caller has already chosen a diagram.

## Safe mutation

Mutate lines containing Nerd Font, box-drawing, multi-width, or visually aligned banner content only with a Python script operating on file lines.
Do not use Edit, Write, `apply_patch`, or shell text mutation on those lines.
Use `/tmp/opencode/` for a temporary script when that makes the transformation easier to inspect.
Measure terminal display cells, not bytes or Unicode code-point counts.
Re-read every touched region and verify glyph integrity, attachment, and alignment.
Banner-only work must leave surrounding prose, ordinary comments, names, and code unchanged.
