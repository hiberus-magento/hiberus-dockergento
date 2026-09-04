# Proposal: A test seam for the wrappers, and the commands nothing covers

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`.

## Intent

`MIGRATION.md:62-64` states the porting rule: **a command ported → its test in Go → then the shell
implementation and its bash test are deleted**, in that order. Fourteen commands are wired in Go
with no Go test of what they ask of the engine. Four of them — `copy-to-container`,
`copy-from-container`, `varnish-on`, `varnish-off` — have no coverage of any kind anywhere in the
repository. Retiring their Bash twins on that evidence is deleting the only description of the
behaviour that exists.

The obstacle is structural: `internal/cli/engine.go:20` builds a concrete `*dockergento.Engine` per
call, and its adapters are assembled inside the engine (`e.properties()`, `gitvcs.Git{}`), so no
fake can be injected. ADR-007 sells the hexagon precisely on this — *"la reconciliación y el GC se
prueban **sin Docker**, en milisegundos"* — and the CLI layer is where that stopped being true.

## Scope

### In Scope

- A test seam in `internal/cli` that lets a handler's engine calls be observed without Docker.
- A fake implementing it, in `_test.go`, in the package's literate style.
- Tests for the four commands with zero coverage: `copy-to-container`, `copy-from-container`,
  `varnish-on`, `varnish-off` — including varnish's choreography (VCL edit → restart → `cache:enable`
  / `cache:disable`, and off's cascade into `purge` + `cache:clean`).
- Debt: `internal/cli/php.go:96` claims `copy-to-container` "is not ported"; it is
  (`run.go` wires it to `copyInto`). The real reason mac Composer stays in shell is the vendor-mirror
  flow gated by `mirrorsVendor` (php.go:98).
- Debt: `tests/unit/migration_status_test.sh:34,42` use `printf|grep -qx` and `commands|grep -qx`
  under `set -uo pipefail`; `grep -q` can SIGPIPE the producer. Here-strings, as `sync-migration-status`
  already did for the reverse check.

### Out of Scope (Non-goals)

- Deleting any Bash implementation or its suite; porting any new command.
- The nine commands that already have parity coverage — deferred to `remaining-wrappers-go-tests`:
  `set-host`, `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `down`,
  `version`. `setup` needs nothing: `core.ParseSetup` is covered by `dockergento/core/setup_test.go`.
- Changing `dockergento`'s public API, unless design proves candidate (iii) below is the smaller
  change.
- Any test that needs a Docker daemon, and any change to the `--json` or exit-code contract.

## Capabilities

### New Capabilities

None. No user-visible behaviour changes.

### Modified Capabilities

None expected. If design chooses seam candidate (iii), `library` gains a requirement about
constructor-level port injection and this change must return to the spec phase.

## Approach

Write the test first against an interface that does not exist yet, then add the minimal seam that
makes it compile. That is what strict TDD means here: the seam is production code introduced by a
failing test, not before it.

Design chooses the seam. Candidates, with the constraint that ADR-007 line 397 says the CLI
"no importa ya nada más que la fachada y los tipos del dominio":

| # | Shape | Trade-off |
|---|---|---|
| i | Narrow consumer-side interface in `internal/cli` naming only what these handlers call (`Exec`, `Restart`, `CopyInto`, `CopyFrom`), plus a package-level factory `var` the test replaces | **Recommended.** Test-only, no public API change, Go's consumer-defined-interface convention, `*dockergento.Engine` satisfies it structurally. Blast radius: the in-scope call sites. |
| ii | Make `engine()` itself a `var` returning an interface | The interface must then name every method the package uses — verified: ~35 across 20 files — and the fake must stub all of them. |
| iii | Port injection in `dockergento.Options` | Truest to the hexagon, but a public API change to the facade, touching every consumer, for a test problem. |

Two mechanics design must settle: `projectOr()` (engine.go:58) and `here()` (engine.go:69) read the
process working directory, so either `Resolve` joins the seam or the test builds a real project with
`t.TempDir()` + `t.Chdir` — `Resolve` is filesystem and git only, no daemon. Go is 1.25, so `t.Chdir`
is available; nothing in the tree uses it yet.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/engine.go` (or a new `seam.go`) | Modified/New | The seam and its factory var |
| `internal/cli/wrappers.go` | Modified | Call sites in `copyInto`, `copyFrom`, `varnish` reach the engine through the seam |
| `internal/cli/wrappers_test.go` | New | The fake and the four commands' tests |
| `internal/cli/php.go` | Modified | Correct the `mirrorsVendor` comment (lines 91-97) |
| `tests/unit/migration_status_test.sh` | Modified | Two here-strings |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Interface drift as `Engine` grows methods | Med | Keep it narrow; a compile-time `var _ seam = (*dockergento.Engine)(nil)` assertion fails loudly |
| Literate comment density pushes the diff past 400 lines | Med | Two slices, this one estimated ~310 lines; if it still overruns, report the overage — never shrink by deleting comments |
| The seam changes runtime behaviour | Low | Behaviour-preserving by construction: same call, one indirection. Proven by the untouched parity suites and `go build ./...` |
| A fake that agrees with the handler and not with the engine | Med | Assert on arguments actually passed (service, argv, order), not on a rewritten expectation |

## Rollback Plan

Single commit on `release/2.0.0`; `git revert` restores it. The seam is confined to `internal/cli`
and the tests are new files, so reverting removes them with nothing else attached.

## Dependencies

None. **No impact on existing projects and no migration step for users**: no command's arguments,
output, `--json` document or exit codes change, and the Bash parity suites are untouched, which is
how that is proven.

## Success Criteria

- [ ] `go test ./internal/cli -short` covers each in-scope command's engine interaction through a
      fake, table-driven, with no daemon — and fails first, before the seam exists.
- [ ] `go test ./... -short` passes (baseline: 172 tests, 22 packages).
- [ ] `tests/run.sh tests/unit` passes, including the two repaired assertions.
- [ ] `gofmt -l ./cmd ./internal` is empty and `go vet ./...` is clean.
- [ ] No file under `console/`, `bin/run`, or `dockergento/` is modified.
- [ ] `MIGRATION.md`'s test conventions (lines 74-90) still describe what was written.
- [ ] Changed lines stay under the 400-line review budget.
