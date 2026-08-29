# Say something before a hundred milliseconds

## Why

`hm db snapshot` on a real catalogue is four minutes of nothing. `hm mysql -i` is longer.
`hm clean` spends twenty-five seconds working out volume sizes without a word, and cloning a
data directory is however long a few hundred megabytes take. The terminal shows a cursor, and the
person in front of it has no way to tell a slow operation from a hung one.

The interesting part of this is not the spinner. It is the rule: **something on the screen before
a hundred milliseconds**, every time, for every operation that can take longer than a moment. A
spinner is one way of keeping that promise, and it is the wrong way in a log file — which is why
the second half of the rule is that nothing animates when nobody is watching.

## What Changes

- A progress component with three shapes: a line printed now, a line with a live spinner for work
  that is silent, and a wrapper that runs a silent command and shows its output only if it fails.
- **Nothing animates unless stdout is a terminal.** Not in a pipe, not in `--json`, not under
  `NO_COLOR`, not on a dumb terminal, not when the run is non-interactive. In those cases the same
  information arrives as two plain lines.
- Elapsed time on anything that took more than a couple of seconds, because "done" without a
  number teaches nobody how long to expect next time.
- Applied where the silences actually are: database snapshots and restores, imports, freezing and
  cloning data directories, working out what `hm clean` would free, and the checks `hm verify`
  runs.
