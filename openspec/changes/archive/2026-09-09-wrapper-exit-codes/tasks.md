# Tasks: wrapper-exit-codes

One work unit, one commit. Strict TDD: every behavioural task states its RED before its GREEN.

Corrected line numbers (re-verified against `internal/cli/wrappers.go` on this branch; the
exploration's numbers had drifted by two): the VCL-edit guard's `if` is `:155-157` (its `Exec` call
starts at `:152`), and the cache-toggle guard's `if` is `:165-167` (its `Exec` call starts at
`:163`). The function `varnish()` runs `:134-196`.

## 1. RED — a failing VCL edit must report the container's code, not answer success

- [x] 1.1 In `internal/cli/wrappers_test.go`, add `TestAFailedVCLEditReportsTheContainersCode`, run
      against `varnish(true, ...)` (`varnish-on` — the spec names this test on the edit-failure path,
      shared code with `varnish-off`, so it is proved once).
      - `fake.outcomes = []outcome{{}, {status: 42}}`: call 0 (`Resolve`, from `projectOr`) succeeds
        with its zero-value outcome; call 1 (`Exec` for the `sed` edit against `varnishService`)
        answers status 42, `err: nil`. Confirmed against `internal/cli/fake_engine_test.go:94-101` —
        `Exec` returns `(out.status, out.err)` from `outcomeFor(number)`, indexed by call order, so
        this shape says exactly "the call was made and the container exited 42".
      - Assert `code == 42`.
      - Assert `fake.calls` is exactly `[Resolve, Exec(sed)]` — no `Restart` and no second `Exec`
        (the `cache:enable` call) recorded. This is what proves the guard stops the function, not
        just that it returns the right number.
      - Assert stdout carries no `"Varnish cache enabled!"` line and no `--json` success document.
      - Assert stderr carries a readable line naming the VCL edit as the failing step (text mode) and,
        in a `--json` run of the same case, exactly one envelope with `ok: false` and
        `error.code == 42`.
      - Run it now: it fails, because today's guard is
        `if err != nil || status != 0 { return report(stderr, jsonOutput, command, err) }` —
        `err` is `nil`, so `report()` (`orchestrate.go:224-227`) returns `exitOK`, not 42, and
        because the return sits above `document(...)` (`:184`) and `fmt.Fprint` (`:188`), no
        assertion about a "step failed" message has anything to find either. This is the RED for
        magento-commands' scenario "The VCL edit fails inside the varnish container" and
        cli-output-contract's "Falla el programa que un envoltorio ejecuta".

## 2. RED — a failing cache toggle must stop before the purge, not answer success

- [x] 2.1 In `internal/cli/wrappers_test.go`, add `TestAFailedCacheToggleStopsBeforeThePurge`, run
      against `varnish(false, ...)` (`varnish-off` — the spec requires this test run against
      `varnish-off` specifically, because it is the one with a cleanup step to skip; the guard code
      itself is identical for both commands, so this is not duplicated for `varnish-on`).
      - `fake.outcomes = []outcome{{}, {}, {}, {status: 42}}`: call 0 `Resolve` succeeds, call 1
        `Exec` (the `sed` edit) succeeds, call 2 `Restart` succeeds, call 3 `Exec` (`cache:disable`
        against `phpService`) answers status 42, `err: nil`.
      - Assert `code == 42`.
      - Assert `fake.calls` is exactly `[Resolve, Exec(sed), Restart, Exec(cache:disable)]` — no
        further `Resolve`/`Exec` for the purge (`purge(...)`, `wrappers.go:174`) and no `Exec` for
        `magento([]string{"cache:clean"}, ...)` (`wrappers.go:178`). This is the direct proof for
        "Turning the page cache off clears what it cached" / "The disable step fails before the
        cleanup runs" — the guarantee only holds if the cleanup never runs on a reported failure.
      - Assert stdout carries no `"Varnish cache disabled!"` line and no `--json` success document.
      - Assert stderr carries a readable line naming the cache-toggle step (text mode) and, in a
        `--json` run, exactly one envelope with `ok: false` and `error.code == 42`.
      - Run it now: it fails for the same reason as 1.1 — `err` is `nil`, `status != 0` is ignored by
        `report()`, and the function returns `exitOK` before ever reaching the purge block
        (`:173-181`), so today it also never proves the purge was skipped, because everything above
        it already looked like success. This is the RED for magento-commands' scenario "The
        full-page-cache toggle fails inside the php container" and "The disable step fails before the
        cleanup runs".

## 3. Confirm the regression that must not move

- [x] 3.1 Run `TestAFailedEditStopsBeforeTheRestart` (`internal/cli/wrappers_test.go:236`, existing,
      unchanged) before touching `wrappers.go`, to record its current pass: it covers the
      `err != nil` branch of the same two guards (`fake.outcomes = []outcome{{}, {err: refused}}`,
      asserting `code == exitDocker` and that only `[Resolve, Exec(sed)]` were asked). Fixing the
      `status != 0, err == nil` branch is exactly where the sibling branch a guard shares regresses,
      so this is a named checkpoint, not an assumption: re-run it after 4.1 and confirm it still
      passes unchanged.

## 4. GREEN — split the two guards

