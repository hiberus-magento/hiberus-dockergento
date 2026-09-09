# Exploration: down-json-and-flags

Change: `down-json-and-flags`
Engram topic: `sdd/down-json-and-flags/explore` (project `hiberus-dockergento`, observation #603)
Date: 2026-09-09

`hm down` has three defects of its own and sits on top of a fourth that is not its. They do not all
cost the same to fix, and that is the decision this exploration exists to put in front of somebody.

## The three

**1. `--rmi` without a value is dropped in silence.** `internal/cli/down.go:31-35`:

```go
case "--rmi":
    if at+1 < len(args) {
        at++
        options.Images = args[at]
    }
```

No `else`. `hm down --rmi` destroys the environment without removing a single image and says nothing
about the flag it ignored.

**2. `-t` / `--timeout` without a value, the same.** `:36-48`. What makes this one sharper is what
sits inside the very same case: a *non-numeric* timeout is refused properly, with `invalid_argument`
and a hint. The code already knows how to say no to a bad value. It just never learned to say no to
a missing one.

**3. A declined confirmation writes prose onto the JSON stream.** `:67-71`:

```go
if what == "" {
    fmt.Fprint(stdout, good("Nothing was destroyed.\n"))

    return exitOK
}
```

The `if jsonOutput` below it is reached only when `what != ""`. `document()` always writes to stdout
and `failure()` always to stderr, so this is not "a stray line of text": it puts prose on the one
stream the JSON document is supposed to have to itself. `hm down --json | jq` fails to parse.

The contract says so in its own words, in `openspec/specs/cli-output-contract/spec.md`: success JSON
`SHALL escribirse en stdout` (`:27-30`), errors `SHALL emitirse por stderr` with `stdout no contiene
el error` (`:42-51`), and progress messages `van a stderr y no contaminan stdout` (`:133-135`).

## The fourth, which is not `down`'s

`internal/cli/engine.go:46-74` never sets `dockergento.Options.Stdin/Stdout/Stderr`. They are typed
`*os.File`, so `composelib.Orchestrator.Stdout` is nil and `o.out()`/`o.err()`
(`adapters/composelib/orchestrator.go:264-278`) fall back to the real `os.Stdout`/`os.Stderr`.

Whatever the compose-go library prints of its own accord therefore bypasses the writers every
command in `internal/cli` is handed, invisible to any test double, with or without `--json`. That is
a property of the engine's construction, not of `down`, and it is out of scope here — but it is the
reason a successful text-mode destroy looks silent from `down()`'s side while a user still sees
Compose talking.

## The convention to converge on

`missing_value` already exists and is used twice, in the same shape both times:

| Where | Shape |
|---|---|
| `internal/cli/web.go:53-56` | `if at+1 >= len(args) { return failure(..., exitUsage, "missing_value", "--port needs a number", hint) }` |
| `internal/cli/orchestrate.go:102-105` | the same, `argument+" needs a value"` |

`mysql.go:25,32` and `db.go` elsewhere already check before consuming. So defects 1 and 2 are not a
design question; they are two call sites that never got the shape their neighbours have.

## The same bug elsewhere

`internal/cli/db.go` has it twice more: `freeze()` at `:100-103` and `snapshot()` at `:234-237`, both
for `--name`. Lower cost when it bites — the name falls back to a generated one instead of being
dropped entirely — but it is the same shape. Reported, not adopted: widening this change to `db`
would make it about a pattern rather than about `down`.

## The shell precedent

`console/commands/down.sh:27-29` is `destroy() { $DOCKER_COMPOSE down "$@" }` — it does no flag
parsing of its own and hands the arguments to Compose, which refuses a trailing `--rmi` or `-t`
through its own CLI validation. So the shell twin already errors on this input today. Fixing the Go
side moves *toward* parity, not away from it, though the message and the code differ.

The declined path (`:74-77`) is `print_info "Nothing was destroyed.\n"; exit 0` — the same words and
the same exit code as Go. The shell has no `--json` at all, so it has nothing to say about defect 3.

## The fork

`down` is routed in `run.go` but has **no test on the `internal/cli` seam**: `Down` is not on the
`commands` interface (`engine.go:17-30`) and `down.go:61` calls the concrete `engine(...)` rather
than `newEngine`. It is one of the nine commands whose shell twin cannot retire until that changes,
and a cycle for it — `down-go-tests` — is already designed, carried verbatim in
`openspec/changes/archive/2026-09-05-down-and-set-host-go-tests/design.md:286-343`.

That carried design **pins these three defects as buggy-but-asserted**. So the two changes overlap,
and which one owns the fix has to be decided rather than discovered later.

| Shape | What it covers | Seam | Lines | What happens to `down-go-tests` |
|---|---|---|---|---|
| Narrow | defects 1 and 2 | untouched | ~55-75 | proceeds unchanged; it still owns defect 3 and the seam |
| Wide | defects 1, 2 and 3 | pulled forward | ~220-240 | superseded — its carried cases assert the bug and would need rewriting |

The reason the fork exists at all: defects 1 and 2 live entirely in the parse loop, above
`projectOr` and above the engine call, so they need nothing from the seam. Defect 3 needs
`what == ""`, which in a real run happens only when a person declines the confirmation. Without a
fake `Down` to answer `""` on demand, it can be fixed but not tested — and `MIGRATION.md:91-93` says
tests go against `newEngine`, not against Docker.

**Decision (user, 2026-09-09): narrow.** This change takes defects 1 and 2 and does not touch the
seam. Defect 3 stays with `down-go-tests`, which is where the seam it needs was already designed.

One thing that decision owes the next cycle: the carried design at
`archive/2026-09-05-down-and-set-host-go-tests/design.md:286-343` currently plans to **pin** defect 3
as buggy-but-asserted. It has to fix it instead. A test written to assert the bug is how a bug
becomes a requirement, and this change is already on record saying it is one.

## Risks

1. **This is a behaviour change, not only a tidy-up.** `hm down --rmi` today still tears the
   environment down, minus the images. After the fix it does nothing and exits non-zero. A script
   that has been getting away with a bare `--rmi` starts failing loudly. That is the point, and the
   shell twin already behaves that way, but it belongs in the change's record rather than in a
   surprise.
2. `interactive` comes from `HM_NON_INTERACTIVE`, and `choose()` (`select.go:32-52`) prompts against
   the real terminal rather than through the `jsonOutput`-aware writers — so `hm down -v --json` on a
   TTY still shows a picker. Pre-existing and adjacent; fixing defect 3 does not touch it.
3. Nothing depends on the silent drop that this exploration could find.

## Size

Narrow: ~55-75 lines. Wide: ~220-240. Both inside the 400-line budget; they differ in what they
leave behind, not in whether they fit.
