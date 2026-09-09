# Proposal: The two wrappers that answer 0 for a command that failed

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `retire-ported-shell-commands`, `docker-tools-in-go` and `remaining-wrappers-go-tests`.

## Intent

`openspec/specs/magento-commands/spec.md:69-72` already says it: **"WHEN the program inside the
container fails THEN the exit code is the program's own."** `hm varnish-on` and `hm varnish-off` are
the only two commands in the tool that break it.

`internal/cli/wrappers.go:155-157` and `:165-167` collapse two different questions into one guard:

```go
status, err := engine.Exec(...)
if err != nil || status != 0 {
    return report(stderr, jsonOutput, command, err)
}
```

`report()` (`orchestrate.go:224-246`) turns an *error* into an exit code and answers `exitOK` when
there is none. A container command that exits non-zero with a nil error therefore arrives as
success. And because the return sits **above** `document(...)` at `:184` and the `fmt.Fprint` at
`:188`, the command emits nothing at all: no text, no document, exit 0.

`varnish-off` also never reaches its cleanup (`:173-181`), which exists because pages Varnish cached
are served until they expire. A failed `cache:disable` leaves those pages being served and reports
that it worked.

### The migration's own record, corrected

`583fb0c`'s design asserted that retiring the twelve shell twins moved no exit code. **That was true
for eleven of them and false for this one.** `console/commands/varnish-on.sh` opened with
`set -euo pipefail`: a non-zero `sed` ended the script there, with the `sed`'s own code, without the
restart and without the message. The shell twin was the last thing behaving correctly here, and it
was deleted. `MIGRATION.md:157-167` claims the third step was taken "con la misma disciplina de las
líneas 62-66: nada de shell se borra antes de que su comportamiento esté cubierto en Go". For these
two commands the behaviour was not covered. The claim gets audited in this change, not defended.

## The decision this change owns: what a failing wrapper says

Fixing the code alone would leave a command that answers non-zero and says nothing. Three findings
settle it, in this order:

1. **`inside()` (`php.go:57-66`) emits nothing on a non-zero status** — it returns the status raw.
   `tests/integration/go_wrappers_test.sh:91-104` proves the whole of what a caller gets: a recorder
   exiting 42 makes `hm npm --version` exit 42, and the output is npm's own, unwrapped.
2. **That silence is licensed, and the licence does not reach varnish.**
   `cli-output-contract/spec.md:97-116` excludes from the JSON envelope "los comandos cuya salida
   estándar es un dato o la salida de un proceso hijo". Every caller of `inside()` is one: `magento`,
   `composer`, `npm`, `n98-magerun`, `test-unit`, `test-integration` are all in the `transparent` map
   at `globals.go:17-22`. **`varnish-on` and `varnish-off` are not in that map, and they emit a
   success envelope** (`wrappers.go:183-185`). A wrapped command owes a document.
3. **So the failure is reported.** `cli-output-contract/spec.md:42-56`: "En modo JSON, los errores
   SHALL emitirse por stderr como un objeto con `ok: false` y un objeto `error` que contenga `code`,
   `type`, `message` y `hint`", and its text-mode scenario (`:53-56`) requires the same failure to
   show "el mensaje de error legible" with the same exit code. Text and JSON do not get to disagree
   about whether the command failed.

**Decided**: the guards take the *shape* of `inside()` — status first and raw, `report()` only for a
Go-level error — but not its silence. The container's status becomes the exit code **and** the
`error.code` of one envelope, so a reader branching on `$?` and a reader branching on `error.code`
never disagree, exactly as `docker_unavailable`/3 pairs them at `spec.md:47-51`.

`cli-output-contract/spec.md:58-73`'s table (0/1/2/3/4/5) is not contradicted: it governs failures
**the tool itself** decides. `magento-commands/spec.md:69-72` governs the inner program's. The delta
says which is which instead of leaving it to be inferred.

## Scope

### In Scope

- **The two guards in `varnish()`** (`wrappers.go:155-157`, `:165-167`) split, in `inside()`'s order:
  a non-zero status is reported with `failure(..., status, ...)` — which returns `code` verbatim
  (`output.go:59-75`) — and `report()` is reached only for a non-nil error.
- **One envelope, naming the step**: which of the three steps failed (the VCL edit on `varnish`, the
  restart, or `cache:enable`/`cache:disable` on `phpfpm`) is in the message, because the exit code
  alone cannot say and today nothing does.
- **No success output on a failing path**: no `document(...)`, no "Varnish cache enabled!".
- **Go tests at the existing seam**, one per guard: `fake.outcomes` already expresses "the call was
  made and the container exited non-zero" with `{status: N}` and a nil error
  (`fake_engine_test.go:39-51`; `docker_compose_test.go:43` uses `{{}, {status: 17}}` today).
  `wrappers_test.go:236-243` covers the `err != nil` branch only.
- **`MIGRATION.md:157-167`**: one paragraph, in that document's language, recording that the
  third-step claim held for eleven of the twelve and what this change repaired for the twelfth.

### Non-goals (Out of Scope)

- **Changing `report()`'s signature.** Giving it a status would touch ~30 call sites across 12 files,
  most of which have no status to give.
- **A new "run and report" helper.** One caller does not earn an abstraction, and retrofitting it
  onto the four healthy sites (`php.go:61-65`, `orchestrate.go:188-194`, `docker_compose.go:10-21`,
  `mysql.go:67-99`) is churn on code that is not broken.
