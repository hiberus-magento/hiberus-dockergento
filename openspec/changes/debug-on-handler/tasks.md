# Tasks: debug-on-handler

Unit 3 of 3, the last and largest: porting `hm debug-on` to Go. No design document exists for
this change — the proposal left no architectural fork open, so this file decomposes the proposal
and the two delta specs (`specs/go-entrypoint/spec.md`, `specs/magento-commands/spec.md`) directly
into ordered, checkable work.

Every task below cites the exact scenario and test name it satisfies, so a reviewer can trace each
commit back to the spec that demanded it. Strict TDD: each checklist item is a RED-then-GREEN pair
unless marked otherwise.

## 0 · Before anything: path safety

- [x] Confirm every file this plan touches already exists, and that no task below creates a new
  file. Run, from the repository root:
  ```
  test -e dockergento/dockergento.go && test -e internal/cli/engine.go && \
  test -e internal/cli/wrappers.go && test -e internal/cli/wrappers_test.go && \
  test -e internal/cli/fake_engine_test.go && test -e internal/cli/run.go && \
  test -e MIGRATION.md && echo "all present"
  ```
  This plan introduces **no new file**. If, while applying, a slice below is split into a new file
  (for example, pulling the two parsers out of `wrappers.go` into a dedicated file), run
  `test -e <path> && echo EXISTS || echo FREE` for that exact path **before** the `Write` call, and
  stop if it prints `EXISTS` — this is the guard unit 1 skipped when it silently clobbered a
  120-line `resolve.go`.

## 1 · Pure parsers (lands alone — no seam, no caller, full RED/GREEN)

Two `string → value` functions, fixture-testable with no `t.Skip`, per the proposal's Approach
table: they land in `internal/cli/wrappers.go`, beside `debugOff`, with tests in
`internal/cli/wrappers_test.go`.

- [x] **RED**: write the `php -v` → Xdebug-version parser's fixture tests. Four fixtures, each
  earning its place because a less adversarial one could pass by accident (the lesson unit 2's
  `ip addr`/`inet6` fixture proved):
  - a v2 version line → classifies `"v2"`
  - a v3 version line → classifies `"v3"`
  - a fixture with **no** Xdebug line at all — not one where the line merely sits late — →
    classifies as neither (this is the fixture the shell's own `set -euo pipefail` could never
    reach; Decision 1 / spec scenario "Xdebug absent or unrecognized is reported, not a failure")
  - a fixture containing `Xdebug v` but neither `v2.` nor `v3.` (the `v4` case — the only one the
    shell's own `else` branch could ever reach) → classifies as neither
- [x] **GREEN**: implement the version-extraction-and-classification parser in
  `internal/cli/wrappers.go`
- [x] **RED**: write the `php -i` settings-selection parser's fixture tests (the
  `grep -e ... | cut -d= -f1-2` equivalent). One fixture does the discriminating work: it carries
  **both** `xdebug.remote_host` and `xdebug.client_host` (plus `xdebug.idekey`), so:
  - selecting for v2 returns only `xdebug.idekey`, `xdebug.remote_host`, `xdebug.remote_port` —
    proving a v2 selection that wrongly also picked up the v3 key would be caught, because the v3
    key is present, not absent
  - selecting for v3 returns only `xdebug.idekey`, `xdebug.client_host`, `xdebug.client_port` on
    the same fixture, for the same reason in the opposite direction
- [x] **GREEN**: implement the settings-selection parser in `internal/cli/wrappers.go`
- [x] Verify: `go test ./internal/cli/...` (new parser tests green, nothing else touched),
  `go vet ./...`, `gofmt -l ./internal ./cmd` empty

## 2 · Seam growth (16 → 18, plus `outcome.captured`)

`internal/cli` never imports `dockergento/adapters/*` (verified, zero hits), so `XdebugHost` needs
a facade method before it can join `commands`. `Capture` already exists at the right signature
(`dockergento.go:469`, `(dir, service string, command []string) (string, int, error)`) and needs
only the interface line.

