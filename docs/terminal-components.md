# Terminal components

The terminal dashboard (`hm tui`) and the list selector draw with a handful of primitives: size,
cursor, alternate screen, keyboard and resize handling. They live in `internal/cli/tui.go`,
`internal/cli/tui_frame.go`, `internal/cli/tui_render.go` and `internal/cli/select.go`, on
`golang.org/x/term` for raw input and the terminal size. The rules below are the ones they keep.
They were first worked out in the Bash implementation of hm; the reasoning carried over.

## Why raw sequences and not a terminal library

The VT100 sequences every terminal of the last thirty years understands are all the dashboard
needs; a library that abstracts terminals buys no portability here and costs a dependency.
Please do not "modernise" these into one.

| Sequence | What it does |
|---|---|
| `\e[?25l` / `\e[?25h` | Hide / show the cursor |
| `\e[?1049h` / `\e[?1049l` | Enter / leave the alternate screen |
| `\e[<row>;<col>H` | Move the cursor |
| `\e[K` / `\e[J` | Erase to the end of the line / of the screen |
| `\e[?2026h` / `\e[?2026l` | Begin / end a synchronised frame, ignored by terminals that do not know it |

## The alternate screen, not `clear`

`clear` wipes the user's scrollback and there is no way back. Leaving the alternate screen
restores **exactly** what was on screen before entering, scrollback included. That is the
difference between a full-screen tool that respects the terminal and one that vandalises it.

## Restoring is not the caller's job

Entering the alternate screen, hiding the cursor or putting the input in raw mode leaves the
terminal in a state that must be undone. If the program dies in between —an error, a `Ctrl-C`, a
`kill`— the user is left with an invisible cursor and no idea why.

So `restore()` puts back the main screen, a visible cursor and cooked input whatever happened. It
runs on the way out (`defer`) and on `SIGINT` and `SIGTERM`, and it is idempotent, because it can
arrive twice. The selector does the same for its own raw mode and cursor.

## Handing the terminal over

`suspend()` and `resume()` leave and re-enter the alternate screen, and switch raw input off and
back on, so another command can write normally. The dashboard shows the output of `hm start` this
way instead of multiplexing it into a box: if the command fails, the user sees the whole error.

## Size and keys

- **Size** comes from the terminal itself (`term.GetSize`), then the shell's `LINES`/`COLUMNS`,
  then 24x80, the size every terminal has assumed since 1978 — whoever formats a table needs a
  number. `SIGWINCH` recomposes and repaints at once, without waiting for a key.
- **Keys** are read from raw input one press at a time. An escape sequence is the escape and the
  bytes already waiting after it, so arrow keys are instant; an escape alone is `Esc`. Nothing
  should depend on `Esc` as its only key: in the selector it does nothing on purpose.

## Nothing happens without a terminal

The dashboard refuses when stdout is not a terminal (`requires_terminal`, exit 2) instead of
writing control codes into a file, and the selector falls back to a numbered list when the
terminal cannot be drawn on. Progress spinners only animate on a terminal that can draw.

## Testing it

Everything that needs a terminal is a field of the dashboard: the keys are a reader, the size a
function, the wait for "any key" a function, and the commands go through a runner. That is what
lets `internal/cli/tui_test.go` run the whole dashboard with no terminal and no Docker:

- **Computing** —cutting columns to the width, naming a key from its bytes, scrolling, composing a
  frame— is tested with made-up input.
- **Emitting** is tested by asserting the sequences a frame is painted with, and that the
  dashboard refuses without a terminal.
