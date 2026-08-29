# Design

## The rule is the feature

Every operation that can exceed a moment prints its label **before it starts**, not after. That is
what makes the hundred milliseconds achievable without measuring anything: the line is printed by
the same statement that begins the work.

The spinner is then a detail — it says the process is still alive, and it costs one background
loop redrawing a single line.

## Three shapes, because there are three situations

| Shape | For | What it does |
|---|---|---|
| `hm_step` | Work that prints its own output (Compose, Composer, Magento) | One line, now. Nothing else |
| `hm_start` / `hm_stop` | Silent work whose output must stay on stdout (a dump being piped into a file) | Line, spinner, final line with elapsed |
| `hm_working` | Silent work whose output is only interesting when it fails | Runs it captured, and prints the capture only on failure |

`hm_step` is the one that will be used most, and it is deliberately trivial. Most long commands
here already print something — what they lacked was printing it *first*.

## Nothing animates unless somebody is watching

An animation is escape sequences. In a pipe they are noise, in a log file they are a mess, and in
`--json` they corrupt the document. So the decision is made once, in one function, and it answers
no unless stdout is a terminal, the output format is human, `TERM` is usable, and the run is
interactive.

When it answers no, the same information is printed as two plain lines — start and finish, with
the elapsed time. A CI log gets more from that than from a carousel of carriage returns.

## The animation itself

A background loop redrawing one line every tenth of a second, killed when the work ends and its
line erased. Braille frames when the locale says UTF-8, ASCII otherwise: a terminal that shows
`â ‹` instead of a spinner is worse than one that shows `-`.

Bash 3.2 has no fractional `read -t`, but `sleep 0.1` is fine, and this is the one place in the
tool where a subprocess per tenth of a second is acceptable — it is one `sleep`, not one `jq`.

## Elapsed time, and why it is always there

"Done" says nothing. "Done in 4m12s" tells the person what to expect the next time, which is the
difference between a tool that feels slow and one that feels honest about being slow. It is shown
above two seconds, so that nothing gains a stopwatch for finishing instantly.
