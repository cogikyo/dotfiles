---
name: hierarchy
description: Use with design when building or reviewing visual hierarchy, typography, layout, spacing, grouping, cards, borders, or shadows.
---

# Hierarchy

This skill owns visual ordering and structure; use `copy`, `color`, or `motion` for those treatments.

## Order the view

Prominence tells the reader what to notice before they read.
_A clear order lets people find the task without inspecting every element._

- Give each screen **one focal point** based on the user's current task.
  - A title and a primary action should not compete at equal strength.
- **De-emphasize to emphasize** before making the focal point louder.
  - Reduce supporting elements' size, weight, or relative contrast while keeping them readable.
- Use **size, weight, contrast, and position** to express importance consistently.
  - Start with a small type scale, usually three or four sizes and two or three weights.
  - Align related elements so the reading order matches their relationship.
- Check the rendered reading order and readability.

## Group before enclosing

- Use **whitespace and proximity** to show what belongs together.
  - Keep related items closer than separate groups.
- Add a **container** only when it explains a boundary or interaction.
  - Prefer spacing or a divider to nesting cards inside cards.
- Check whether grouping remains clear after removal, including at narrow widths.
- Declare an edge or elevation **once**: choose a border or a shadow when both do the same job.
  - _Repeated outlines make readers sort the containers before reaching their content._
- Make **structural markers** carry information.
  - Numbering needs a real sequence; a link arrow needs to explain direction or behavior.
  - Use one icon per action consistently across the product.

## Clutter tells

Apply the parent's test to these examples of **borrowed importance** or unneeded structure:

- **Accessories:** a badge or pill above a headline, a tracked ALL-CAPS eyebrow, or status chips adding no useful grouping or interaction.
- **Template prominence:** hero stat rows, big numbers with small labels, or decorative icon tiles.
  - Keep a metric-led view when the metric genuinely leads the task.
- **Container accumulation:** nested cards, every item boxed, or a border and shadow defining the same surface.
- **False order:** `01/02/03` section markers on content with no sequence.