- [x] 4.1 In `internal/cli/wrappers.go`, replace the guard at `:155-157`
      (`if err != nil || status != 0 { return report(stderr, jsonOutput, command, err) }`,
      following the `Exec` call at `:152-154`) with `inside()`'s order
      (`internal/cli/php.go:57-66`: status first and raw, `report()` only for a Go-level error):
      ```go
      if status != 0 {
          return failure(stderr, jsonOutput, command, status, "<kind>",
              "The VCL edit failed in the varnish container", "")
      }
      if err != nil {
          return report(stderr, jsonOutput, command, err)
      }
      ```
      `failure()` (`internal/cli/output.go:59-75`) returns its `code` argument verbatim, so handing
      it `status` yields both the raw exit code and the `ok: false` envelope in one call — no new
      helper, per the proposal's non-goal.

      The `kind` is **`command_failed`**, decided rather than left open. The vocabulary in this
      package is `<subject>_<state>`, and `docker_failed` is the precedent for one kind covering a
      family of failures with the message telling them apart — which is what the spec asks for here
      ("a message naming which step failed"). Reusing `docker_failed` itself would be false: Docker
      answered, the program it ran did not. `command_failed` is not taken (`missing_command` and
      `wrong_command` are the only neighbours). Both guards use it; the message is what differs.
      Run `TestAFailedVCLEditReportsTheContainersCode` (1.1): GREEN. Run
      `TestAFailedEditStopsBeforeTheRestart` (3.1): must still pass.
- [x] 4.2 In `internal/cli/wrappers.go`, replace the guard at `:165-167`
      (following the `Exec` call at `:163-164`, against `phpService`, running
      `cache:enable`/`cache:disable`) the same way, with a message naming the cache-toggle step
      (e.g. `"The %s failed in the phpfpm container"` interpolating `cache`, which is already the
      command-name variable in scope at that point in `varnish()`), with the same `command_failed`
      kind: the two steps are the same kind of failure and the message is what separates them.
      Run `TestAFailedCacheToggleStopsBeforeThePurge` (2.1): GREEN.
- [x] 4.3 Run the full existing `TestWhatVarnishAsks` (`internal/cli/wrappers_test.go:163-234`, both
      `on` and `off` subtests): confirm the happy path is untouched — same argv, same order, same
      calls, same document, same success text.

## 5. Audit the migration record

- [x] 5.1 In `MIGRATION.md`, near `:157-167` (the paragraph claiming "la misma disciplina de las
      líneas 62-66: nada de shell se borra antes de que su comportamiento esté cubierto en Go" for
      the twelve retired shell twins), add one paragraph, in the document's own language (Spanish),
      recording that the claim held for eleven of the twelve and not for `varnish-on`: its deleted
      shell twin (`console/commands/varnish-on.sh`, confirmed via
      `git show 583fb0c^:console/commands/varnish-on.sh`) opened with `set -euo pipefail`, so a
      failing `sed` ended the script there — with the `sed`'s own exit code, no restart, no message —
      a behaviour that was not covered in Go until this change. Do not soften or remove the original
      claim; append the correction next to it.
      Verify `tests/unit/migration_status_test.sh` still passes after the edit (it is named in the
      proposal's own success criteria as a check that must keep passing).

## 6. Full verification

- [x] 6.1 `go test ./...` — baseline was 297 across 24 packages; expect the same count plus the two
      new tests (299), all packages still passing.
- [x] 6.2 `gofmt -l ./cmd ./internal` — empty output.
- [x] 6.3 `go vet ./...` — clean.
- [x] 6.4 `./tests/run.sh unit` — baseline 616 assertions, unchanged (note: the argument is `unit`,
      not `unit/` or `tests/unit`; `tests/run.sh` otherwise takes a path like
      `tests/integration/x.sh`).
- [x] 6.5 `./tests/run.sh tests/integration/go_wrappers_test.sh` — baseline 28 assertions, unchanged
      (this suite proves the happy path against a real container; nothing in this change touches
      what it asserts).
- [x] 6.6 Re-read the diff against the 400-line review budget, against **this change's own parent**
      (`e4af01c`), not a hash copied from an earlier change:
      `git diff --shortstat e4af01c..HEAD -- . ':(exclude)openspec'`.
      Measured: **184 lines** (3 files, 182 insertions, 2 deletions) — over the proposal's ~50-90
      estimate and well under the 400-line budget, so no delivery decision is needed. The overrun is
      the test surface, not the fix: 10 lines change `wrappers.go`, 163 are the two tests, which the
      specs require to assert three observables each across a text and a `--json` subtest.

## 7. Commit

- [ ] 7.1 One commit, on `release/2.0.0`, no AI attribution, conventional-commit style, imperative
      mood, e.g.:
      ```
      fix(cli): report the container's own exit code from varnish's guards

      varnish-on and varnish-off answered 0 when the sed edit or the cache
      toggle exited non-zero with no Go-level error, because report()
      only maps errors and the return sat above the success document.
      Split both guards into inside()'s shape: status first and raw,
      reported through failure() with the container's own code, then
      report() only for a Go-level error. Corrects MIGRATION.md's claim
      that retiring the twelve shell twins moved no exit code -- true for
      eleven, false for varnish-on's deleted set -euo pipefail twin.
      ```