- [x] Add `func (e *Engine) XdebugHost() (string, error) { return machine.Host{}.XdebugHost() }` to
  `dockergento/dockergento.go`, beside the other thin facades (`machine` is already imported —
  used at `Diagnose`'s `Machine: machine.Host{}`, so no new import)
- [x] Add two lines to the `commands` interface in `internal/cli/engine.go:17-34`:
  `Capture(dir, service string, command []string) (string, int, error)` and
  `XdebugHost() (string, error)` — sixteen to eighteen. Leave the doc comment at `:14-16`
  unchanged; it must still read true afterward
- [x] Add `captured string` to `outcome` in `internal/cli/fake_engine_test.go:51-54` — **not** a
  new field on `fakeEngine`. This is a correction to an earlier sketch, not a style choice:
  `debug-on` calls `Capture` twice (`php -v`, then `php -i`) and needs a different answer each
  time; `outcomes` is already indexed by call number, so a field on `outcome` gives per-call
  answers for free, and a field on `fakeEngine` could not
- [x] Add `fakeEngine.Capture(dir, service string, command []string) (string, int, error)`,
  recording `call{Method: "Capture", Dir: dir, Service: service, Command: command}` and answering
  `(out.captured, out.status, out.err)`
- [x] Add `fakeEngine.XdebugHost() (string, error)`, recording `call{Method: "XdebugHost"}` — no
  directory, the same shape `Environments` and `Installed` already use for "asks about the machine,
  not the project" — and answering `(out.captured, out.err)`
- [x] Confirm both compile assertions still hold with no other change:
  `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:36`) and
  `var _ commands = (*fakeEngine)(nil)` (`fake_engine_test.go:90`)
- [x] **Not assertion-only**: write one small fake-level test in `internal/cli/fake_engine_test.go`
  (or a nearby `_test.go`, not `wrappers_test.go` — `debugOn` does not exist yet) proving the two
  new fake methods record the right call and forward `outcome.captured` / `.status` / `.err`
  correctly, for both `Capture` and `XdebugHost`, in isolation from any handler. See "On the
  assertion-only concern" below for why this task exists and is not optional.
- [x] Verify: `go build ./...`, `go vet ./...`, `gofmt -l ./internal ./cmd ./dockergento` empty,
  `go test ./internal/cli/... ./dockergento/...` green

### On the assertion-only concern

The proposal's own slicing (and unit 2's proposal before it) names this slice "an assertion-only
commit whose proof is that the two compile assertions still hold" and discloses that as "a poor
thing to ask a reviewer to accept alone" rather than hiding it. I agree it is worth disclosing, but
I do not think the fix is to merge this slice into 3a or 3b — `XdebugHost` gets exercised in 3a,
but `Capture` is not called by anything until 3b, so merging into 3a would leave the exact same gap
one slice later, just less visible. The fix that keeps the four-commit shape **and** gives this
slice its own RED/GREEN under strict TDD is the added fake-level test above: it is small, needs no
handler, and is what turns "the interface still compiles" into "the interface still compiles, and
what it forwards is provably correct."

## 3a · Handler, enabling half (through `Restart`)

`debugOn(stdout, stderr io.Writer, jsonOutput bool) int` in `internal/cli/wrappers.go`, beside
`debugOff`. This slice covers only what a successful run does up to and including the restart, and
the failure/refusal paths that never reach `Capture` — the happy path through the banner and JSON
is 3b's, because "succeeded" here is only provable once the readback exists.

- [ ] `projectOr` / `newEngine`, matching every other wrapper's opening
- [ ] `engine.XdebugHost()` — consulted unconditionally; `$XDEBUG_HOST` is never read on this path
  (Decision 8). On a plain error, `report()` immediately with no further call
  (`TestDebugOnReportsAnUnresolvableHost` — the fake answers `("", ErrNoBridgeAddress)`-shaped;
  call log ends at `XdebugHost`, exit `exitDocker`, fault type `docker_failed`)
- [ ] When `XdebugHost` answers a non-empty host: two `Exec` calls against `phpfpm`, patching
  `/usr/local/etc/php/conf.d/xdebug.ini` — first `xdebug.remote_host`, then `xdebug.client_host` —
  with the shell's stray, inert `\=` dropped from both patterns (GNU sed already read it as a
  literal `=`; same undefined-escape shape as `debug-off`'s dropped `\z`, Decision 7)
