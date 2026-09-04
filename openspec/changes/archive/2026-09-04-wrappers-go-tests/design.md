# Design: A test seam for the wrappers

## Technical Approach

A consumer-side interface in `internal/cli` naming the five methods the in-scope handlers ask of
`*dockergento.Engine`, reached through a package-level factory variable a test replaces. Production
keeps `engine()` (`engine.go:20`) and its concrete return type; only the five call sites that must be
observable go through the variable. Proposal candidate (i); (ii) and (iii) are rejected below.

## Architecture Decisions

### Decision: the interface, and `Resolve` in it

```go
// commands is what a command asks of the tool. Narrow on purpose: it names what these handlers
// call and nothing else, so a test can answer for the engine without a daemon.
type commands interface {
	Resolve(dir string) (core.Project, error)
	Exec(dir, service string, command []string, options core.ExecOptions) (int, error)
	Restart(dir string, services []string) error
	CopyInto(dir string, paths []string, all bool) error
	CopyFrom(dir string, paths []string) error
}

var _ commands = (*dockergento.Engine)(nil)
```

Signatures quoted from `dockergento/dockergento.go:129,271,293,508,529`. It lives in `engine.go`,
beside the factory whose return type it is.

`Resolve` belongs in it. Every in-scope handler enters through `projectOr` (`engine.go:51`), so the
alternative is a real project on disk for every test. That route costs more than it looks:
`Engine.Resolve` builds `app.Resolver` eagerly, and `e.registry()` (`dockergento.go:1403`) opens
SQLite at `~/.hm/hm.db` whenever `HM_STATE_DIR` is unset (`dockergento.go:1428`) — a unit test
writing into the developer's home. It also spawns `git rev-parse` per call
(`dockergento/adapters/gitvcs/vcs.go:26`), needs `config/docker/properties.json` under the temporary
root (`dockergento/adapters/fsprops/properties.go:41`), resolves that root from the working
directory — so a `TMPDIR` inside a checkout moves it — and `t.Chdir` forbids `t.Parallel()`. Faked,
the whole test is memory.

### Decision: the factory variable

| Option | Trade-off | Decision |
|---|---|---|
| Keep `engine()`, add `var newEngine = func(...) commands { return engine(...) }` | 5 call sites move; 39 keep the concrete type | **Chosen** |
| Make `engine()` itself return the interface | The interface must name ~35 methods, and `api.Server.Engine` is `*dockergento.Engine` (`internal/api/server.go:30`), so it stops compiling | Rejected |
| Ports in `dockergento.Options` | Public API change to the facade for a test problem | Rejected |

Moving: `engine.go:58` (`projectOr`), `wrappers.go:150,219,244`, `php.go:59` (`inside`). `inside` is
required — varnish-off's cascade runs through `purge` and `magento`, and both reach the engine there;
it also leaves the seam routed for the nine commands of `remaining-wrappers-go-tests`. Behaviour is
unchanged by construction: the same object built by the same function, one static type wider at the
call site, and `engine()` never returns nil.

### Decision: an ordered call log, not per-method slices

`dockergento/app/orchestrate_test.go:11` records per method, which loses the interleaving of `Exec`
and `Restart` — and for varnish the order *is* the behaviour. So one `[]call` in call order, each
entry naming the method and the arguments that matter, compared whole with `cmp.Diff`
(`go-cmp` is already a direct dependency). Comparing the complete log is also how an unexpected call
fails a test: it appears in the diff, and a usage error asserts an empty log. Outcomes come from
`outcomes []outcome{status, err}` indexed by call number, past the end meaning success. The fake
lives in `internal/cli/fake_engine_test.go`, so it never reaches the binary.

