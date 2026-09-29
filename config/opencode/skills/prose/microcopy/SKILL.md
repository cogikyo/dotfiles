---
name: microcopy
description: "Use when writing or changing user-visible UI text in frontend work: labels, placeholders, help and empty-state text, tooltips, toasts, errors, confirmations, and button or menu copy."
---

# Microcopy

People read UI text while they do something else, so every string competes with their task for attention.
Assume the reader is an expert user of the product unless the project says otherwise.

## Defaults

A project's design or copy guide overrides these defaults.

### Less text

- Add no help, hint, or empty-state text unless the screen would otherwise confuse an expert user.
- Fix unclear UI with a better label, icon, or layout before adding explanatory text.
- Do not repeat context the screen already shows, such as a timezone name the page already implies.
- Do not prefix a control with a label such as "Measure:" or "By:" when its value or icon already names it.
- Let a familiar icon act as the label when the product uses that icon consistently, and keep the text as the accessible name.
- Add no tooltip to an action whose icon or label is already clear, such as remove, close, or edit.
- Where a tooltip is warranted, use a native `title` or ARIA attribute instead of a custom tooltip component.
- Keep labels and indicator text on one line; shorten the text before letting it wrap.
- Show keyboard hints only for non-standard shortcuts.
- Let numbers, units, and formats carry meaning without explanatory sentences.

### Consistency

- Use one term for one concept and one icon for one action across the product.
- Match the product's capitalization, and prefer sentence case when it has no convention.

### Actions and feedback

- Name the result on a button, such as "Save view" instead of "Submit", and end labels and buttons without a period.
- Write an error as what happened and what to do next, without apologies, blame, or codes the user cannot act on.
- Ask for confirmation only before a destructive or hard-to-undo action, and name the object and the consequence.
  - Hold-to-confirm text names the object it affects, such as "Delete Q3 view", instead of the gesture.
- Show a toast only for a result the user cannot already see on screen.

## Procedure

Find every user-visible string in scope, including loading, empty, error, disabled, and permission-denied states.
For each string, existing or new, ask whether an expert user would miss it if it were gone.
If not, delete it; if so, shorten it until it is as short as it can be while staying clear.
Replace text with an icon the product already uses for that meaning, and align a term with the product's dominant term for the same concept.
Keep text that prevents a real mistake, explains a non-obvious consequence, or is the only accessible name of a control.
Check the code path behind an error, confirmation, or consequence before you describe it.
Check the result on screen or in a screenshot when layout decides whether a label is needed or fits on one line.
