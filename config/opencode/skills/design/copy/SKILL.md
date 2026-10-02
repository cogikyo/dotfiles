---
name: copy
description: Use with design when writing, reviewing, or trimming user-facing words on any product surface; covers truthful claims, labels, messages, and word-level clutter.
---

# Copy

Read `design` first for the attention principle, shared test, floor, and project override.
Work on **words**; load `hierarchy`, `color`, or `motion` when the approved scope includes visual emphasis.

## The test

- Apply the parent's **would the reader miss it?** test to each string.
  - Shorten surviving text as far as clarity allows, keeping the parent's floor.
- Give each string **one job** and say each idea once.
  - _Readers should not have to separate new information from repetition._

## Truth

- Never invent values, stats, testimonials, logos, or claims to fill space.
  - **Empty is better than deceptive**; label illustrative values honestly.
- Check the code path behind an error, confirmation, or consequence before describing it.

## Clutter tells

> [!INFO] Filling space
>
> Words need a **reader need**, even when there is room for them.

_Repeated context makes readers spend time separating new information from what they already know._
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

- **Actions:** name the result, such as "Save view" rather than "Submit"; use no trailing period.
- **Errors:** say what happened and what to do next, without apology, blame, or codes the reader cannot act on.
- **Confirmations:** use only for destructive or hard-to-undo actions; name the object and consequence.
- **Toasts and notifications:** show only results not already visible.
- **Empty states:** offer one clear next action or nothing.
- **Tooltips:** use as a last resort; prefer native `title` for optional hints and ARIA for accessible names.
  - Check optional hints are available to the intended input methods.
- **Labels:** keep one line and use sentence case unless the product has a convention.
  - Use one term per concept across the product.
- **Keyboard hints:** show only non-standard shortcuts.
- **CLI/TUI output and logs:** lead with the result and print only what the reader acts on.
- **Landing and marketing pages:** give each section one message; use no filler stats or unsourced social proof.

## Procedure

1. Find every user-visible string in scope, including loading, empty, error, disabled, and permission-denied states.
2. Apply **the test** to each, keeping the parent's floor.
3. Prefer a better label, familiar icon, or layout over explanatory text.
   - Keep the accessible name and stay within the approved change scope.
4. Align terms with the product's dominant term for each concept.
5. Check the rendered result when layout decides whether a label is needed or fits.

Credits: [Anthropic frontend-design](https://github.com/anthropics/skills/tree/main/skills/frontend-design), [Impeccable](https://impeccable.style/anti-patterns), and [NN/g tooltip guidelines](https://www.nngroup.com/articles/tooltip-guidelines/).
