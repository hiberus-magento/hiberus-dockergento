# Proposal: What `down`, `set-host` and `version` ask of the engine

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `wrappers-go-tests` and `remaining-wrappers-go-tests`.

## Intent

`MIGRATION.md`'s porting rule is: a command ported → its test in Go → then the shell twin is deleted.
The seam exists (`internal/cli/engine.go:17` `commands`, `:31` `newEngine`, `fake_engine_test.go`) and
carries seven methods. Three Go-wired commands still have no Go test: `down`, `set-host` and
`version`. `version` was cut from the predecessor by its size gate at 247 lines; this change closes
all three and leaves no Go-wired command untested.

## Scope

### In Scope

- Go tests on the existing seam for `version`, `set-host`, `down`.
- `commands` gains exactly four methods, verified in `dockergento/dockergento.go`:
  `Installed() (core.Installation, core.Tooling)` (:468), `SetHost(dir, domain string, database bool) error`
  (:479), `RemoveHost(domain string) error` (:489), `Down(dir string, options core.DownOptions, interactive bool) (string, error)` (:235).
- Four call sites move from `engine(` to `newEngine(`: `version.go:23`, `wrappers.go:281`
  (`RemoveHost`), `wrappers.go:296` (`SetHost`), `down.go:61`. No other `engine(` site changes.
- The shared `answering(t)` helper (`fake_engine_test.go:130`) pins the RED-safety environment.

### Non-goals (Out of Scope)

- Any behaviour change. Arguments, `--json` documents, exit codes and prompts stay as they are.
- Two quirks pinned literally, never fixed here: `down` prints "Nothing was destroyed." even under
  `--json` (`down.go:67-71`), and `--rmi`/`-t` with no following value are silently ignored (`:31-38`).
- The missing-domain guard in `set-host` and `down`; `here()` vs `project.Root`; the varnish
  `report(nil err)` defect.
- Deleting any Bash implementation or parity suite; porting any command; any test needing a daemon.

## Capabilities

### New Capabilities

None. No user-visible behaviour changes.

### Modified Capabilities

- `go-entrypoint`: "A ported command's engine interaction is provable without Docker" (spec.md:192)
  gains scenarios for `down` and `set-host`, plus the two `version` scenarios cut from the
  predecessor, which return here. "A usage error returns before any engine call" (:268) broadens
  from the three path-taking commands to `version`, `set-host` and `down`.

## Approach

Test first, then the minimal seam growth that compiles it — strict TDD, as both predecessors did.

**RED safety is a hard requirement, and lands first.** While a site is unrouted, RED reaches the real
engine: `Hosts.file()` falls back to the literal `/etc/hosts` (`app/hosts.go:186`) and `write()`
shells out to `sudo cp` wired to the real terminal (`:173`), so an unrouted `set-host --remove` could
ask for the developer's password and edit the machine's hosts file. `down` would dial the daemon
(`Installed`/`presentVolumes` have no deadline) and could reach the real `Choose`. Before any RED run,
`answering(t)` pins unconditionally, beside the existing `HM_STATE_DIR`:
`HM_HOSTS_FILE=filepath.Join(t.TempDir(), "hosts")` (a missing file fails at `os.ReadFile` before any
write), `HM_NON_INTERACTIVE=1` (`select.go:39` makes `choose()` refuse) and
`DOCKER_HOST=unix:///nonexistent`. These pins ship inside the first commit, so every later RED run in
the package is covered. The one `down` subtest asserting `Interactive: true` overrides
`HM_NON_INTERACTIVE` with a nested `t.Setenv` after the site is routed.

**Fake shape.** `outcome` (`fake_engine_test.go:35`) gains `result string`, read the way `Exec` reads
`status`. `call` (:19) gains `Domain`, `Database`, `Interactive`, `DownOptions core.DownOptions`
(`Options` is already `core.ExecOptions`). `core.DownOptions` (`core/orchestration.go:44-58`) is all
exported, so `cmp.Diff` needs no new options. `Installed` returns values a call log cannot carry:
answer fields `installed`, `tooling`, as `Property` already does.

**What each test pins.** `version`: usage error on any argument with an empty log; `Installed()` alone
in the log; the JSON document, with `binary` from the pure `buildOfThisBinary()` asserted separately.
`set-host`: `--remove` skips `projectOr` and calls only `RemoveHost`; the default path calls
`SetHost(here(), domain, database)` with `--no-database` flipping the flag; unknown `-` flag with an
empty log. `down`: each flag's effect on `DownOptions`; `-t nada` and an unknown flag as usage errors
with an empty log; `"destroyed"`/`"saved"` → the JSON document; `""` → the text even under `--json`;
`errNothingChosen` → `exitInterrupted`; a `core.Refusal` through `asRefusal` (first test in the family
to exercise that mapping); anything else → `exitDocker`.

## Delivery

Three commits in this order, each self-consistent, each carrying its own interface method, fake growth
and routing:

