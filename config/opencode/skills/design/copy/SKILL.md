---
name: copy
description: Use with design when writing, reviewing, or trimming user-facing words on any product surface.
---

# Copy

Work on **words**; load `hierarchy`, `color`, or `motion` when the approved scope includes visual emphasis.

## The test

- Apply the parent's **would the reader miss it?** test to each string.
  - Shorten surviving text as far as clarity allows, keeping the parent's floor.
- Give each string **one job** and say each idea once.
  - _Readers should not have to separate new information from repetition._
- Prefer a better label, familiar icon, or layout over explanatory text.
- Use the product's dominant term for each concept.

## Truth

- Never invent values, stats, testimonials, logos, or claims to fill space.
  - **Empty is better than deceptive**; label illustrative values honestly.
- Check the code path behind an error, confirmation, or consequence before describing it.

## Clutter tells

Apply the parent's test to these word-level tells:

- **Repeated context:** text that says what the screen already shows.
  - An eyebrow or kicker repeating its heading.
  - Helper text or a subtitle restating its label.
  - An obvious control's tooltip repeating "Add line".
  - `A · B · C` metadata or `Key: value` rows the screen already implies.
  - A prefix such as "Sort by:" when the value names itself.
- **Filler claims:** vague promises that do not explain what the reader can do.
  - Replace "Supercharge your workflow" with a specific, supported benefit or remove it.

## Surfaces

Include loading, empty, error, disabled, and permission-denied states.

- **Actions:** name the result, such as "Save view" rather than "Submit"; use no trailing period.
- **Errors:** say what happened and what to do next, without apology, blame, or codes the reader cannot act on.
- **Confirmations:** use only for destructive or hard-to-undo actions; name the object and consequence.
- **Toasts and notifications:** show only results not already visible.
- **Empty states:** offer one clear next action or nothing.
- **Tooltips:** use as a last resort; prefer native `title` for optional hints and ARIA for accessible names.
  - Check optional hints are available to the intended input methods.
- **Labels:** keep one line and use sentence case unless the product has a convention.
  - Use one term per concept across the product.
  - Check the render when layout decides whether a label is needed or fits.
- **Keyboard hints:** show only non-standard shortcuts.
- **CLI/TUI output and logs:** lead with the result and print only what the reader acts on.
- **Landing and marketing pages:** give each section one message; use no filler stats or unsourced social proof.
