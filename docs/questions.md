# How the tool asks

Every choice — which mail catcher, what `hm down -v` should do, which version to switch to — is a
list you move through.

```
✅ What should happen?
  ❯ 1) Save a snapshot and destroy the environment
    2) Destroy it
    3) Cancel
```

Arrow keys or `j`/`k` to move, Enter to take the highlighted one, or the digit of an option to
take it directly. The first option is preselected, so **the safest answer is the one you get by
pressing Enter**.

That was the point of changing it. Bash's `select` — a numbered list where you type a number —
had no default, ignored the arrow keys everybody presses first, and redrew itself on every
mistake, scrolling the question you were reading off the screen.

## Escape does nothing

Deliberately. The commands that ask read the answer and act on it, so a cancel that returned an
empty answer would have them carry on with nothing chosen — and for `hm down -v` that means the
wrong branch of a destructive question. Ctrl-C still does what Ctrl-C does everywhere.

## If you have `fzf`, you get `fzf`

Somebody who installed it has opinions about picking from a list, and this tool has no business
overriding them: same question, same answer, your own key bindings. Aborting `fzf` is
unambiguous, so it stops the command rather than guessing.

## Where nothing can be drawn

A terminal that cannot be drawn on — `TERM=dumb`, or output that is not a terminal — gets the
numbered list, exactly as before. It has always been good at working anywhere.

## Nothing is ever chosen for you

With `--yes` or `HM_NON_INTERACTIVE=1`, a question with a default answers itself and a **choice
between options refuses**:

```
Non-interactive mode cannot choose: How do you want to create the database?
  → Pass the value as an option, or run without --yes
```

A choice has no safe default. Picking one silently could install the wrong thing.
