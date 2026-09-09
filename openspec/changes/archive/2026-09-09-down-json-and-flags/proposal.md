# Proposal: The two `down` options that take a value and accept none

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `retire-ported-shell-commands`, `docker-tools-in-go`, `remaining-wrappers-go-tests` and
`wrapper-exit-codes`.

## Intent

Two options in `down`'s parse loop consume a value if one is there and say nothing when it is not.

`internal/cli/down.go:31-35`:

```go
case "--rmi":
    if at+1 < len(args) {
        at++
        options.Images = args[at]
    }
```

No `else`. `hm down --rmi` tears the environment down without removing a single image and never
mentions the flag it ignored. `-t` / `--timeout` (`:36-48`) has the same missing branch — and it is
sharper there, because the *same case* already refuses a non-numeric value with `invalid_argument`
and a hint (`:41-45`). The code knows how to say no to a bad value. It never learned to say no to a
missing one.

Both live entirely above `projectOr` (`:55`) and above the engine call (`:61`), so neither needs
anything from the `commands` seam.

## The convention this converges on

`missing_value` already exists and is used twice, in one shape. `internal/cli/web.go:53-56`:

```go
if at+1 >= len(args) {
    return failure(stderr, jsonOutput, "web", exitUsage, "missing_value",
        "--port needs a number", binaryName()+" web --port 8420")
}
```

`internal/cli/orchestrate.go:102-105`, the same, generic in the flag name:

```go
if at+1 >= len(args) {
    return failure(stderr, jsonOutput, "logs", exitUsage, "missing_value",
        argument+" needs a value", binaryName()+" logs --tail 100")
}
```

So this is not a design question. It is two call sites that never got the shape their neighbours
have.

`failure()` (`output.go:59-75`) writes to **stderr in both modes**: the JSON envelope at `:63`, the
legible message and hint at `:69-72`. Nothing reaches stdout on either path, which is what makes the
three observables below assertable per case.

## Scope

### In Scope

- **The `--rmi` guard** (`down.go:31-35`): `at+1 >= len(args)` refuses with `exitUsage`,
  `missing_value`, a message naming the flag and a hint in `binaryName()+" down --rmi all"` form.
- **The `-t` / `--timeout` guard** (`down.go:36-48`): the same guard, placed *before* the existing
  `strconv.Atoi`, so the missing case and the non-numeric case stay two different answers with two
  different `type`s. Hint reuses the existing `binaryName()+" down -t 30"` (`:44`).
- **Go tests for both**, in a new `internal/cli/down_test.go`, each with a text subtest and a
  `--json` subtest, asserting all three observables. These call `down(...)` directly with buffers:
  the refusal returns during parsing, so no engine, no fake and no Docker is involved.
- **The delta for `environment-lifecycle`** (below).

### Non-goals (Out of Scope)

- **The `commands` seam.** `Down` is not on the interface (`engine.go:17-30`) and `down.go:61` calls
  the concrete `engine(...)`. Growing the seam is `down-go-tests`' work and is already designed
  verbatim at `archive/2026-09-05-down-and-set-host-go-tests/design.md:291-296`. **Reason**: nothing
  in this change reaches the engine, so it would pay for a seam it does not use — and pulling it
  forward supersedes a change that is already specified.
- **The `--json` stream defect at `down.go:67-71`.** A declined confirmation writes
  `"Nothing was destroyed.\n"` to stdout before the `if jsonOutput` branch is reached, so
  `hm down --json | jq` fails to parse — against `cli-output-contract/spec.md:27-30` and `:133-135`.
  **Reason**: `what == ""` happens only when a person declines, so without a fake `Down` answering
  `""` on demand it can be fixed but not tested, and `MIGRATION.md:91-93` requires tests against
  `newEngine` rather than Docker. It stays with `down-go-tests`, which owns the seam it needs. It is
  on record here as a defect (see the handoff below).
- **The same shape in `internal/cli/db.go`** — `freeze()` at `:100-103` and `snapshot()` at
  `:234-237`, both for `--name`. **Reason**: adopting them would make this change about a pattern
  rather than about `down`. Lower cost when they bite, too: the name falls back to a generated one
  instead of being dropped entirely. Reported, not adopted.
- The happy path: same argv, same options, same prompts, same success document.
- `choose()` prompting the real terminal under `--json` (`select.go:32-52`) — pre-existing and
  adjacent to defect 3, not to these two.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `environment-lifecycle`: **"Removing an environment is Compose's own command"**
  (`spec.md:145-157`) — "The tool SHALL do what Compose does, and SHALL NOT remove more than it was
  asked to." It gains one scenario: an option that takes a value, given none, is a usage error
  rather than a silent drop. Its neighbour "Deleting the data is asked about" (`:118-144`) is not
  touched.

### Governing but not modified

- `cli-output-contract`: "Errores estructurados" (`spec.md:42-56`) and "Códigos de salida estables"
  (`:69-77`, table at `:74-76`) already require the error envelope on stderr, no error on stdout,
  the legible message in text mode, and exit 2 for invalid arguments. The new refusals **conform**;
  restating them as scenarios would be spec noise.
