# Progress

The rule: **something on the screen before a hundred milliseconds**, for every operation that can
take longer than a moment.

`hm db snapshot` on a real catalogue is four minutes. `hm mysql -i` is longer. `hm clean` spends
twenty-five seconds working out volume sizes. A terminal showing a cursor gives no way to tell a
slow operation from a hung one.

## How it is kept

The label is printed by the same statement that begins the work:

```bash
hm_start "Saving the database as 'before-upgrade'..."
# ... the dump runs ...
hm_stop 0
```

There is nothing to measure and nothing to get wrong: the line cannot arrive late, because it
arrives before the work does.

## The three shapes

| | For | What it does |
|---|---|---|
| `hm_step` | Work that prints its own output — Compose, Composer, Magento | One line, now |
| `hm_start` / `hm_stop` | Silent work whose own output belongs on stdout | Line, spinner, result with elapsed time |
| `hm_working` | Silent work whose output only matters if it fails | Runs it captured, prints the capture only on failure |

## Nothing animates unless somebody is watching

The spinner is escape sequences. In a pipe they are noise, in a log file they are a mess, and in
`--json` they would corrupt the document. So one function decides, and it answers no unless:

- stdout is a terminal, and
- the output format is human, and
- `TERM` is usable, and
- `NO_COLOR` and `HM_NO_COLOR` are unset, and
- the run is interactive (`HM_NON_INTERACTIVE` unset).

`HM_NO_PROGRESS=1` turns it off on its own, for anybody who wants the lines without the movement.

When the answer is no, the same information arrives as two plain lines — start and finish, with
the elapsed time. A CI log gets more from that than from a carousel of carriage returns.

## Elapsed time

Shown above two seconds. "Done" says nothing; "done in 4m12s" tells you what to expect next time,
which is the difference between a tool that feels slow and one that is honest about being slow.

## Where it is used

Database snapshots and restores, imports, freezing and cloning data directories, working out what
`hm clean` would free, and each of the checks `hm verify` runs. Those are the places that used to
be silent for minutes.

Commands that already stream their own output — `hm start`, `hm composer`, `hm magento` — are left
alone. What they needed was a line *first*, not a spinner over the top of somebody else's output.
