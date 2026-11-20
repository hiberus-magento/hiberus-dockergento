## 6. I/O and Error Handling

Where it lives:

| Concern | Code |
|---|---|
| Exit codes and the JSON envelope | `dockergento/contract/contract.go` |
| Typed refusals | `dockergento/core/refusal.go` (`core.Refusal`) |
| Reporting an error, choosing the format | `internal/cli/output.go` (`failure`, `isTerminal`), `internal/cli/orchestrate.go` (`report`), `internal/cli/globals.go` (`globals`, `wanted`) |
| Colours | `internal/cli/palette.go` |
| Progress | `internal/cli/progress.go` |
| Questions and lists | `internal/cli/ask.go`, `internal/cli/select.go` ([questions](../docs/questions.md)) |
| Interruptions | `internal/cli/interrupt.go` (`share`, `tunnel`), `internal/cli/project_command.go`, `internal/cli/tui.go` |
| Docker and project checks | `dockergento/adapters/dockerd/daemon.go`, `internal/cli/engine.go` (`projectOr`), `dockergento/app/container.go` (`resolveContainer`) |

### 6.1 Error Handling Philosophy

The tool is **fail-fast**: an operation stops at the first error, before it can leave an
environment half-changed, and the error says what went wrong and what to do about it.

In Go that is not a shell option but a type. The layers below the CLI return errors; the ones
they shape on purpose are `core.Refusal`:

```go
type Refusal struct {
    Kind    string // machine-readable type: "service_not_running", "docker_unavailable", ...
    Code    int    // the exit code, from the contract
    Message string // what happened
    Hint    string // what to do about it, usually a command to run
}
```

A use case that cannot proceed returns a refusal; nothing below the CLI prints an error or calls
`os.Exit`. The handler turns the error into output and an exit code through `report`:

```
err == nil                     → exit 0
a list was aborted             → "Nothing was chosen." on stderr, exit 130
err is a core.Refusal          → its message and hint (or the JSON error), its own code
any other error                → "docker_failed", exit 3
```

Errors are never swallowed. Where a failure is expected and is part of the logic (a container
that may not exist, an optional file), the code asks the question directly — the Docker API, the
filesystem port — instead of running something and ignoring its exit code.

### 6.2 Exit Codes

The exit codes are a contract that scripts and agents branch on (`dockergento/contract`):

| Code | Constant | Meaning |
|------|----------|---------|
| `0` | `ExitOK` | Success |
| `1` | `ExitError` | Generic error |
| `2` | `ExitUsage` | Invalid arguments, unknown command, or a question non-interactive mode cannot answer |
| `3` | `ExitDocker` | Docker daemon unavailable, or Docker failed |
| `4` | `ExitProject` | Not a configured Dockergento project |
| `5` | `ExitService` | A required service is not running |
| `6` | `ExitBlocked` | Refused on purpose: from a git worktree, a port already taken, ... |
| `130` | `exitInterrupted` | A question or list was aborted |

Commands that run something inside a container (`exec`, `magento`, `composer`, ...) return **that
command's own exit code**: flattening it would break everything that branches on it. A project
command's exit code is `hm`'s.

A project command receives the codes as `HM_EXIT_OK`, `HM_EXIT_ERROR`, `HM_EXIT_USAGE`,
`HM_EXIT_DOCKER`, `HM_EXIT_PROJECT`, `HM_EXIT_SERVICE` and `HM_EXIT_BLOCKED`.

### 6.3 Output

#### 6.3.1 Text or JSON

The format is decided once, in `globals` and `wanted`:

1. `--json` or `--no-json`, when given, win.
2. Otherwise, a terminal gets readable text and anything else (a pipe, a file, `/dev/null`) gets
   JSON. "A terminal" is asked of the terminal itself, not of the file mode: `/dev/null` is a
   character device and must not look like one.

Every JSON answer is one envelope, whichever door it came through (terminal, HTTP, MCP):

```json
{ "schema_version": 1, "command": "describe", "ok": true, "data": { ... } }
{ "schema_version": 1, "command": "start", "ok": false,
  "error": { "code": 3, "type": "docker_unavailable",
             "message": "Docker is not running", "hint": "Start Docker and try again" } }
```

Commands whose output *is* data are never wrapped: `mysqldump`, `mysql`, `copy-from-container`
and the passthrough commands (`exec`, `bash`, `magento`, `composer`, `npm`, `n98-magerun`,
`grunt`, `test-*`, `cloud`, `docker-compose`).

#### 6.3.2 The streams

- **stdout** carries the answer: the text a person reads or the JSON document.
- **stderr** carries everything else: errors in both formats (so `hm <command> > file` never
  captures a failure as output), questions, and — in JSON mode — progress and decoration.

In text mode an error is printed as:

```
<message>
  → <hint>
```

#### 6.3.3 Colour

`coloured()` in `internal/cli/palette.go` decides, in this order: `--no-color` (which sets
`HM_NO_COLOR`) or `NO_COLOR` set and non-empty, never; `TERM=dumb` or empty never; `FORCE_COLOR` or
`CLICOLOR_FORCE` always; otherwise only when stdout is a terminal. Colour marks state — `good`
(green), `bad` (red), `warning` (yellow), `section`/`link` (blue), `header` (bold white),
`table` (cyan) — and the text is identical without it: removing the colour never removes
information.

