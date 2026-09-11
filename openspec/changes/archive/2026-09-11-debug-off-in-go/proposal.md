# Proposal: `debug-off` stops going through bash

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as
`docker-tools-in-go` and its predecessors.

## Intent

Five cycles in a row added Go tests to commands already wired in Go. This one ports again, and it is
**step one** of `MIGRATION.md:62`'s three-step rule — the first port in six cycles.
`console/commands/debug-off.sh` is nine lines and needs no new engine surface: `Exec` and `Compose`
are already on the `commands` interface (`internal/cli/engine.go:17-34`).

**Porting it frees nothing else, and that is said here so nobody expects it to.** `exec.sh` keeps its
live callers — `debug-on.sh` (six sites) and `create-project.sh` (`:43`, `:44`, `:49`, `:57`, `:60`).
Both get their own cycle.

## Scope

### In Scope

- **`debug-off` in Go**: the sed that re-comments `zend_extension` in
  `/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini` inside `phpfpm`, then `docker compose restart
  phpfpm`, then the success line `  xdebug disabled\n` (two leading spaces) in green — `print_info`
  is `GREEN` (`console/components/print_message.sh:58-60`), which is `good()`
  (`internal/cli/palette.go:68`).
- One `case "debug-off"` in `internal/cli/run.go`'s `Run()` switch and one handler beside `varnish`
  in `internal/cli/wrappers.go`.
- **`MIGRATION.md`**: row `shell` → `go` (`:260`), counter `32 de 65` → `33 de 65` (`:21`), in the
  same commit as the `case` line (`tests/unit/migration_status_test.sh` checks that direction —
  `go-entrypoint/spec.md:130-135`).
- The Go seam test in the same commit, in the house pattern.
- Arguments stay ignored, as `debug-off.sh` ignores `"$@"` and as the routed `varnish` already does.

### Out of Scope

- **`debug-on`** (38 lines) — it needs a new `Capture` method on `commands`, a generalisation of
  container resolution, and a host-detection adapter (WSL, `docker0`) that is new Go rather than
  composition. Its own cycle.
- **`create-project`** (119 lines, interactive, writes host files). Its own cycle.
- **Deleting `console/commands/debug-off.sh`.** That is step three, and it is spec-legal to defer:
  "A command proved in Go has no shell twin" (`go-entrypoint/spec.md:910-919`) names fifteen commands
  by name, and `tests/unit/command_safety_test.sh:64-66` records that "twenty routed commands
  legitimately still have a file". Nothing reaches `debug-off.sh` by path; only `bin/run` does.
- `exec.sh`, `start_service_if_not_running.sh`, `toolinfo.Reader.Xdebug` — all untouched.
- Any change to `bin/run`, `data/command_descriptions.json:239-243` (stays `tools` / `caution`), or
  any bash suite. **There is no bash test for `debug-off`**, so step one has none to port.

## Decisions

### 1. The restart: replicate literally (a), not unify through `Restart` (b)

The twins drift — `debug-on.sh:26` uses `restart.sh phpfpm`, `debug-off.sh:8` a bare
`$DOCKER_COMPOSE restart phpfpm` — and the reason was never written down. Chosen: **(a)**,
`engine.Compose(project.Root, []string{"restart", "phpfpm"})`.

| | (a) literal `Compose` | (b) unify through `Restart` |
|---|---|---|
| New engine surface | none — `Compose` is a raw passthrough already on the interface | none — `Restart` is too |
| What it runs | compose's own `restart`: the container is kept | `Operator.Restart` (`app/orchestrate.go:237-247`) = worktree refusal + `Stop` + `Start`, which is `up` and may recreate |
| Worktree | unchanged | **refuses with code 6** where today it works |
| Linux | nothing | `afterStarting()` hands off to `hm post-start` (`:113-121`) |
| Output | nothing | announces "Starting containers in detached mode" (`:85`) |

The evidence that settles it, all of it checkable:

1. `debug-off` is **not** in `hm_alters_environment` (`console/helpers/worktree.sh:85-92`, pinned by
   `tests/unit/worktree_test.sh:59-64`), so `bin/run:241` does not refuse it from a worktree today.
   `Operator.Restart` calls `refuseFromAnUnregisteredWorktree` unconditionally — (b) would invent a
   refusal nobody asked for. `Operator.Compose`'s own comment (`app/orchestrate.go:326-327`) already
   records this exact reasoning for the passthrough.
2. `Operator.Restart`'s doc (`:231-236`) says it is **deliberately not** compose's `restart`, because
   it must pick up compose-file changes. `debug-off` changes a file *inside the container*, not the
   compose file. A recreating restart is the wrong primitive for an edit that lives in the container
   filesystem; compose's `restart` cannot recreate, and that is a feature here.
