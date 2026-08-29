# Design

## Three ways to ask, chosen once

| When | What is used |
|---|---|
| Not interactive | Nothing. It refuses, as it always has |
| `fzf` is installed, and both ends are a terminal | `fzf` |
| A terminal that can be drawn on | The arrow selector |
| Anything else | The numbered list, exactly as before |

The order is deliberate: a person who installed `fzf` has opinions about how to pick from a list,
and this tool has no business overriding them. Everybody else gets arrows. A dumb terminal, a
pipe, or a machine with no `TERM` still gets something that works, which is what the numbered
list has always been good at.

## What the selector does and does not do

Up and down move, wrapping at the ends. `j` and `k` move as well, because the people using this
spend their day in editors that use them. A digit jumps to that option and takes it. Enter takes
the highlighted one.

**Escape does nothing**, and that is a decision rather than an omission. Existing callers read
`REPLY` and act on it; a cancel that returned an empty answer would make them continue with
nothing chosen, which for `hm down` means the wrong branch of a destructive question. Ctrl-C still
does what Ctrl-C does everywhere.

`fzf` is different: aborting it is unambiguous, so when it returns nothing the command stops with
the interrupted exit code rather than guessing.

## Redrawing without scrolling

The list is drawn once and then rewritten in place: the cursor goes up as many lines as there are
options, and each line is cleared and reprinted. Nothing scrolls, so the question above the list
stays where it was — which is the part people were losing every time they typed a wrong number.

## The logic is separable from the terminal

Moving through a list and rendering it are two functions that take an index and return text, with
no terminal involved. They are tested directly; the terminal path is tested through a
pseudo-terminal, and what is left in the middle is a `read`.

That split is the reason this can be trusted at all: the alternative is a component that can only
be verified by a person looking at it.