- [ ] The uncomment `Exec` call, always run regardless of host resolution, against the
  **different** file `/usr/local/etc/php/conf.d/docker-php-ext-xdebug.ini`, dropping the shell's
  stray `\;`
- [ ] Any of the three edit `Exec` calls answering non-zero with no Go-level error →
  `failure()` naming the edit as the failing step, no further call recorded — no `CopyFrom`, no
  `Restart`, no `Capture`, and in particular no `Start` (Decision 3)
  (`TestAFailedDebugOnEditReportsTheContainersStatus` — shared verbatim with
  `magento-commands`' "An xdebug-enabling edit fails inside the php container", proved once)
- [ ] The same three calls, on a Go-level error instead of a non-zero status → `report()`, no
  further call (`TestADebugOnEditErrorStopsBeforeTheRestart`)
- [ ] On macOS (`runtime.GOOS == "darwin"`, the same check `php.go:100` already uses — no new
  platform-detection mechanism): print `Copying generated code into host` via `warning()` before
  calling `copyFrom([]string{"generated"}, stdout, stderr, jsonOutput)` — `copyFrom` prints nothing
  of its own on success, so this line only appears if the handler writes it
- [ ] `engine.Restart(project.Root, []string{"phpfpm"})` — **not** `engine.Compose`, the opposite
  choice from `debugOff`'s own, because `restart.sh` is a faithful stop-then-start and
  `Operator.Restart` is what reproduces that, worktree refusal included
- [ ] `Restart`'s error, whatever shape it takes, goes through the existing generic `report()`
  (`orchestrate.go:248-253` already maps a `core.Refusal` to its own code/kind and anything else to
  `exitDocker`/`docker_failed` — no special-casing needed in the handler itself):
  - a `core.Refusal{Kind: "blocked_in_worktree", Code: 6}` → call log ends at `Restart`, no
    `Capture` ever recorded, exit `6` — **new** behaviour, disclosed (Decision 5)
    (`TestDebugOnRefusesFromAnUnregisteredWorktree`)
  - a plain error → exit `exitDocker`, fault type `docker_failed`, no `Capture` recorded
    (`TestADebugOnRestartErrorIsReported`)
- [ ] The two host-patch `Exec` calls' command arguments reflect whatever `XdebugHost()` answered,
  never an environment read, even when the calling process's own `XDEBUG_HOST` is set — asserted
  without needing the restart or the readback to complete
  (`TestDebugOnIgnoresXdebugHostEnvironmentVariable` — Decision 8, disclosed)
- [ ] Verify: `go test ./internal/cli/...` green (the six tests above, plus every existing test
  unaffected), `go vet ./...` clean, `gofmt -l ./internal ./cmd` empty
- [ ] **Do not** add the `run.go` case yet, and do not touch `MIGRATION.md` yet — routing is
  deliberately deferred to 3b, so the command is never reachable without the banner it exists to
  print

## 3b · Handler, readback half + routing + `MIGRATION.md`

Completes `debugOn`: the two `Capture` calls, the version/settings parsing from Slice 1, the text
banner, the `--json` envelope, the `run.go` case, and `MIGRATION.md`.

- [ ] After a successful `Restart`, `engine.Capture(project.Root, phpService, []string{"php", "-v"})`
- [ ] A failing first `Capture` (non-zero status or error) → `failure()` naming the step ("reading
  the Xdebug version"), no second `Capture` ever recorded
  (`TestAFailedDebugOnCaptureIsReported`, first-call half)
- [ ] Classify the captured `php -v` output with Slice 1's version parser
- [ ] When it classifies v2 or v3: `engine.Capture(project.Root, phpService, []string{"php", "-i"})`
- [ ] A failing second `Capture` → `failure()` naming the step ("reading its settings")
  (`TestAFailedDebugOnCaptureIsReported`, second-call half)
- [ ] Select settings with Slice 1's settings parser (v2 keys or v3 keys, per classification) and
  print, between two separator lines: only the selected keys — proven against the dual-key fixture
  so a wrong selection cannot pass by the other version's key being absent
- [ ] When classification is neither v2 nor v3 (absent or the `v4` case): no second `Capture` at
  all; print, between the same two separator lines, that Xdebug is not installed — and still exit
  `0`, because by this point the edits and the restart already succeeded (Decision 1, the branch
  the shell's own `else` could never reach for the "absent" case)
  (`TestWhatDebugOnAsksXdebugAbsent` — both discriminating fixtures drive the same branch in one
  test; text case)
- [ ] The full ordered call log for a successful Linux run — `Resolve`, `XdebugHost`, both host
  patches, the uncomment, `Restart`, both `Capture` calls, in that order, with `Restart` (not
  `Compose`) pinning the method choice — for both v2 and v3 fixtures
  (`TestWhatDebugOnAsks`, text case: v2, v3, and the restart-method-choice assertion, all the same
  test per the spec)
- [ ] `--json` on success: `{"xdebug": true, "version": <verbatim php -v string>, "host": <patched
  address, or JSON `null` when nothing was patched — not an empty string>, "settings": {<the ini
  keys exactly as read, unrenamed>}}`. Watch this specifically: a plain Go `""` marshals to JSON
  `""`, not `null` — the "no host" case needs `any(nil)` (or an equivalent nil-able value) in the
  `map[string]any` passed to `document()`, not an empty string
  (`TestWhatDebugOnAsks`, JSON case: v2 and v3)
- [ ] `--json` when absent/unrecognized: `{"xdebug": false}` only — no `version`, `host`, or
  `settings` key, mirroring `debug-off`'s own minimal shape
  (`TestWhatDebugOnAsksXdebugAbsent`, JSON case)
- [ ] The macOS success path, end to end: no host-patching `Exec` calls, but the uncomment `Exec`,
  the warning line, `CopyFrom`, `Restart`, and the completed readback with `host` reported as
  absent (`null` in JSON, omitted in text)
  (`TestWhatDebugOnAsksOnMac`)
- [ ] Add the `case "debug-on": return debugOn(stdout, stderr, jsonOutput)` to the switch in
  `internal/cli/run.go`, beside the existing `case "debug-off"`
- [ ] Update `MIGRATION.md`: `:21` `33 de 65` → `34 de 65`, and `:261` `debug-on`'s row from `shell`
  to `go` — in the same commit as the `run.go` case
- [ ] Verify: `go test ./...` green, `go vet ./...` clean, `gofmt -l ./cmd ./internal` and
  `gofmt -l ./dockergento` both empty, `./tests/run.sh unit` green (argument is exactly `unit`, not
  `tests/unit` — `run.sh:81` checks `-d "$TESTS_DIR/$target"`, so `tests/unit` resolves to
  `tests/tests/unit`, misses, and `:83` falls through to running every suite in the repository),
  and specifically confirm `tests/unit/migration_status_test.sh` passes (it checks both directions:
  the row matches what `run.go`'s switch answers for, and the counter matches the row count)

## Final verification (whole change)

- [ ] `go test ./...` — compare against the baseline of **423 passed across 24 packages**; expect
  it to grow by the new test functions in Slices 1-3b and nothing to regress
- [ ] `go vet ./...` clean
- [ ] `gofmt -l ./dockergento` empty
- [ ] `gofmt -l ./internal ./cmd` empty
- [ ] `./tests/run.sh unit` — compare against the baseline of **616 assertions, 0 failed**
- [ ] `commands` has exactly eighteen methods, and `internal/cli/engine.go:14-16`'s doc comment is
  unchanged and still reads true
- [ ] Any size or diff check run during review derives its base at check time (`git merge-base
  HEAD main` or equivalent) — never a hardcoded base-commit hash

## Explicitly not verifiable here

- That the `sed` edits land in a real `xdebug.ini` / `docker-php-ext-xdebug.ini` inside a running
  container
- That PHP-FPM actually picks up the edits after the restart
- What a real PHP image's `php -v` / `php -i` print, for any of this project's supported PHP
  versions — the fixtures are built from documented output shapes, not from this project's own
  images
- Anything requiring a live Docker daemon. **Do not run `hm debug-on` or `bin/run debug-on`**
  against any project while applying or verifying this change — it edits a live PHP ini inside a
  container and restarts a service, and this machine has unrelated projects running.

The only oracle for the three items above is manual verification against a real project, done by a
human, outside this task list.