- **`purge`'s own silence under `--json`.** `inside()` emits no document for any of its callers, not
  even on success, and `purge` is not in the `transparent` map either. When `varnish-off`'s cleanup
  step fails (`:173-181`) the exit code is **already** the right one; the missing document is
  `purge`'s to emit, in a change that covers every `inside()` caller at once.
- Any change to the happy path: same argv, same order, same messages, same success document.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `magento-commands`: "A wrapper runs what it says it runs" (spec.md:59-72). Scenario "The program
  fails" (:69-72) reads as satisfied and is not, for two commands. It gains a scenario for a wrapper
  that owns an envelope: the program's code **and** a reported failure. "Turning the page cache off
  clears what it cached" (:74-82) is reached too — when the disable fails the cleanup cannot run, so
  the guarantee survives only if the failure is reported rather than swallowed.
- `cli-output-contract`: "Errores estructurados" (spec.md:42-56) and "Códigos de salida estables"
  (:58-73). A wrapped command — one not covered by the passthrough exclusion at :97-116 — that fails
  because the program it ran failed emits the error envelope on stderr and exits with that program's
  code, with `error.code` mirroring it.
- `go-entrypoint`: "A ported command's engine interaction is provable without Docker"
  (spec.md:192-200). Its two varnish scenarios (:218-222, :224-228) prove the happy order only; each
  gains the failing-status case.

## Approach

| Area | What happens |
|---|---|
| `internal/cli/wrappers.go:155-157`, `:165-167` | `if status != 0 { return failure(stderr, jsonOutput, command, status, "<kind>", "<step> failed in the <service> container", "") }`, then `if err != nil { return report(...) }`. Four lines each, `inside()`'s order. |
| `internal/cli/wrappers_test.go` | Two RED cases: edit exits non-zero → that code, no restart, no document, no success line; `cache:*` exits non-zero → that code, and for `varnish-off` no purge and no cache-clean. |
| `MIGRATION.md:157-167` | The audited claim. |

Strict TDD: RED first, at the `newEngine` seam. No seam change and no new fake behaviour is needed.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/wrappers.go:152-167` | Modified | Two guards split |
| `internal/cli/wrappers_test.go` | Modified | Two cases for `status != 0, err == nil` |
| `MIGRATION.md:157-167` | Modified | The third-step claim audited |
| `openspec/specs/magento-commands`, `cli-output-contract`, `go-entrypoint` | Modified | Delta specs |

## Delivery

One work unit, measured with `git diff --shortstat <parent>..HEAD -- . ':(exclude)openspec'` against
this change's own parent — `e4af01c` at the time it was applied. A fixed hash written here goes
stale the moment another change lands first, so the parent is named, not hardcoded.

Estimated at ~50-90 lines. It came in at **184** (three files, 182 insertions), because the specs
ask for three observables per failing path and each of the two tests carries a text and a `--json`
subtest — 163 of those lines are the tests and 10 are the fix. Comfortably inside the 400-line
budget either way: **no delivery decision is needed before apply.**

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A caller depended on the 0 | Low | `varnish()` is reached only from `run.go:120-123`; its return value is the process's exit code and is chained into no further Go logic. Nothing in `tests/` asserts 0 on a failing varnish run |
| A behaviour change ships unannounced | Certain | It **is** the change: a command that answered 0 starts answering the container's code. Recorded here, in the spec delta and in `MIGRATION.md`; it is what the deleted shell twin did |
| `error.code` outside the documented table confuses a reader | Med | The delta says which requirement governs which code, and `magento-commands:69-72` already established the rule for wrappers |
| The message duplicates what the container already printed | Low | The inner command's own diagnosis reached the terminal through the inherited stdio; the added line says only which step it was, which nothing else says |

## Rollback Plan

One commit on `release/2.0.0`; `git revert` restores both guards, the two tests and the
`MIGRATION.md` paragraph. Nothing else depends on it.

## Dependencies

None. **No migration step for users**: no argument, prompt, success message or success document
changes. A script that treated `hm varnish-on` as always succeeding was already wrong.

## Success Criteria

The observable outcome, for a run where a step inside the container exits 42:

- [ ] **Exit code**: `hm varnish-on` and `hm varnish-off` exit **42** — for the VCL edit and for the
      `cache:enable`/`cache:disable` step, proved at the fake-engine seam for each.
- [ ] **Text output**: stdout carries **no** "Varnish cache enabled!"/"disabled!"; stderr carries the
      container's own output plus one legible line naming the step that failed.
- [ ] **`--json` output**: stdout carries **no** success document; stderr carries exactly one
      envelope with `ok: false`, `error.code` = 42, a `type` naming the failure kind, and a message
      naming the step — parseable by `jq`.
- [ ] A failed edit still stops before the restart (`wrappers_test.go:236` keeps passing), and a
      failed `cache:disable` still stops before the purge and the cache-clean.
- [ ] The happy path is untouched: same argv, same order, same messages, same document — the four
      existing varnish cases pass unchanged.
- [ ] `go test ./... -short`, `tests/run.sh unit`, `gofmt -l ./cmd ./internal` empty,
      `go vet ./...` clean.
- [ ] `MIGRATION.md` records that the third-step claim of `583fb0c` was true for eleven of the twelve
      and false for this one; `tests/unit/migration_status_test.sh` still passes.
- [ ] Manual verification, safe by construction: on a running project, `hm exec` a broken
      `/etc/varnish/default.vcl` path is **not** needed — the seam test covers it. Optional check on
      a project the developer can rebuild: `hm varnish-off` with `bin/magento` temporarily replaced
      by a recorder exiting 42, as `go_wrappers_test.sh:91-99` already does for npm.
