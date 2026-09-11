# Proposal: `Capture` in the engine, not yet on the seam

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis. Same rationale as `debug-off-in-go` and its predecessors.

Unit 1 of three (exploration §E). Units 2 (host-detection adapter) and 3 (the `debug-on` handler,
which is also where `Capture` joins the `commands` seam) are separate cycles.

## Intent

`debug-on` and `create-project` both need to read what a command printed inside an arbitrary
service. Nothing in the engine can: `Exec` streams to writers fixed at `engine()` construction
(`internal/cli/engine.go:50`), and `dockerd.Runner.Capture` (`adapters/dockerd/exec.go:72-106`) has
the right shape but takes a **container id**. The only resolution that turns a service into an id
correctly — `Database.container` (`app/database.go:121-145`) — is hardcoded to `db`.

This cycle builds that capability in `dockergento/app` and exposes it on the facade. It stops
there, and Decision 3 is why.

## Scope

### In Scope

- A **generalised service→container resolution** in `dockergento/app`, parameterised on service;
  `Database.container` delegates to it.
- An `app` type that owns **`Capture`**: run the command, keep stdout, keep stderr apart, answer
  with the output and the container's own status.
- **`(*dockergento.Engine).Capture(dir, service string, command []string) (string, int, error)`** —
  `Resolve`, wire the adapters, delegate, choose where stderr goes.
- Tests in `dockergento/app` using the fakes that package already has.

### Out of Scope / Non-goals

- **`internal/cli`, entirely.** No `commands` method, no `fakeEngine` change, no amended doc
  comment, no touched compile assertion. See Decision 3.
- Any caller, any `run.go` case, any `MIGRATION.md` counter change.
- The host-detection adapter and the `debug-on` handler (units 2 and 3).
- **`toolinfo.Reader.Xdebug`** (`adapters/toolinfo/tooling.go:67-124`). It already resolves `phpfpm`
  and execs into it, so a reader will notice the overlap: it is deliberately not reused and
  deliberately not fixed. Its match at `:84-85` is a hand-rolled label disjunction that hits any
  container lacking `hm.project` when the project name is empty — a defect recorded across three
  cycles. `core.Container.Key()` (`core/environment.go:111-118`) is the repo's one rule and is what
  the generalised resolution uses. Converging `toolinfo` on it is its own change.
- `--json`, output text, exit-code mapping — there is no handler.

## Decisions

### 1. `Database.container` is refactored to share the generalised resolution

Two copies of a resolution rule is how the `toolinfo` defect happened; a third would be worse. The
refusal is already parameterised on the service name (`app/database.go:139-144`), so `phpfpm` yields
`Service 'phpfpm' is not running` / `hm start phpfpm` with no new formatting.

**Proof it is behaviour-preserving:** the five tests in `dockergento/app/database_test.go` pass
**byte-unchanged**. They assert only observable resolve/refuse behaviour — the right id among two
projects (`:60-84`), code 5 and the hint when the db is down (`:122-133`), the client's own status
(`:135-147`) — never internals. If any needs editing, the refactor was not behaviour-preserving.

### 2. stderr passes through; the status is the container's own

| | Behaviour |
|---|---|
| stdout | buffered and returned as the `string` |
| stderr | written to a caller-supplied writer, which the facade fills with `e.errors()` (`dockergento.go:468-474`) exactly as `Engine.Dump` hands `e.options.Stderr` (`:378`). Never inside the returned string — a warning in the middle of a `php -i` buffer is a parsing bug waiting for the right image |
| non-zero status | returned as the `int`, **unmapped**. No `core.Refusal`, no code 1 |
| `error` | reserved for *we could not run it*: the code-5 resolution refusal, or a daemon/exec failure |

A caller whose command ran and failed gets whatever stdout it managed, the container's status, and a
`nil` error. `magento-commands/spec.md` (from `wrapper-exit-codes`, archived 2026-09-09) already says
"the command exits with that same status, not a mapped code". `Engine.Dump` maps to code 1 only
because it returns `error` alone and has nowhere to put a status; `Capture` has an `int`.

**Revised from the exploration**: it proposed buffering *at the facade*. With no CLI caller, logic on
the facade would be unprovable — `Engine.Capture` constructs `dockerd.Engine` and `dockerd.Runner`
concretely, so like `Engine.Dump` it needs a daemon. The buffering therefore lives in `app`, where
the fakes reach it, and the facade is thin wiring in the shape of `Engine.Anonymise`
(`dockergento.go:452-465`). The signature handed to unit 3 is unchanged.

