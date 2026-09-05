# Design: What the one-container wrappers, mysqldump and version ask of the engine

## Technical Approach

No new mechanism. The seam the predecessor built — `commands` (`engine.go:17`), `newEngine`
(`engine.go:29`), the compile-time assertion (`engine.go:25`), the ordered call log in
`fake_engine_test.go` — is used as it stands, grown by exactly three methods and four routed call
sites. Everything below is a decision about *how the tests are written against it*, because the
architecture question was settled by `2026-09-04-wrappers-go-tests`.

The three methods, signatures quoted from `dockergento/dockergento.go`:

```go
Property(project core.Project, key string) string  // :1239
Dump(dir, path string) error                       // :322
Installed() (core.Installation, core.Tooling)      // :468
```

## Architecture Decisions

### Decision 1: the two answer-only methods are still logged

`Property` and `Installed` return no error, so `outcome{status, err}` indexed by call number
(`fake_engine_test.go:33`) cannot answer them. The fake gains **answers** — `properties
map[string]string` keyed by property name, `installed core.Installation`, `tooling core.Tooling` —
and `call` gains `Key` (Property) and `Path` (Dump). A nil `properties` reads `""` for any key,
which is exactly the value `wrappers.go:70,78` code their fallbacks for.

They are **also appended to the ordered log**, `Property` as
`{Method: "Property", Dir: project.Root, Key: key}` and `Installed` as `{Method: "Installed"}`.

| Option | Trade-off | Decision |
|---|---|---|
| Answers + entry in the ordered log | One log per test still compares whole with `cmp.Diff` | **Chosen** |
| Answers only, no log entry | `tests()` would have no RED: the real engine also answers `""` for a root that does not exist (`fsprops/properties.go:53`), so want and got agree before the call site moves | Rejected |
| A separate `propertyKeys []string` on the fake | Loses interleaving — the exact defect the ordered log was chosen over per-method slices to avoid | Rejected |