3. The cautionary precedent runs the other way: `varnish-on`'s port silently dropped a shell exit
   code (`MIGRATION.md:169-176`). A step-one port pays for parity, not for tidiness.

**If this pick turns out wrong** — a report that `hm debug-off` leaves xdebug loaded, or that
phpfpm does not pick the ini up — look at, in order: `dockergento/app/orchestrate.go:231-247` (what
`Restart` really does, versus what compose's `restart` does), `dockergento/dockergento.go:305-318`
(the passthrough's files and environment), and `adapters/toolinfo/tooling.go:67-124`, which already
greps `^zend_extension` for `describe`/`doctor` and is the cheapest oracle for "did it actually turn
off". The asymmetry then gets **explained** in a comment rather than erased — that is the deliverable
either way.

### 2. The defensive start does not survive — disclosed behaviour change

The exploration (§B) is wrong on a detail, and the correction changes the answer:
`start_service_if_not_running.sh` does **not** run `up -d <service>`. It probes with `compose exec -T`
and, only when the status is **exactly 1** (`:11`), runs `$COMMAND_BIN_NAME start` — the *whole*
environment, no service argument.

So `engine.Start` does not subsume it, it exceeds it. With `len(services) == 0`,
`Operator.Start` also announces, ensures the proxy, runs `checkDependenciesAreNotBound`
(`app/orchestrate.go:93-97`) and, on Linux, shells back to `hm post-start`. Making that
unconditional would drift the common path (environment already up) to protect the uncommon one, and
would cost a `commands` method for the privilege — `Start` is not on the interface;
`internal/cli/orchestrate.go:37-41` calls the concrete `engine(`. Replicating it *conditionally*
needs a service-state probe that is not on the seam either.

**Decision: drop it.** With the environment down, `hm debug-off` now fails with the status compose
exec returns, instead of silently starting the whole environment; the handler names the failing step
the way `varnish` does (`wrappers.go:155-158`). This is an acceptance criterion, not a footnote.

### 3. The stray backslash is dropped

`s/^\zend_extension/;zend_extension/g` escapes `z`, which means nothing. GNU sed treats an
unrecognised escape as the literal character, so the pattern is already `^zend_extension`. The Go
port writes it plainly, with a comment recording that the shell had `\z` and that it was inert, so
the drop is never re-read as a transcription slip.

Safe in both directions: if some image's sed *did* give `\z` a meaning, the shell's `debug-off` has
been silently matching nothing on that image, and dropping the backslash fixes it. The rest of the
argv is carried verbatim, including the redundant `-e`: `-e` is POSIX-specified and redundant,
`\z` is undefined. One deviation, deliberate, pinned by the test.

### 4. What the seam test asserts

`answering(t)` (`fake_engine_test.go:275-292`), `fakeEngine`'s ordered call log, `cmp.Diff`
(`exec_test.go:42-77` is the shape). Under decisions 1-3 the whole log is:

```go
want := []call{
    {Method: "Resolve", Dir: cwd},
    {Method: "Exec", Dir: project.Root, Service: phpService, Options: terminalOptions(""),
        Command: []string{"sed", "-i", "-e", "s/^zend_extension/;zend_extension/g",
            "/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini"}},
    {Method: "Compose", Dir: project.Root, Command: []string{"restart", "phpfpm"}},
}
```

Three subtests: the log above; a failing edit (`outcomes` → `status != 0, err == nil`) recording
**only** `Resolve` and `Exec` and returning that same status; and a Go-level error on the edit
stopping before the `Compose`. The absence of a `Start` call is itself assertable, and is what pins
decision 2.

**Not provable at this seam**, and to be said rather than implied:

- That the sed pattern matches the ini's real content, and that PHP-FPM stops loading `xdebug.so`
  afterwards. The fake runs nothing. Oracle: `hm describe`'s xdebug field, or manual verification.
- That the daemon receives a compose `restart` — `Compose` is a passthrough; the seam proves argv.
- The success line's text and colour, and the `--json` envelope; those are `cli-output-contract`'s
  and are asserted by the handler's own output test, not by the call log.
- Whether the environment was down. The fake has no state for it; only the missing `Start` is visible.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `go-entrypoint`: "A ported command's engine interaction is provable without Docker"
  (`spec.md:253-266`) gains `debug-off`, with scenarios beside the varnish ones (`:284-323`);
  "The state of the migration is written down and true" (`:106-112`, `:130-135`, `:146-149`) covers
  the row and the counter. **Not** touched: "A command proved in Go has no shell twin"
  (`:910-941`) — `debug-off.sh` stays.
- `magento-commands`: "A wrapper runs what it says it runs" (`spec.md:59-63`) gains one scenario for
  the xdebug ini edit failing inside the php container, in the shape of `:76-88`. The spec phase
  decides whether the `--json` success payload (`{"xdebug": false}`, mirroring varnish's
  `{"varnish": bool}`, `wrappers.go:191-193`) needs pinning here or is already governed by
  `cli-output-contract`.

## Approach

| Layer | Change |
|---|---|
| ports / adapters / app / facade | **None.** `Exec` and `Compose` already exist end to end |
| `internal/cli/wrappers.go` | one handler beside `varnish`: `projectOr` → `Exec` → status guard → `Compose` → success line or `document()` |
| `internal/cli/run.go` | one `case` |
| `internal/cli/*_test.go` | one table test on the `newEngine` seam; `fakeEngine` grows nothing |
| `MIGRATION.md` | row + counter |

Strict TDD: RED on the seam first, with `answering(t)`'s pins (`HM_NON_INTERACTIVE`,
`DOCKER_HOST=unix:///nonexistent`, `HM_LEGACY_ROOT`) so an unrouted RED cannot reach a daemon or the
bash bridge.

## Size

**One work unit, one commit, well inside the 400-line budget.** Estimated **~120-180** authored lines
(handler ~40, test ~70-90, `run.go` 2, `MIGRATION.md` 2), spec deltas excluded.

The exploration's ~150-220 is believable at its low end, and its upper end shrinks: both decisions
above chose the zero-new-surface option, so there is no `commands` method and no `fakeEngine` method
— roughly the 15-30 lines that estimate reserved for them. The repo's measured 1.3-2x overshoot is
already inside the range.

Measure against **this change's own parent** — the `release/2.0.0` tip captured with `git rev-parse
HEAD` *before* the first commit — as
`git diff --shortstat <that-hash>..HEAD -- . ':(exclude)openspec'`. Never a hash copied from an
earlier proposal: that has silently measured unrelated commits twice here.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/wrappers.go` | Modified | the handler |
| `internal/cli/run.go` | Modified | one `case` |
| `internal/cli/` (test) | New | one seam test |
| `MIGRATION.md:21`, `:260` | Modified | counter and row |
| `console/commands/debug-off.sh` | Unchanged | retired in a later step |
| `console/commands/exec.sh`, `debug-on.sh`, `create-project.sh` | Unchanged | stated, because nothing is freed |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Dropping the defensive start surprises someone on a stopped environment | Med | Disclosed in the criteria; the failure names the step and is actionable |
| The literal `Compose` restart is the wrong call after all | Low | Decision 1 names exactly what to read, and the asymmetry is documented instead of erased |
| Compose's `restart` does not reload the ini for some image | Low | Compose `restart` restarts the process, which re-reads `conf.d`; verified manually against a running project |
| `MIGRATION.md` row and `case` land in different commits | Low | Same commit; `migration_status_test.sh` fails otherwise |
| `--json` payload invented rather than specified | Low | Deferred to the spec phase, named above, not decided silently |

## Rollback Plan

One commit on `release/2.0.0`. `git revert` restores the `shell` row, the counter and the bridge;
`console/commands/debug-off.sh` was never touched, so `hm debug-off` falls through to it again
immediately. No state, no files written, no migration for users.

## Dependencies

None. No impact on existing projects: the arguments, the message and the container-facing effect are
unchanged.

## Success Criteria

- [ ] `HM_LEGACY_ROOT=/nonexistent hm debug-off` works — proof it does not reach `bin/run`.
- [ ] Against a running project, `hm debug-off` prints `  xdebug disabled` in green and `hm describe`
      then reports xdebug off; run twice, the second is a no-op.
- [ ] **Behaviour change, proved deliberately**: with the environment stopped, `hm debug-off` exits
      with compose exec's own status and names the ini edit as the failing step — it does not start
      the environment.
- [ ] The seam test asserts the exact three-call log, including `sed`'s argv with **no** backslash
      before `z` and with `-e` kept.
- [ ] No `Start` call is recorded, and `commands` (`engine.go:17-34`) has the same sixteen methods.
- [ ] From an unregistered worktree, `hm debug-off` still works and is not refused with code 6.
- [ ] `MIGRATION.md` says `33 de 65` and the `debug-off` row says `go`;
      `tests/unit/migration_status_test.sh` and `tests/unit/command_safety_test.sh` pass.
- [ ] `go test ./... -short`, `tests/run.sh unit`, `gofmt -l ./cmd ./internal` empty, `go vet ./...`
      clean, and `var _ commands = (*dockergento.Engine)(nil)` still compiles.
