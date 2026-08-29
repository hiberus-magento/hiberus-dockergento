# A list you can move through

## Why

Every choice this tool asks for is Bash's `select`: a numbered list, and you type a number. It has
three problems that are not style.

- **There is no default.** `hm down -v` offers "save and destroy", "destroy", "cancel", and the
  safest of the three is not reachable by pressing Enter — it has to be typed, like the others.
- **The arrow keys do nothing.** Everybody presses them first, and what arrives is `^[[A` in the
  answer, an "invalid option" and the question again.
- **The list is redrawn on every mistake**, so a wrong keystroke scrolls the thing you were
  reading off the screen.

The terminal primitives to fix it have been there since the dashboard: raw key reads, arrow
decoding, cursor movement, and a decision about when a terminal can be drawn on.

## What Changes

- `custom_select` becomes a list you move through with the arrow keys — or `j`/`k`, or a digit —
  with the first option preselected and Enter taking it.
- **`fzf` is used when it is installed**, because somebody who has it wants it: same prompt, same
  answer, their own key bindings.
- **The numbered list stays** for a terminal that cannot be drawn on, and the non-interactive
  refusal is unchanged: a choice between options has no safe default, and picking one silently
  could install the wrong thing.
- Everything that asks a question keeps working unchanged: the callers pass the same arguments and
  read the same `REPLY`.
