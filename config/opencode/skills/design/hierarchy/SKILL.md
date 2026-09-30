---
name: hierarchy
description: Use with design when building or reviewing visual hierarchy, typography, layout, spacing, grouping, cards, borders, or shadows; makes prominence follow the user's task.
---

# Hierarchy

Read `design` first for the attention principle, shared test, floor, and project override.
This skill owns visual ordering and structure; use `copy`, `color`, or `motion` for those treatments.

## Order the view

Prominence tells the reader what to notice before they read.
*A clear order lets people find the task without inspecting every element.*

- Give each screen **one focal point** based on the user's current task.
  - A title and a primary action should not compete at equal strength.
- **De-emphasize to emphasize** before making the focal point louder.
  - Reduce supporting elements' size, weight, or relative contrast while keeping them readable.
- Use **size, weight, contrast, and position** to express importance consistently.
  - Start with a small type scale, usually three or four sizes and two or three weights.
  - Align related elements so the reading order matches their relationship.

## Group before enclosing

- Use **whitespace and proximity** to show what belongs together.
  - Keep related items closer than separate groups.
- Add a **container** only when it explains a boundary or interaction.
  - Prefer spacing or a divider to nesting cards inside cards.
- Declare an edge or elevation **once**: choose a border or a shadow when both do the same job.
  - *Repeated outlines make readers sort the containers before reaching their content.*
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

## Procedure

1. Name the focal point and intended reading order in the view's current state.
2. Apply the parent's test to competing treatments and structural chrome.
3. Check whether grouping remains clear after removal, including at narrow widths.
4. Check the rendered order and readability; report unseen states instead of assuming they work.

Credits: [Anthropic frontend-design](https://github.com/anthropics/skills/tree/main/skills/frontend-design), [Impeccable](https://impeccable.style/anti-patterns), [Refactoring UI](https://www.refactoringui.com/book) (chapter titles only), and [Linear's calmer interface](https://linear.app/now/behind-the-latest-design-refresh).