## Data Flow

    handler ──→ newEngine(...) ──→ commands ──┬─→ *dockergento.Engine  (production)
       │                                      └─→ fakeEngine           (test, records calls)
       └──→ projectOr ──→ newEngine(...).Resolve

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/cli/engine.go` | Modify | `commands`, `newEngine`, the compile-time assertion; `projectOr` routed |
| `internal/cli/wrappers.go` | Modify | Three call sites (`:150`, `:219`, `:244`) |
| `internal/cli/php.go` | Modify | `inside` routed (`:59`); the `mirrorsVendor` comment (`:96`) corrected |
| `internal/cli/fake_engine_test.go` | Create | `call`, `outcome`, `fakeEngine`, and the `answering` helper, which also pins `HM_STATE_DIR` to a temporary directory |
| `internal/cli/wrappers_test.go` | Create | The copy and varnish tests |
| `tests/unit/migration_status_test.sh` | Modify | Here-strings at `:34` and `:42` |
| `MIGRATION.md` | Modify | One bullet: a Go-wired command is tested against `newEngine`, not Docker |

`MIGRATION.md` and the split of the tests into two files are additions to `proposal.md`'s Affected
Areas table, which names one `wrappers_test.go` and no document. Neither adds behaviour, and the
proposal already requires MIGRATION.md's test conventions to describe what was written
(`proposal.md:115`) — this is what makes that true rather than merely unbroken.

## Testing Strategy

| Test | Cases |
|---|---|
| `TestWhatCopyingIntoTheContainerAsks` | named paths; `--all` as first argument; `--all` not first (`all` stays false, both still passed as paths) |
| `TestCopyingWithNoPathIsRefused` | into and out of: `exitUsage`, empty call log — not even `Resolve` |
| `TestACopyTheEngineRefusesIsReported` | `CopyInto`/`CopyFrom` error → `exitDocker` (`orchestrate.go:241`) |
| `TestWhatCopyingOutOfTheContainerAsks` | paths as given, `Dir` = `here()` |
| `TestWhatVarnishAsks` | `on`: Resolve, `sed` on `varnish` as root, `Restart`, `cache:enable full_page`. `off`: the same with `cache:disable`, then Resolve + `rm -rf` (purge) and Resolve + `cache:clean` |
| `TestAFailedEditStopsBeforeTheRestart` | outcome 2 fails → log ends at the `sed` |
| `TestOutsideAProjectNothingIsAsked` | empty `core.Project` → `exitProject`, log is `Resolve` alone |

Assertions carry the literal `#skip-varnish` sed expressions: they are the only description in Go of
the two edits being inverses. `core.ExecOptions.Tty` is compared with `cmpopts.IgnoreFields`, since it
reads the process's own stdin. No `t.Parallel()`: `newEngine` is process state, restored by
`t.Cleanup`, which is what the package already does.

**TDD sequencing.** RED is a failing assertion, not a compile error: step one adds the interface, the
variable, the fake and the tests **and routes `projectOr` only**. The package compiles, the test runs,
and the log holds `Resolve` alone where the want holds the whole choreography. Step two moves the four
handler call sites and it goes green. Both steps are one commit, tests with the behaviour they verify.

During RED the handlers themselves are still unrouted, so each builds the real engine and its first
method call re-enters `Engine.Resolve`: the registry database is opened and `git rev-parse` is run
before anything else happens. It does not stop there — a missing `config/docker/properties.json` is
not an error (`dockergento/adapters/fsprops/properties.go:62-64`), so `Resolve` answers with a
nameless project and the call carries on into the Docker adapters, where it fails. That is acceptable
for a step that runs once and is never committed, but the database must not be the developer's: the
shared `answering` helper sets `HM_STATE_DIR` to `t.TempDir()` with `t.Setenv` before installing the
fake, so no test touches `~/.hm` in RED or in GREEN. `t.Setenv` is also why these tests cannot be
parallel, which they already were not.

## Threat Matrix

`N/A` — no routing, VCS or PR automation, executable-file classification, or new subprocess is
introduced; the change removes subprocesses from the test path. The one adversarial concern found,
`Resolve` spawning `git` and opening `~/.hm/hm.db` from a unit test, is what decision 1 answers.

Platform: `copy-to-container` and `copy-from-container` only matter on macOS, where the code lives in
a volume; the tests are platform-independent because the engine is faked, so they run on Linux CI too.

## Migration / Rollout

No migration. No command's arguments, output, `--json` document or exit codes change.

## Size and Commits

≈315 changed lines (engine.go ≈20, wrappers.go 6, php.go 10, fake and helper ≈80, tests ≈190,
shell 8, MIGRATION.md 2), under the 400 budget. If it overruns, cut in this order: merge the two
varnish failure tests into one table, then drop the `--all`-not-first case, then move
`copy-from-container` to a second slice. Never comments.

1. `test(cli): what the copy and varnish commands ask of the engine` — seam, fake, tests, five call
   sites, the MIGRATION.md bullet.
2. `docs(cli): say why the mac Composer flow is still shell` — `php.go:91-97`.
3. `test(migration): read the command lists without a broken pipe` — the two here-strings.

## Open Questions

- [ ] `wrappers.go:155,166` return `report(..., err)` when `status != 0` and `err == nil`, which is
      `exitOK`: varnish reports success after a `sed` that failed. A real defect, found here, and
      fixing it is a behaviour change this change forbids. It needs a follow-up; no test pins it.
- [ ] `copyInto`/`copyFrom` pass `here()` while `varnish` passes `project.Root`. The tests make the
      difference visible (the fake's project has a distinct root); reconciling it is not this change.
