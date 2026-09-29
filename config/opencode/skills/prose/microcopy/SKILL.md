---
name: microcopy
description: Use with prose when writing, reviewing, or trimming UI text in frontend work, including labels, placeholders, help and empty-state text, tooltips, toasts, errors, confirmations, and button or menu copy.
---

# Microcopy

Read `prose` for the source-fidelity and editing procedure.
People read UI text while they do something else, so every string competes with their task for attention.
Assume the reader is an expert user of the product unless the project says otherwise.

## Defaults

A project's design or copy guide overrides these defaults.

- Add no help, hint, or empty-state text unless the screen would otherwise confuse an expert user.
- Do not prefix a control with a label such as "Measure:" or "By:" when its value or icon already names it.
- Let a familiar icon act as the label when the product uses that icon consistently, and keep the text as the accessible name.
- Add no tooltip to an action whose icon or label is already clear, such as remove, close, or edit.
- Show keyboard hints only for non-standard shortcuts.
- Give every action in a menu an icon, and use the same icon for the same action across the product.
- Use one term for one concept across the product.
- Match the product's capitalization, and prefer sentence case when it has no convention.
- Name the result on a button, such as "Save view" instead of "Submit", and end labels and buttons without a period.
- Write an error as what happened and what to do next, without apologies, blame, or codes the user cannot act on.
- Ask for confirmation only before a destructive or hard-to-undo action, and name the object and the consequence.
- Show a toast only for a result the user cannot already see on screen.
- Let numbers, units, and formats carry meaning without explanatory sentences.

## Procedure

Find every user-visible string in scope, including loading, empty, error, disabled, and permission-denied states.
Judge each string by whether an expert user would miss it, then delete, shorten, replace with an icon, or align it with the product's term.
Check the result on screen or in a screenshot when layout decides whether a label is needed.
