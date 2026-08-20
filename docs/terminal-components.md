# Terminal components

`console/components/tui.sh` holds the primitives for drawing on a terminal: size, cursor,
alternate screen, keyboard and resize handling. It is the base the terminal dashboard is
built on, and it has no dependencies.

## Why raw sequences and not `tput`

`tput` costs 10-15 ms per invocation and buys no portability over the VT100 sequences every
terminal of the last thirty years understands. Please do not "modernise" these into `tput`
calls.

| Sequence | What it does |
|---|---|
| `\e[?25l` / `\e[?25h` | Hide / show the cursor |
| `\e[?1049h` / `\e[?1049l` | Enter / leave the alternate screen |
| `\e[<row>;<col>H` | Move the cursor |
| `\e[s` / `\e[u` | Save / restore the cursor position |
| `\e[2K` | Erase the current line |
| `\e[2J` | Erase the screen |

## The alternate screen, not `clear`

`clear` wipes the user's scrollback and there is no way back. Leaving the alternate screen
restores **exactly** what was on screen before entering, scrollback included. That is the
difference between a full-screen tool that respects the terminal and one that vandalises it.

## Restoring is not the caller's job

Entering the alternate screen or hiding the cursor leaves the terminal in a state that must
be undone. If the program dies in between —an error, a `Ctrl-C`, a `kill`— the user is left
with an invisible cursor and no idea why.

So `tui_enter_screen` and `tui_hide_cursor` install a trap on `EXIT`, `INT` and `TERM` that
restores the main screen, the cursor and terminal echo. It is idempotent, because it can
arrive twice, and it **chains** whatever trap the program already had rather than replacing
it.

## Handing the terminal over

`tui_suspend` and `tui_resume` leave and re-enter the alternate screen so another command can
write normally. The dashboard shows the output of `hm start` this way instead of
multiplexing it into a box: if the command fails, the user sees the whole error.

## Bash 3.2

macOS ships Bash 3.2.57, so no associative arrays, no `read -N`, no `wait -n`. Two
consequences worth knowing:

- **Size** comes from `stty size`, which is POSIX and works everywhere, rather than
  `checkwinsize`, which needs Bash 4.
- **Escape timeouts** cannot be fractional. Bash 3.2 rejects them outright:
  `read: 0.01: invalid timeout specification`. So the wait for the rest of an escape
  sequence is a whole second on 3.2 and 0.05 s from Bash 4 on. Arrow keys are instant either
  way —their bytes are already buffered— and only a lone `Esc` pays the wait, which is why
  nothing should depend on `Esc` as its only key.

## Nothing happens without a terminal

Every emitting function returns successfully without writing anything when stdout is not a
terminal, so the same code is harmless in a pipe, in a script or in an agent's hands. The
size is the exception: it returns 24x80, because whoever formats a table needs a number.

## Testing it

The functions are split in two so they can be tested at all:

- **Computing** —parsing `stty size`, naming a key from its bytes, choosing the timeout— are
  pure functions tested with made-up input, no terminal involved.
- **Emitting** is tested by asserting that nothing comes out without a terminal, and against
  a real pseudo-terminal through `script(1)` for the interactive path, including that the
  terminal is restored after an interruption.