A project command receives the palette as the escaped strings the shell tool used (`BLUE`,
`GREEN`, `RED`, `YELLOW`, `COLOR_RESET`, ...), empty when colour is off.

#### 6.3.4 Progress

`begin(label, ...)` in `internal/cli/progress.go` announces a step and returns the function that
ends it, saying whether it succeeded and, past two seconds, how long it took. It animates only when there is a terminal,
the output is text, colour is allowed and `TERM` can draw; otherwise it prints one line. In JSON
mode it writes to stderr, because stdout carries the document.

### 6.4 User Input

#### 6.4.1 Questions

`ask(text, suggestion)` in `internal/cli/ask.go`:

- With `--yes` or `HM_NON_INTERACTIVE=1` it never waits: a question with a suggestion takes it,
  one without fails with an actionable error (exit `2`) naming the option to pass instead.
- Otherwise it writes the question to **stderr** and reads one line from stdin, a byte at a time
  so that answers piped in for later questions are not swallowed. An empty answer takes the
  suggestion. An answer piped in is still an answer: `printf 'n\n' | hm masquerade` means no.

#### 6.4.2 Lists

`choose(question, options)` in `internal/cli/select.go` picks the best way the terminal allows:

1. `fzf`, when it is installed — whoever installed it has opinions about picking from a list.
2. A selector moved with the arrow keys or `j`/`k`, Enter to take the highlighted option or the
   digit of an option to take it directly. The first option is preselected, so the safest answer
   is the one Enter gives.
3. A numbered list, when the terminal cannot be drawn on.

Escape does nothing on purpose; aborting (Ctrl-C) is `errNothingChosen`, reported as "Nothing was
chosen." with exit `130` — never treated as an empty answer, which for `down -v` would be the
wrong one.

#### 6.4.3 Input from stdin

Commands that consume data read it from stdin when it is not a terminal:

```bash
hm mysql < dump.sql
cat dump.sql | hm mysql
```

The dump is streamed into the database container's client through the Docker API
(`Database.Feed` in `dockergento/app/database.go`).

### 6.5 Validations

Each handler validates in this order, before any costly operation:

1. **Its own arguments.** An undeclared option or a missing value is a usage error
   (`invalid_argument`, exit `2`) with a hint showing the right form.
2. **The project.** `projectOr` resolves the current directory; outside a configured project it
   refuses with `project_not_configured` (exit `4`) and the hint `hm setup`.
3. **Docker.** `Daemon.Reachable()` pings the Docker API; with no answer it is
   `docker_unavailable` (exit `3`), "Docker is not running", "Start Docker and try again".
4. **The services it needs.** `resolveContainer` finds the service's container by its labels; a
   stopped or missing one is `service_not_running` (exit `5`) with the hint `hm start <service>`.
   Nothing is started behind the user's back.
5. **Safety.** From a git worktree with no environment of its own, a command that would act on
   the main environment is refused (exit `6`) unless `--force` is given for that invocation; a
   destructive step asks for confirmation or requires `--force`.

A missing file passed to `mysql -i` is reported as a warning ("No such file: ...") and the
command stops without changing anything.

### 6.6 Interruptions and Cleanup

- **Commands that hold something open** (`share`, `tunnel`) take Ctrl-C and SIGTERM from before
  anything is opened (`internal/cli/interrupt.go`): at the question nothing is closed, while
  opening the relay is closed once the opening returns, and while holding it is closed and the
  command finishes on its own. A container left on the daemon is the failure this prevents.
- **Project commands**: the interruption belongs to the script. `hm` waits for it to finish
  rather than dying first and leaving it behind.
- **The dashboard** restores the main screen, the cursor and the terminal on the way out,
  including on Ctrl-C and SIGTERM, and redraws on SIGWINCH.
- **Passthrough commands** (`exec`, `bash`, `grunt watch`, ...) are attached to the terminal; the
  interruption reaches the process in the container.

### 6.7 Logging

The tool **keeps no logs of its own**. What happened is on the terminal, the containers' logs are
Docker's, and `hm logs [service...] [-f] [-n N] [--since T] [-t]` reads them through the Compose
library. Redirect the output when a record is needed: `hm start > start.log 2>&1`.

### 6.8 Practices

- **Return a refusal, do not print.** Below `internal/cli`, an expected failure is a
  `core.Refusal` with an actionable hint; the CLI, HTTP and MCP each render it their own way.
- **Validate early.** Arguments, project, Docker and services are checked before anything is
  changed.
- **Never wait for nobody.** Every question has a non-interactive answer or fails with exit `2`.
- **Keep stdout clean.** Only the answer goes to stdout; a JSON reader never has to parse around a
  question, a spinner or an error.
- **Keep the contract.** Exit codes, error types and envelope fields are pinned by tests (Go tests
  through `Run` with the fake engine, and `tests/integration/cli_contract_test.sh`); changing one
  is a breaking change.