### 3. The seam is deferred to unit 3, on purpose — it was not forgotten

The `commands` doc comment (`internal/cli/engine.go:14-16`) says the interface names what the
handlers routed through `newEngine` call **"and nothing else"**. All sixteen methods honour that
today; every one has a handler caller. Landing a seventeenth with no caller would have contradicted
a written invariant somebody put there deliberately, and the exploration's claim that this has
precedent here did not survive checking: every CLI-reachable `*dockergento.Engine` method has a
caller too (`Diagnose`, `Console`, `Templates`, `Freeze`, `RegistryState`). The nearest precedent,
`public-engine` (archived 2026-09-02), built the public facade for a web interface that did not
exist — but `internal/cli` consumed it in the same change.

So `Capture` joins `commands` in unit 3, beside the handler that calls it, and the invariant stays
true the whole way.

**What unit 3 inherits, recorded here so it is not rediscovered:**

- One line on `commands`, taking it from sixteen methods to seventeen, and no comment to amend —
  the handler arrives with it.
- **`fakeEngine` needs a data field, and it is a first.** `outcome`
  (`internal/cli/fake_engine_test.go:51-54`) carries only `status` and `err`; nothing on that seam
  has ever answered with returned data. `Capture` is also the first seam method returning data
  **and** a status, so it consumes both, in the order `Resolve` and `StopEverything` use: the
  outcome first, so a refusal stays reachable (`out.err != nil` → `"", out.status, out.err`), then
  the answer (`f.captured, out.status, nil`). `call` grows nothing — `Dir`, `Service` and `Command`
  already exist for `Exec`.
- **Two compile assertions, not one**, are what break if the interface grows and a type forgets to
  implement it: `var _ commands = (*dockergento.Engine)(nil)` (`internal/cli/engine.go:36`) fails
  `go build`, and `var _ commands = (*fakeEngine)(nil)` (`fake_engine_test.go:90`) fails the test
  build of package `cli`. **No automated test enforces the method count** — the "sixteen methods" in
  prior proposals is hand-maintained prose. Neither assertion is a risk in *this* change; both are
  unit 3's to handle.

### 4. `toolinfo.Reader.Xdebug` is neither reused nor fixed

Stated in Non-goals above with the disjunction's exact location, so the overlap does not read as an
oversight.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This change adds engine capability with no command behaviour, no output and no exit code
attached to it; `go-entrypoint`'s seam requirements are untouched because `internal/cli` is
untouched. The spec phase should confirm this rather than assume it — if it finds that
`database-access` documents `Database.container`'s resolution as `db`-specific at the spec level, a
delta belongs there, though Decision 1 keeps the observable behaviour identical.

## Approach

| Layer | Change |
|---|---|
| `dockergento/ports` | none — `ContainerRunner.Capture` already exists (`ports.go:237-252`) |
| `dockergento/adapters/dockerd` | none — `Runner.Capture` is used as is |
| `dockergento/app` | generalised resolution; `Database.container` delegates; the `Capture` type |
| `dockergento/dockergento.go` | `Engine.Capture`: `Resolve` → wire → delegate, stderr to `e.errors()` |
| `internal/cli` | **nothing** |

### What proves what

Everything this cycle ships is provable without a daemon, in the packages' own fakes — that is now
the point of the scope rather than a limitation of it.

| Claim | Proved by |
|---|---|
| Resolution picks this project's container for an arbitrary service | `app` tests over the existing `engine` fake (`app/environments_test.go:10-28`), two projects in the list |
| A service that is not running refuses with code 5, naming **that** service and `hm start <service>` — not `db` | same |
| stdout comes back as the returned string | the existing `runner` fake's `answer` (`app/database_test.go:12-48`) |
| stderr is **absent** from the returned string | the same fake's `complaint`, which it already writes to the `errors` writer (`:36`) — no new fake code |
| a non-zero status comes back unmapped, with a `nil` error | the fake's `status`, in the shape of `TestElCodigoDeSalidaEsElDeLaConsulta` (`:135-147`) |
| the refactor preserved behaviour | `dockergento/app/database_test.go`'s **five existing tests, unedited** |
| `Engine.Capture` itself | **not unit-tested**, the same as `Engine.Dump`: it wires concrete adapters. Its correctness is the wiring being one line each, and the logic sitting one layer down where it is tested. Said plainly rather than implied |

Strict TDD, RED first in `dockergento/app`.

## Size

Re-derived bottom-up, counting additions **and** deletions:

| Piece | Lines |
|---|---|
| generalised resolution in `app` (body + house doc comment) | ~30 |
| `Database.container` reduced to a delegate (~25 deleted, ~5 added) | ~30 |
| the `app` `Capture` (buffering, stderr split, doc comment) | ~30 |
| `Engine.Capture` facade wiring | ~25 |
| `app` tests, reusing the existing `engine` and `runner` fakes | ~110 |
| **Total** | **~225** |

**Removing `internal/cli` took less off than it looks.** My previous count of ~205 was itself short:
it charged the refactor only its 3-line delegate and ignored the ~25 deleted lines, and it inlined
the buffering into the facade where Decision 2 now shows it cannot be tested. The same derivation
*with* the seam included is ~250, so the deferral is worth ~25 lines — about 10%. The seam was
always the cheap part; the `app` layer and its tests are the expensive part and none of that moved.

Against the repo's measured 1.3-2x overshoot (`debug-off` estimated 150-210, landed 302) that is
**~290-450**. Applying the full factor probably double-counts the under-count I just corrected, so
the mass sits near the low end — but I will not call 400 comfortable, and the exploration's 150-220
remains optimistic. `sdd-tasks` should forecast this **Medium** and keep a split ready: slice A =
generalised resolution + `Database.container` refactor + its tests; slice B = the `app` `Capture` +
the facade + its tests. Each has a clear finish and reverts alone.

Measure against **this change's own parent** — the `release/2.0.0` tip captured with
`git rev-parse HEAD` *before* the first commit — as
`git diff --shortstat <that-hash>..HEAD -- . ':(exclude)openspec'`. Never a hash copied from an
earlier proposal.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `dockergento/app/database.go:121-145` | Modified | delegates to the generalised resolution |
| `dockergento/app/` (new or existing file) | New | the resolution and the `Capture` type |
| `dockergento/dockergento.go` | New | `Engine.Capture` |
| `dockergento/app/*_test.go` | New | the tests above |
| `dockergento/app/database_test.go` | **Unchanged** | that is the proof |
| `internal/cli/` | **Unchanged** | Decision 3 |
| `adapters/toolinfo/tooling.go` | Unchanged | stated deliberately, see Non-goals |
| `MIGRATION.md`, `bin/run`, `console/` | Unchanged | no command is ported |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The signature is wrong for unit 3, with no caller to have proved it | Med | It mirrors `Runner.Capture`'s semantics and `toolinfo`'s ergonomics; changing it is cheap while nothing calls it. This is the cost the deferral accepts in exchange for the invariant |
| `Engine.Capture` is untested wiring and drifts from the tested layer | Low | One line each; identical in shape to `Engine.Dump` and `Engine.Anonymise`, which carry the same exposure today |
| Size crosses the 400-line budget | Med | Two-slice split named above, forecast by `sdd-tasks` |
| The `Database.container` refactor drifts behaviour | Low | Five tests must pass unedited; any edit to them means it drifted |
| A reader assumes `toolinfo`'s overlap, or the missing seam method, was forgotten | Med | Both named explicitly — Non-goals and Decision 3 |
| Unit 3 grows larger for absorbing the seam work | Low | ~25 lines, itemised in Decision 3 so it can be planned rather than discovered |

## Rollback Plan

One or two commits on `release/2.0.0`, additive only. `git revert` removes `Engine.Capture`, the
`app` `Capture` and the generalised resolution; `Database.container` returns to its inline form and
its five tests still pass. Nothing is routed and `internal/cli` is untouched, so no user-visible
command changes and there is nothing to restore.

## Dependencies

None. **No impact on existing projects and no migration**: no command changes, no state, no files
written, no `bin/run` path touched.

## Success Criteria

- [ ] `internal/cli/` has **no diff**. `commands` still has sixteen methods and its doc comment is
      unchanged and still true.
- [ ] `dockergento/app/database_test.go`'s five tests pass **byte-unchanged**.
- [ ] Resolution against a service that is not running returns a code-5 refusal naming that service
      and `hm start <service>` — not `db`.
- [ ] A command that runs and exits non-zero returns its stdout, its own status and a `nil` error;
      its stderr reached the error writer and is **absent** from the returned string.
- [ ] `(*dockergento.Engine).Capture` exists with the signature unit 3 will put on the seam.
- [ ] `go build ./...`, `go test ./... -short`, `tests/run.sh unit`, `gofmt -l ./cmd ./internal`
      empty, `go vet ./...` clean.
- [ ] `MIGRATION.md` still says `33 de 65`.
