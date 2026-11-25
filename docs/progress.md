# Progress

The rule: **something on the screen before a hundred milliseconds**, for every operation that can
take longer than a moment.

`hm db snapshot` on a real catalogue is four minutes. `hm mysql -i` is longer. `hm clean` spends
twenty-five seconds working out volume sizes. A terminal showing a cursor gives no way to tell a
slow operation from a hung one.

## How it is kept

The use cases in `dockergento/app` say what they are doing through two callbacks the engine is
handed (`dockergento.Options`), and know nothing about terminals:

| Callback | For | Example |
|---|---|---|
| `Announce(message)` | A step whose work prints its own output, or none worth timing | `Starting containers in detached mode` |
| `Progress(label)` → `done(ok, note)` | Silent work long enough to worry about: it begins, then ends well or badly | `Restoring 'before-upgrade'...` |

The command line builds both in one place, `reporting` in `internal/cli/progress.go`. The label is
said by the same call that begins the work, so it cannot arrive late.

Where each one is used: setup and rebuild (`app.Operator`: starting, rebuilding, the proxy, the
permissions after a start), database snapshots, restores and imports (`app.Snapshots`,
`app.Importer`), freezing and cloning data directories (`app.Templates`), and each check `hm verify`
runs. `hm magento indexer:reindex` and the rest of the commands that run a program in a container
print that program's own output, which is its progress: they have no steps of the tool's own.

## For a person: lines and a spinner

`Announce` prints the message. `Progress` prints the label, a spinner while the work runs when
somebody is watching, and then the label again with `done` (or the note the use case gives) or
`failed`, plus the elapsed time above two seconds. "Done" says nothing; "done (4m12s)" tells you
what to expect next time.

Under `--json`, stdout carries the document, so all of it goes to stderr instead.

### Nothing animates unless somebody is watching

The spinner is escape sequences. In a pipe they are noise, in a log file they are a mess, and in
`--json` they would corrupt the document. So one function, `animates`, answers no unless:

- stdout is a terminal, and
- the output is text, not `--json`, and
- `TERM` is set and is not `dumb`, and
- `NO_COLOR` and `HM_NO_COLOR` are unset, and
- the run is interactive (`HM_NON_INTERACTIVE` unset).

`HM_NO_PROGRESS=1` turns it off on its own, for anybody who wants the lines without the movement.
When the answer is no, the same information arrives as two plain lines, start and finish.

The line under a copy between this machine and the php container (`hm copy-to-container`, the macOS
Composer dance) follows the same reading, on stderr, and only on a terminal.

## For a program: `--progress=jsonl`

```bash
hm --progress=jsonl --json setup -p=shop
hm db restore before-upgrade --progress=jsonl
```

A global option, accepted before or after the command name (after it, not for the commands whose
arguments belong to the program they run, like `magento` or `exec`). It writes the same steps to
**stderr** as one JSON object per line, in place of the lines and the spinner:

```json
{"phase":"setup","step":"Starting containers in detached mode","pct":null,"elapsed":0.4}
{"phase":"db restore","step":"Restoring 'before-upgrade'...","pct":null,"elapsed":1.2}
{"phase":"db restore","step":"Restoring 'before-upgrade'... done","pct":null,"elapsed":95.7}
```

| Key | What it is |
|---|---|
| `phase` | The command, with its subcommand for the families: `setup`, `rebuild`, `db snapshot`, `db restore`, `worktree add` |
| `step` | What the text output would have said, one event per line of it. A timed step is two events: its label, then the label with `done`, the use case's note, or `failed` |
| `pct` | How far through, from 0 to 100, or `null` when unknown — which today is every step: none of them knows its total |
| `elapsed` | Seconds since the command started, to a tenth |

- stdout does not change: under `--json` it is still the one envelope; in text mode the steps leave
  it for stderr and the rest of the output stays.
- The copy line is not drawn while it is on.
- Only the tool's own steps become events. A program the command runs (Compose, Composer,
  `bin/magento`) may still write its own lines on stderr: skip any line that does not parse as an
  object with a `phase`.
- `--progress=text` is the default, and any other value is refused by the command as an unknown
  option (exit 2).
- Without the option, stdout and stderr are exactly what they were.

## In the MCP server

The long tools (`reindex`) send `notifications/progress` while they run when the request asked for
them with a `progressToken`: see [the MCP page](mcp.md#progress).
