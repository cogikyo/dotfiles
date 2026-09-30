---
name: copy
description: Use when writing, reviewing, or trimming user-facing text in any product surface; covers clear, truthful copy and the attention spent by text and its emphasis.
---

# Copy

Work on product copy and its emphasis within the project's existing design.
A project's design or copy guide, such as `DESIGN.md`, overrides these defaults.

## Attention

> [!INFO] Attention is a fixed budget
>
> Every string, label, icon, badge, color, weight, border, and animation spends **attention**.
> *Adding things makes readers work harder to find what matters.*
> NN/g: "The very fact that something appears on the initial display tells users that it's important."

## The test

- Treat every element as **unnecessary until proven essential**.
- For each string or element, ask whether the reader would miss it if it were gone.
  - If not, delete it.
  - If so, shorten it as far as clarity allows.
- Give each element **one job** and say each idea once.
  - *Readers should not have to separate new information from repetition.*

## Truth

- Never invent values, stats, testimonials, logos, or claims to fill space.
  - **Empty is better than deceptive**; label illustrative values honestly.
- Check the code path behind an error, confirmation, or consequence before describing it.

## Emphasis

Size, color, saturation, contrast, weight, borders, and motion guide attention as words do.
*Readers use prominence to judge importance before they read.*

- Spend **boldness in one place** and keep the rest quiet.
- Use **one accent color** for primary action, selection, or state.
- Use **motion** only to convey state.
- If everything is emphasized, nothing is.

## Clutter tells

> [!INFO] Filling space
>
> Ban the **move** of adding things without a reader need; these are examples, not a ban list.
> *A ban list only moves the clutter into another form unless you apply the test.*

- **Repeated context:** text that says what the screen already shows.
  - An eyebrow or kicker above a heading, especially one that repeats it.
  - Helper text or a subtitle restating its label.
  - An obvious control's tooltip repeating "Add line".
  - `A · B · C` metadata or `Key: value` rows the screen already implies.
  - A prefix such as "Sort by:" when the value names itself.
- **Borrowed importance:** decoration that makes ordinary content compete for attention.
  - A badge or pill above a headline.
  - A hero stat row or big-number-small-label treatment.
  - Decorative icons and icon tiles.
  - Status chips or pulsing dots on static state.
- **Unneeded structure:** markers without a useful relationship to explain.
  - `01/02/03` section numbers without a sequence.
  - `→` appended to links without adding meaning.

## Floor

- Keep text that prevents a **real mistake**, explains a non-obvious consequence, or is a control's only accessible name.
- Keep **hierarchy** and deliberate character.
- Keep essential meaning visible instead of moving it into tooltips.
  - *Hiding meaning makes readers hunt for it* (NN/g); "Mystery isn't minimalism" (Impeccable).

## Surfaces

- **Actions:** name the result, such as "Save view" rather than "Submit"; use no trailing period.
- **Errors:** say what happened and what to do next, without apology, blame, or codes the reader cannot act on.
- **Confirmations:** use only for destructive or hard-to-undo actions; name the object and consequence.
- **Toasts and notifications:** show only results not already visible.
- **Empty states:** offer one clear next action or nothing.
- **Tooltips:** use as a last resort; prefer native `title` for optional hints and ARIA for accessible names.
  - Neither replaces visible text needed to complete a task.
- **Labels:** keep one line and use sentence case unless the product has a convention.
  - Use one term per concept and one icon per action across the product.
- **Keyboard hints:** show only non-standard shortcuts.
- **CLI/TUI output and logs:** lead with the result and print only what the reader acts on.
- **Landing and marketing pages:** give each section one message; use no filler stats or unsourced social proof.

## Procedure

1. Find every user-visible string and element in scope, including loading, empty, error, disabled, and permission-denied states.
2. Apply **the test** to each, keeping the floor above.
3. Prefer a better label, familiar icon, or layout over explanatory text.
   - Keep the accessible name and stay within the approved change scope.
4. Align terms with the product's dominant term for each concept.
5. Check the rendered result when layout decides whether a label is needed or fits.

Credits: [Anthropic frontend-design](https://github.com/anthropics/skills/tree/main/skills/frontend-design), [Impeccable](https://impeccable.style), [NN/g tooltip guidelines](https://www.nngroup.com/articles/tooltip-guidelines/), and [Linear's calmer interface](https://linear.app/now/behind-the-latest-design-refresh).