`Dir` carries `project.Root` rather than the whole `core.Project`: it proves the *resolved* project
was passed (the fake's root is distinct from cwd) without dragging a `*Worktree` pointer into every
diff.

### Decision 2: three files, split by what they pin

| File | Holds | Lines |
|---|---|---|
| `inside_test.go` (new) | `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration` — the five that reach the container through `inside` (`php.go:57`) | ≈150 |
| `wrappers_test.go` (modify) | `mysqldump` joins the copy tests it is shaped like: usage error with an empty log, `projectOr`, one engine call at `here()`, refusal → `exitDocker` | 299 → ≈360 |
| `version_test.go` (new) | `version`, mirroring `version.go` | ≈95 |

Rejected: everything into `wrappers_test.go` (≈550 lines, four unrelated themes); a fourth
`dump_test.go` for two functions (a header and an import block bought no readability, and the
mysqldump tests read as siblings of `TestACopyTheEngineRefusesIsReported`).

### Decision 3: the cases, taken from the Bash parity suites

| Test | Cases |
|---|---|
| `TestWhatPurgeAsks` | Resolve(cwd), then `Exec` on `phpService` with the seven directories written out **literally** — `var/log` absent is the behaviour (`wrappers.go:20-25`, `go_wrappers_test.sh:84`), and `strings.Join(generated, " ")` would assert nothing |
| `TestWhatNpmAndMagerunAsk` | Two subtests side by side: `npm run build` → `{"npm","run","build"}` (argv pass-through); `n98-magerun cache:clean --no-interaction` → `{"bash","-c","n98-magerun cache:clean --no-interaction"}` (joined into one shell string) |
| `TestWhatTheTestSuitesAsk` | Table over `{kind, args, properties, keys, command}`: unit with `BIN_DIR=bin`; unit with nothing set → `./vendor/bin`; integration with `WORKDIR_PHP=/app`; integration with nothing set → `/var/www/html/./vendor/bin/phpunit` — the doubled `/./` is what `wrappers.go:82` composes today and is pinned, not fixed; `--filter AlgunTest` appended (`go_wrappers_test.sh:130`). The want log is `Resolve` + one `Property` per key in order + `Exec` |
| `TestWhatMysqldumpAsks` | success → `Resolve(cwd)`, `Dump{Dir: cwd, Path: …}`; `--json` → `data.path`; refusal at index 1 → `exitDocker` |
| `TestMysqldumpWithNoPathIsRefused` | no args and `[]string{""}` → `exitUsage`, empty log (`wrappers.go:95`) |
| `TestWhatVersionReports` | JSON document; the text block; a detached checkout with nothing underneath, which is the only coverage of `orUnknown`/`orMissing`. Log is `{Method: "Installed"}` alone — no `Resolve`, because `version` needs no project (`go_wrappers_test.sh:206`) |
| `TestAnOptionNobodyDeclaredIsAUsageError` | `version --tonteria` → `exitUsage`, empty log |

All `Exec` calls carry `Service: phpService` and `Options: terminalOptions("")`, compared with the
existing `cmpopts.IgnoreFields(core.ExecOptions{}, "Tty")`. The dump path is
`filepath.Join(t.TempDir(), "dump.sql")` so that even the RED step, where the real `Engine.Dump`
runs, cannot write outside the test. The text subtests pin `NO_COLOR=1` (`palette.go:11`) and
`COMMAND_BIN_NAME=hm`, so neither the terminal nor the invocation name decides the assertion.

### Decision 4: the version document is asserted minus `binary`

`buildOfThisBinary()` (`version.go:73`) reads `debug.ReadBuildInfo()`, which differs between a
`go test` binary and a released one. So: decode the envelope, take `binary` out, `cmp.Diff` the rest
against a literal map, then assert `binary == buildOfThisBinary()` separately. This is what the
parity suite already does — `jq -S 'del(.data.binary)'` plus `.data.binary != null`
(`go_wrappers_test.sh:192,199`). Rejected: a literal `"dev"` (wrong the moment `Version` is stamped
or VCS info is embedded); dropping the field (it is the one thing the shell half cannot answer).

### Decision 5: RED is a failing assertion, and it reaches no daemon

Step one adds the three interface methods, the fake's methods and answers, and **all** the tests,
while `wrappers.go:69,77,104` and `version.go:23` still call `engine(`. Step two moves those four
lines. Both steps sit inside one commit each, tests with the behaviour they verify.

What RED actually fails on, verified per method:

- **`tests()`** — `Engine.Property` is `e.properties().Load(project.Root)` (`:1230`), and
  `e.properties()` is a `fsprops.Reader` over two files (`:1385`); `installedRoot()` is
  `os.Executable` and two `filepath.Dir` (`:1452`). No registry, no `git`, no Docker. A root of
  `/code/shop` has no properties file, so `Load` returns `{}` (`fsprops/properties.go:53`) and the
  real engine answers `""`. RED is therefore two failures: the log has no `Property` entries, and
  the command reads `./vendor/bin/phpunit` where the fake's `BIN_DIR=bin` wanted `bin/phpunit`.
  `HM_STATE_DIR` pinning is not even required here — `answering` (`fake_engine_test.go:105`) does it
  anyway.
- **`mysqldump`** — the real `Engine.Dump` re-enters `Resolve` (registry, under the pinned
  `HM_STATE_DIR`) and then `e.database().Ready(project)`, which does reach for a daemon.
- **`version`** — the real `Installed()` spawns `git` (`gitvcs/vcs.go:173`) and reaches the daemon:
  `DockerVersion` is an API call, not a subprocess — `connect()` plus
  `ServerVersion(context.Background())` with no timeout of its own (`toolinfo/tooling.go:226-238`),
  and `connect()` honours `DOCKER_HOST` through `client.FromEnv` (`:126-133`). The subprocesses are
  `docker compose version --short` (`:46`) and `docker compose version` (`:244`), neither of which
  needs a daemon. So `DOCKER_HOST=unix:///nonexistent` is the only bound on `version`'s RED run: a
  context with no deadline is what would otherwise wait on a real socket. The document carries this
  checkout's own description instead of the fake's.
- **`purge`, `npm`, `n98-magerun`** — already routed through `projectOr` and `inside`, so no call
  site moves and no RED exists by construction. They are characterization coverage; the discipline
  that replaces RED is that `want` is written from the parity suite and `wrappers.go`, never from
  running the code and copying what came out.

**RED command**: `DOCKER_HOST=unix:///nonexistent go test ./internal/cli -short`. Two of the steps
touch a daemon — `mysqldump` through `database().Ready` (`dockergento.go:328`) and `version` through
`Installed`'s `ServerVersion` — so the unreachable socket is load-bearing for both, not a
precaution for one.
**GREEN command**: the same without the variable, plus `go test ./... -short`.

### Decision 8: MIGRATION.md is not touched

`MIGRATION.md:91-93` already says a Go-wired command is tested against `newEngine` rather than
Docker, in general terms, and the command table already marks all seven `go`. Adding seven names to
a document that deliberately does not list them would be noise.

## Data Flow

    purge / npm / magerun ──→ projectOr ──→ Resolve
                          └──→ inside ──────→ Exec
    tests ────────────────────→ Property × 1..2 ──→ inside ──→ Exec
    dump ─────────────────────→ Dump(here(), path)
    version ──────────────────→ Installed()          (no projectOr)
                                     ↓
                            newEngine ──┬─→ *dockergento.Engine (production)
                                        └─→ fakeEngine (ordered log + answers)

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/cli/engine.go` | Modify | `commands` gains `Property`, `Dump`, `Installed` |
| `internal/cli/wrappers.go` | Modify | `:69`, `:77`, `:104` route through `newEngine` |
| `internal/cli/version.go` | Modify | `:23` routes through `newEngine` |
| `internal/cli/fake_engine_test.go` | Modify | Three methods; `properties`/`installed`/`tooling` answers; `call.Key`, `call.Path` |
| `internal/cli/inside_test.go` | Create | The five one-container commands |
| `internal/cli/wrappers_test.go` | Modify | The two `mysqldump` tests |
| `internal/cli/version_test.go` | Create | The three `version` tests |

No file under `console/`, `bin/run`, `dockergento/` or `tests/` changes.

## Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | What each handler asks of the engine | `newEngine` substituted by `fakeEngine`; whole log compared with `cmp.Diff` |
| Unit | What each handler prints | `--json` decoded from a buffer; the text block compared with `NO_COLOR=1` |
| Integration | That the fake and the engine agree | Unchanged `tests/integration/go_wrappers_test.sh`, which compares both halves against a real container |

No `t.Parallel()`: `newEngine` and `t.Setenv` are process state, which is what the package already
assumes.

## Threat Matrix

`N/A` — no routing, VCS or PR automation, executable-file classification, or new subprocess is
introduced; the change removes subprocesses from the test path. The one adversarial concern, the two
RED steps that reach a daemon and the one that could write outside the test, is answered by decision
5's `DOCKER_HOST` and by the dump path living in `t.TempDir()`.

## Migration / Rollout

No migration. No command's arguments, output, `--json` document or exit codes change.

## Size and Commits

≈355 changed lines: `inside_test.go` ≈150, `version_test.go` ≈95, `wrappers_test.go` ≈62,
`fake_engine_test.go` ≈34, `wrappers.go` 6, `engine.go` 4, `version.go` 2. Budget 400, no
`size:exception`.

1. `test(cli): what the one-container commands ask of the engine` — `Property`, `wrappers.go:69,77`,
   `call.Key`, the `properties` answer, `inside_test.go`. ≈180 lines.
2. `test(cli): what mysqldump and version ask of the engine` — `Dump` and `Installed`,
   `wrappers.go:104`, `version.go:23`, `call.Path`, the `installed`/`tooling` answers, the
   `wrappers_test.go` additions and `version_test.go`. ≈175 lines.

Two rather than one because the split is the cut order made mechanical: reverting or never writing
commit 2 leaves the interface grown by `Property` alone and every test passing. Each commit is
self-consistent — `go test ./... -short` passes at both.

**Hard stop for apply.** Measure after each commit with
`git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'`. If the total after commit 1 exceeds
220, or commit 2 projects past 400, cut in the proposal's order — `version` first (drops
`Installed`, the two answers and `version.go:23`), then `mysqldump` (drops `Dump`, `call.Path` and
`wrappers.go:104`) — into `down-and-set-host-go-tests`. The `inside` family is never cut. Never
shrink by deleting comments, merging tests or compressing code; if both cuts fire and it is still
over, stop and report.

## Open Questions

- [ ] `wrappers.go:82` composes `/var/www/html/./vendor/bin/phpunit` when the project sets neither
      property. The test pins it because behaviour changes are out of scope; whether the fallbacks
      should compose cleanly is a follow-up.
- [ ] `dump` passes `here()` while the resolved project's root is available — the same
      `here()`-versus-`project.Root` inconsistency the predecessor left open for the copy commands.
      The tests make it visible; reconciling it is not this change.
