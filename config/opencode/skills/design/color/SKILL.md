---
name: color
description: Use with design when building or reviewing palettes, accent colors, saturation, color meaning, gradients, glow, or color accessibility; keeps color purposeful and consistent.
---

# Color

Read `design` first for the attention principle, shared test, floor, and project override.
Use `hierarchy` for visual ordering; this skill owns palette and color meaning.

## Reserve the accent

Let **neutrals dominate** and use color as an accent.
*A quiet base makes meaningful color easier to recognize.*

- Use **one accent per view** for primary action, current selection, or state.
  - Avoid assigning new accents to ordinary content for decoration.
  - Required semantic colors, such as error and warning, keep distinct roles rather than becoming extra accents.
- Treat **saturation and color contrast** as emphasis.
  - Keep supporting surfaces quieter than the meaningful accent.
  - Reuse the project's palette and tokens before introducing a new color.
- Give color **consistent meaning** across the product.
  - A hue used for danger should not also decorate an unrelated feature.

## Make meaning accessible

- Never rely on **color alone** for status, selection, errors, or action affordances.
  - Pair it with a label, shape, icon, or other perceivable cue appropriate to the state.
- Keep text, controls, and focus indicators at the **required contrast** against their actual backgrounds.
  - Check relevant themes and states, including colored surfaces.
  - *A subdued palette must still be legible to the people using it.*
- Never use **glow as the primary affordance**.
  - The control must remain recognizable without the glow.

## Clutter tells

Apply the parent's test to these examples of **decorative emphasis**:

- **Gradient text:** changing hue across a headline or number without encoding information.
- **Extra accents:** unrelated colors assigned to each icon, card, or section.
- **Colored scenery:** decorative background washes, halos, or tinted icon tiles with no useful meaning.

## Procedure

1. Identify the accent and the semantic role of each non-neutral color in scope.
2. Apply the parent's test to color treatments with no distinct role.
3. Check that removing color alone does not remove essential meaning or control recognition.
4. Check rendered contrast in relevant states and themes; state what could not be checked.

Credits: [Impeccable](https://impeccable.style/anti-patterns), [ibelick baseline-ui](https://github.com/ibelick/ui-skills/blob/main/skills/baseline-ui/SKILL.md), and [Linear's calmer interface](https://linear.app/now/behind-the-latest-design-refresh).
