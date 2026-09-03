# `down`, and the list you choose from

## Why

`down` is one line — stop and remove — except for one letter. With `-v` it deletes the volumes and
the database with them: no warning, no way back. An environment on this machine was lost exactly
that way while the shell implementation was being written, which is why that version asks, says
which volumes, and offers to save a copy first.

Porting it means porting the question, and the question is not a yes or no. It is three answers,
picked from a list — which is a piece the CLI needs anyway: `setup` asks two, the mail catcher is
one, and everything the wizards do is one.

So this is the selector, and `down` on top of it.

## What Changes

- **A list you move through, in Go**, with the three ways of asking the shell implementation has
  and for the same reasons: `fzf` when somebody installed it, because they have opinions about
  picking from a list; the arrow keys when the terminal can be drawn on; and the numbered list
  everywhere else, which has always been good at working anywhere. Non-interactive refuses,
  because a choice between options has no safe default.
- **`hm down` is Go**, and without `-v` it is what it always was: stop and remove, destroying
  nothing that cannot be rebuilt.
- **With `-v` it asks**, naming the volumes it is about to delete, and offers to take a database
  snapshot first — which is the option that is offered first because it is the one nobody regrets.
- **Saving and failing to save destroys nothing.** If the copy cannot be taken, the environment is
  left standing and the command says so.
- **The flags `down` accepts are the ones Compose's own `down` accepts**: `-v`, `--remove-orphans`,
  `--rmi` and `-t`. Anything else is a usage error rather than being passed on to be complained
  about somewhere deeper.

## And one thing that stops being said twice

`stop --snapshot` asked the shell implementation to take the copy, because that is where snapshots
were. They are not there any more, so it takes it directly: one implementation of a copy, and one
place where what it writes is decided.

## What this fixes on the way

Removing an environment through the library always removed orphans, whether or not it was asked
to. That is right when a branch environment goes — it is being erased — and wrong for `down`,
which is Compose's command and should do what Compose does. It is now something the caller says.