| # | Commit | Estimate |
|---|---|---|
| 1 | `version` + the `answering` safety pins | ~104 |
| 2 | `set-host` | ~120 |
| 3 | `down` | ~150 |

A size gate runs after commit 2, measured only with
`git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'` and nothing added to it. At the
corrected ~1.5x density the projection is ~418, so the gate is likely to trip: **if the projection
exceeds 400, `down` moves to a follow-up change named `down-go-tests`.** `version` is never cut again
— it was already deferred once — and comments are never cut. Alternative considered: two separate
changes (`version` alone ~104, `down`+`set-host` ~314). Rejected as ceremony for 104 lines, unless the
user prefers it.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/engine.go` | Modified | `commands` gains `Installed`, `SetHost`, `RemoveHost`, `Down` |
| `internal/cli/version.go` | Modified | `:23` routes through `newEngine` |
| `internal/cli/wrappers.go` | Modified | `:281`, `:296` route through `newEngine` |
| `internal/cli/down.go` | Modified | `:61` routes through `newEngine` |
| `internal/cli/fake_engine_test.go` | Modified | Four fake methods, `outcome.result`, four `call` fields, two answer fields, three env pins in `answering` |
| `internal/cli/version_test.go` | New | `version`'s tests |
| `internal/cli/wrappers_test.go` | Modified | `set-host`'s tests, beside the copy tests |
| `internal/cli/down_test.go` | New | `down`'s tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| RED touches the real `/etc/hosts` through `sudo` | High without the pin | `HM_HOSTS_FILE` in `answering`, in commit 1, before any RED run |
| RED dials the daemon or reaches the real `Choose` | Med | `DOCKER_HOST=unix:///nonexistent` and `HM_NON_INTERACTIVE=1`, same commit |
| Combined size over the 400 budget | High (~418 projected) | Size gate after commit 2; `down` cut to `down-go-tests`; no `size:exception` |
| A test "fixing" a found quirk instead of pinning it | Med | Both quirks named in Non-goals; assertions state the current text literally |
| `asRefusal` mapping first exercised here and asserted wrongly | Med | Assert the refusal's own code/kind/message/hint, read from `core.Refusal`, not a rewritten expectation |
| The `Interactive: true` subtest fights the global `HM_NON_INTERACTIVE` pin | Med | Nested `t.Setenv` inside that subtest only, and only once the site is routed |
| Interface drift as `Engine` grows | Low | `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:27`) fails loudly |

## Rollback Plan

Up to three commits on `release/2.0.0`; `git revert` restores them in reverse order. Everything is
inside `internal/cli`, and the tests are additions, so reverting removes them with nothing attached.
The `answering` pins revert with commit 1 and are inert for existing tests.

## Dependencies

None. **No impact on existing projects and no migration step for users**: no command's arguments,
output, `--json` document or exit codes change. Proven by the untouched Bash parity suites and
`go build ./...`.

## Success Criteria

- [ ] Each of the three commands has a Go test that fails first, then passes, under
      `go test ./internal/cli -short` with no daemon, no network, no hosts-file access and no project
      beyond `t.TempDir()`.
- [ ] `answering(t)` pins `HM_HOSTS_FILE`, `HM_NON_INTERACTIVE` and `DOCKER_HOST` before the first RED run.
- [ ] `go test ./... -short` passes (baselines at HEAD `7417863`: 44 in `./internal/cli`, 204 overall;
      apply re-baselines).
- [ ] `tests/run.sh unit` passes, unmodified.
- [ ] `gofmt -l ./cmd ./internal` is empty; `go vet ./...` is clean.
- [ ] `var _ commands = (*dockergento.Engine)(nil)` still compiles after the interface reaches eleven methods.
- [ ] Exactly the four named call sites route through `newEngine`; no other `engine(` site changed.
- [ ] No file under `console/`, `bin/run`, or `dockergento/` is modified.
- [ ] Changed lines ≤ 400 measured with the shortstat command above, with the gate and cut order
      applied — no `size:exception`.
- [ ] Manual verification on a real project, kept light: `hm version`, `hm version --json`,
      `hm set-host --remove` of a domain the tool added, against a temporary file via `HM_HOSTS_FILE`,
      and the usage errors `hm down --tonteria` and `hm down -t nada`. Nothing that destroys a real
      environment.

## Rescoping (2026-09-05, after design)

The design's fresh-context gate found that an unrouted `set-host` RED run writes
`config/docker/properties.json` into the test's working directory, so the shared helper now also
pins the working directory (`t.Chdir(t.TempDir())`) and `HM_LEGACY_ROOT`. Ten existing tests must
read their expected directory after that pin (≈56 changed lines), which lifts the projection to
≈482 and makes the `down` cut certain rather than contingent. Rather than let the size gate discover
that mid-apply, this change is rescoped up front to `version` + `set-host` (≈317 lines); `down`
becomes the follow-up change `down-go-tests`, taking its five spec scenarios, its test file, the
`Down` seam method and the `outcome.result` field with it.
