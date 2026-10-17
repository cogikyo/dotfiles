---
name: balance
description: Use with design whenever a dense view must show all its content and still guide attention, or when the user says "balance" or "balancing stones"; owns the rendered loop across rounds.
---

# Balancing stones

Read `design` first; `hierarchy` owns the individual levers.
A dense view is balanced when all its content fits, the eye lands where the task starts, and no element pulls so hard that its neighbours disappear.
The answer depends on the content, so find it on the rendered view instead of planning it in advance.

## Read the content

- List every value the view must show and what the reader does with it: scan, compare, copy out, or glance.
- Name the one or two values the task starts from; they get the strongest position.
  - Take the user's priority order over your own when they differ.
- Collect the real extremes: fewest and most items, longest values, and empty values.

## Spread the levers

- Spread emphasis across size, weight, icon, color, format, and position so each value carries at most one strong treatment.
  - _Stacking size, weight, color, and a badge on one element makes it shout and flattens everything around it._
- Let structure name a value before a label does: position, a format such as currency or monospace, or an icon with one meaning.
  - Keep the label when nothing else names the field, and keep the full name in `title` or `aria-label` when it goes.
- Repeat an element's identity, such as its icon or color, wherever it appears.
- Keep key lines on one row; truncate the lowest-priority value first, with the full value in `title`.

## Each round

1. Render real records at real size and look; text and ASCII mockups cannot show weight.
   - If you cannot render, ask for a screenshot instead of moving.
2. Take the misfit from the user's latest steer, restated in one sentence; ask when it could mean two things.
   - When they gave none, name the worst misfit and say it is your pick.
3. Make one move for it.
4. Re-render the whole view and revert the move if an earlier fit broke, because moves in a dense view interact.
5. Show the render and wait for the user's next steer; stop when they name no misfit.

## Floor

- Every non-empty value stays on screen and nameable; removing or merging a field waits for the user.
- On the look, the user's instinct wins; push back once, with a render.

Credits: Christopher Alexander, _Notes on the Synthesis of Form_ (fit as the absence of misfits); Edward Tufte, _Envisioning Information_ (layering and separation); [37signals, "Use real content to judge a design"](https://signalvnoise.com/posts/2149-use-real-content-to-judge-a-design); [Hyperplexed, "I Rescued Abandoned UI Designs"](https://www.youtube.com/watch?v=1_y9qCIPhQ4).