- `environment-orchestration:171-174` ("An option that needs a value is given without one → it is a
  usage error") is the same rule already written down for `logs`. It is the precedent, not a delta.
- `go-entrypoint:192-201` ("A ported command's engine interaction is provable without Docker") is
  **not** satisfied for `down` by this change and is not amended: these refusals return before the
  engine call and prove nothing about `Down`. That coverage stays owed by `down-go-tests`.

## Approach

| Area | What happens |
|---|---|
| `internal/cli/down.go:31-35` | The `web`/`logs` guard, verbatim shape: `if at+1 >= len(args) { return failure(stderr, jsonOutput, "down", exitUsage, "missing_value", "--rmi needs a value", binaryName()+" down --rmi all") }` |
| `internal/cli/down.go:36-48` | The same guard, above the existing `strconv.Atoi`, hint `binaryName()+" down -t 30"` |
| `internal/cli/down_test.go` (new) | Two RED tests, each a text and a `--json` subtest: exit code, stderr content, stdout empty |
| `openspec/specs/environment-lifecycle` | One scenario in the delta |

Strict TDD: RED first. No seam change, no new fake behaviour, no change to `failure()`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/down.go:31-48` | Modified | Two guards added |
| `internal/cli/down_test.go` | New | Two cases × (text, `--json`) |
| `openspec/specs/environment-lifecycle` | Modified | Delta spec, one scenario |

## Delivery

One work unit, measured with
`git diff --shortstat <parent>..HEAD -- . ':(exclude)openspec'` against **this change's own parent**
at the time it is applied. A fixed hash written here goes stale the moment another change lands
first, so the parent is named, never hardcoded.

Estimated at **~120-180 lines**, and that estimate is deliberately not the exploration's ~55-75. The
sibling `wrapper-exit-codes` estimated 50-90 and came in at **184**, because the specs ask for three
observables per failing path and each case carries a text and a `--json` subtest. The same density
applies here: eight lines of production code, the rest tests and their scaffolding. Comfortably
inside the 400-line budget either way — **no delivery decision is needed before apply.**

## The handoff `down-go-tests` inherits

Its carried design at `archive/2026-09-05-down-and-set-host-go-tests/design.md:286-343` was written
before this change existed and plans to **pin** what this change treats as defects:

- `:308` — `TestWhatDownPrints`, row `result:"" with --json`, pins the stdout-prose defect as
  "found, not fixed, and the assertion says so in a comment". **That plan must be replaced by a
  fix.** A test written to assert a bug is how a bug becomes a requirement, and this proposal is
  already on record calling it one (`cli-output-contract:27-30`, `:133-135`).
- `:306` and `:340-342` — `TestWhatDownAsks` rows for `--rmi` last and `-t` last pin
  `core.DownOptions{}`, "silently ignored… pinned rather than fixed". **This change fixes them**, so
  those rows must arrive as refusals or not at all; `down-go-tests` extends the `down_test.go` this
  change creates rather than authoring it.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| **Behaviour change with a blast radius**: `hm down --rmi` today still destroys the environment, minus the images; afterwards it does nothing and exits 2. A script relying on the silent degradation starts failing | Certain | It is the point of the change, and it moves *toward* shell parity: `console/commands/down.sh:27-29` is `destroy() { $DOCKER_COMPOSE down "$@" }` — no flag parsing of its own, arguments straight to Compose, which already refuses a trailing `--rmi` through its own validation. The shell twin errors on this input today. Recorded here and in the delta rather than shipped as a surprise |
| A caller depended on the silent drop | Low | `down()` is reached only from `run.go`'s switch; its return value is the process exit code and feeds no further Go logic. Nothing under `tests/` asserts success for a bare `--rmi` |
| The message or code differs from Compose's | Certain, accepted | Parity here is "it refuses", not "it refuses identically". `hm` answers with its own `missing_value` envelope and exit 2, as `web` and `logs` already do |
| Defect 3 ships unfixed while being on record | Med | The handoff section above makes the correction explicit for the next cycle instead of leaving the archived plan to be followed as written |

## Rollback Plan

One commit on `release/2.0.0`; `git revert` restores both guards, `down_test.go` and the delta.
Nothing else depends on it.

## Dependencies

None. **Existing projects need no migration**: no configuration, template, prompt, success message
or success document changes. The only user-visible difference is that two malformed invocations stop
being accepted — a script passing a bare `--rmi` or `-t` was already not getting what it asked for.

## Success Criteria

For each of `hm down --rmi` and `hm down -t` given as the last argument, all three observables:

- [ ] **Exit code**: 2 (`exitUsage`), for both flags and both long/short spellings.
- [ ] **Text output**: stderr carries the legible message naming the flag plus the `  → ` hint
      (`output.go:69-72`); stdout is empty; the environment is **not** destroyed.
- [ ] **`--json` output**: stderr carries exactly one envelope with `ok: false`,
      `error.code` = 2, `error.type` = `missing_value`, a message and a hint — parseable by `jq`;
      stdout is empty.
- [ ] A **non-numeric** timeout still answers `invalid_argument` with its own message
      (`down.go:42-44`) — the two cases stay distinguishable by `error.type`.
- [ ] The happy path is untouched: `--rmi all`, `-t 30`, `-v`, `--remove-orphans` and their
      combinations produce the same `core.DownOptions` and the same output as before.
- [ ] `go test ./... -short`, `tests/run.sh unit`, `gofmt -l ./cmd ./internal` empty,
      `go vet ./...` clean.
- [ ] Manual verification on a real project: `hm down --rmi` refuses and leaves the containers
      running; `hm down --rmi all` still destroys and removes the images.
