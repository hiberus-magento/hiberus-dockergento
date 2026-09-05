# Proposal: What the one-container wrappers, mysqldump and version ask of the engine

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessor `wrappers-go-tests`.

## Intent

`MIGRATION.md` states the porting rule: a command ported → its test in Go → then the shell twin is
deleted. The predecessor built the seam (`internal/cli/engine.go:17` `commands`, `:29` `newEngine`,
`fake_engine_test.go`) and used it for four commands. Nine Go-wired commands still have no Go test.
This change takes seven of them. The remaining two, `down` and `set-host`, become the named
follow-up `down-and-set-host-go-tests`.

**Why the split**: measured at the predecessor's density (43 lines per test, 6–10 per fake method —
exploration §Size), all nine come to ~530–545 changed lines. Yesterday's slice already spent a
`size:exception` at 457. Seven commands come to ~300–305, inside the 400-line budget, which is
requested with **no exception this time**. `down` and `set-host` carry the only real branching and
the one open shape decision (`Down`'s `(string, error)`), so they cut cleanly.

## Scope

### In Scope

- Go tests, on the existing seam, for `purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`,
  `mysqldump`, `version`.
- `commands` gains exactly three methods, verified against `dockergento/dockergento.go`:
  `Property(project core.Project, key string) string`, `Dump(dir, path string) error`,
  `Installed() (core.Installation, core.Tooling)`.
- Four call sites move from `engine(` to `newEngine(`: `wrappers.go:69`, `:77`, `:104`,
  `version.go:23`. Every other `engine(` site stays untouched.

### Non-goals (Out of Scope)

- `down` and `set-host` — deferred to `down-and-set-host-go-tests`.
- Deleting any Bash implementation or parity suite; porting any command.
- Any behaviour change: arguments, `--json` documents, exit codes and prompts stay as they are.
- Reconciling `here()` vs `project.Root` in `dump` — a known inconsistency; tests assert `Dir == cwd`
  so a reviewer does not read it as an oversight.
- Any test needing a Docker daemon.

## Capabilities

### New Capabilities

None. No user-visible behaviour changes.

### Modified Capabilities

- `go-entrypoint`: "A ported command's engine interaction is provable without Docker" (spec.md:192)
  gains scenarios for the seven commands; "A usage error returns before any engine call" (:227)
  broadens beyond the two copy commands to `mysqldump` and `version`.

## Approach

Test first, then the minimal seam growth that compiles it — strict TDD, as the predecessor did.

Two of the new methods return no error, so the outcome-by-call-index scheme does not reach them: the
fake gains **answer fields** (`properties map[string]string` keyed by property name, `installed`,
`tooling`) alongside the existing `outcomes`. `call` gains `Key` and `Path`. `core.Installation` and
`core.Tooling` are return values only and never enter the log, so `cmp.Diff` needs no new options.

Each test pins what the Bash parity suites assert (exploration §Behaviour to pin): the exact `rm -rf`
list for `purge`; argument pass-through for `npm` and the `bash -c "n98-magerun …"` join;
`binDir + "/phpunit --config ./dev/tests/unit/phpunit.xml.dist"` and the integration `cd` form, with
the `./vendor/bin` and `/var/www/html` fallbacks when `Property` answers `""`; usage errors with an
empty call log (`mysqldump` with no path, `version` with an unknown option); an engine refusal
mapping to `exitDocker`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/engine.go` | Modified | `commands` gains `Property`, `Dump`, `Installed` |
| `internal/cli/wrappers.go` | Modified | `:69`, `:77`, `:104` route through `newEngine` |
| `internal/cli/version.go` | Modified | `:23` routes through `newEngine` |
| `internal/cli/fake_engine_test.go` | Modified | Three fake methods, answer fields, two `call` fields |
| `internal/cli/inside_test.go` | New | The five commands that reach the container through `inside` |
| `internal/cli/wrappers_test.go` | Modified | `mysqldump`'s tests, beside the copy tests they are shaped like |
| `internal/cli/version_test.go` | New | `version`'s tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `Property` is called twice in `tests()` with different keys (`BIN_DIR`, `WORKDIR_PHP`) | Med | Fake answers from a map keyed by key, not by call index; the log records `Key` so order is still visible |
| `version` needs `Installed`/`Tooling` answers a call log cannot carry | Med | Dedicated answer fields on the fake; assert on rendered output and the JSON document |
| Literate style inflates the diff past 400 lines | Med | Density measured at 43 lines/test; hard stop at 400 with the cut order below — never shrink by deleting comments |
| Interface drift as `Engine` grows | Low | `var _ commands = (*dockergento.Engine)(nil)` (engine.go:25) fails loudly |
| A fake that agrees with the handler and not with the engine | Med | Assert the argument strings the Bash parity suites already assert, not a rewritten expectation |

**Cut order if authoring runs long** (move to `down-and-set-host-go-tests`, in this order):
1. `version` (~55 lines) — self-contained; drops `Installed`, the `installed`/`tooling` answers and
   `version.go:23` from the slice entirely. **Applied on 2026-09-05**: the size gate measured 247
   lines after the first commit's content, so `version` moved to `down-and-set-host-go-tests`; this
   change delivers the other six commands at 359 lines.
2. `mysqldump` (~55 lines) — drops `Dump`, the `Path` field and `wrappers.go:104`.
The `inside` family (`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`) is never cut: it
shares one fake shape and is the cheapest coverage per line.

## Rollback Plan

One commit on `release/2.0.0`; `git revert` restores it. Everything is inside `internal/cli`, and the
tests are additions, so reverting removes them with nothing attached.

## Dependencies

None. **No impact on existing projects and no migration step for users**: no command's arguments,
output, `--json` document or exit codes change. Proven by the untouched Bash parity suites and
`go build ./...`.

## Success Criteria

- [ ] Each of the seven commands has a Go test that fails first, then passes, under
      `go test ./internal/cli -short` with no daemon, no network and no project beyond `t.TempDir()`.
- [ ] `go test ./... -short` passes (baseline 188 in 22 packages; `./internal/cli` baseline 28,
      re-confirmed at HEAD `5e9f443`).
- [ ] `tests/run.sh unit` passes, unmodified.
- [ ] `gofmt -l ./cmd ./internal` is empty; `go vet ./...` is clean.
- [ ] `var _ commands = (*dockergento.Engine)(nil)` still compiles after the interface grows.
- [ ] Exactly the four named call sites route through `newEngine`; no other `engine(` site changed.
- [ ] No file under `console/`, `bin/run`, or `dockergento/` is modified.
- [ ] Changed lines ≤ 400, measured over the source commits with
      `git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'` and nothing added to it: a new
      file's lines are already its insertions, so adding `wc -l` would count them twice — **no
      `size:exception` this time**.
- [ ] Manual verification on a real project, kept light: `hm version` and `hm version --json`, then
      `hm purge` on a running project. Nothing that moves media.
