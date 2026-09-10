# Proposal: What `list`, `describe` and `logs` ask of the engine

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as the
predecessors `wrappers-go-tests`, `remaining-wrappers-go-tests`, `down-and-set-host-go-tests` and
`retire-ported-shell-commands`.

## Intent

`MIGRATION.md:62-64` states the rule in three steps: **a command ported → its test in Go → then the
shell implementation and its bash test are deleted**, in that order. Twelve commands completed all
three in `retire-ported-shell-commands`. Five did not: they are wired in Go, have no runtime caller
left, and hold their shell twins hostage because step two is missing.

Three of those five are genuinely read-only — `list`, `describe`, `logs` — and they are exactly the
three of the five that `data/command_descriptions.json` declares `"safety": "safe"` (`:392-396`,
`:261-262`, `:431-432`). The other two are declared `"dangerous"` (`bash` at `:79-80`, `masquerade`
at `:449-450`) and are deferred with reasons, below.

This change writes step two for those three. It deletes nothing.

## Scope

### In Scope

- Go tests on the existing `newEngine` seam for `list`, `describe` and `logs`, table-driven, no
  daemon, no network, no project beyond `t.TempDir()`.
- `commands` (`internal/cli/engine.go:17-30`) grows by exactly **three** methods, each verified
  against `dockergento/dockergento.go`:
  `Environments() ([]core.Environment, error)` (`:140`),
  `Describe(dir string, withSecrets bool) (core.Description, error)` (`:152`),
  `Logs(dir string, services []string, options core.LogOptions) error` (`:322`).
- Three call sites move from `engine(` to `newEngine(`: `list.go:23`, `describe.go:25`,
  `orchestrate.go:166-167`. **No other `engine(` site changes** — in particular not
  `orchestrate.go:200`, see the decision below.
- `fakeEngine` gains three methods; `call` gains `WithSecrets bool` and
  `LogOptions core.LogOptions`; two answer fields (`description core.Description`,
  `environments []core.Environment`) in the shape `project` and `stopped` already have.
- The delta for `go-entrypoint` (below).

### Non-goals (Out of Scope)

- **Deleting any shell twin.** `console/commands/list.sh`, `describe.sh` and `logs.sh` stay, along
  with their bash suites. That is a later cycle, exactly as `retire-ported-shell-commands` was for
  the twelve before these. The lines this unlocks are the payoff, not the work.
- **`bash`, `exec` and `masquerade`** — their own cycle, for three reasons on record: all three are
  declared `"dangerous"`; `bash` (`tools.go:10-18`) is a wrapper over `execute()`, which is also the
  `exec` handler (`orchestrate.go:171-195`), so the two are joined at one call site and have to move
  together; and `masquerade`'s decline path needs `os.Stdin` redirected in the test, the single most
  awkward piece in the estimate. That cycle also closes a real gap — `exec` has no Go test at all
  today.
- Any behaviour change. Arguments, `--json` documents, exit codes and text stay as they are.
- Growing the seam with `Configuration` — decided below, with its cost.
- Any test that needs a Docker daemon.

## Decision 1 — `checkServices` stays off the seam

`logs`' unknown-service validation (`orchestrate.go:197-221`) calls
`engine(nil, nil, jsonOutput).Configuration(project)` at `:200`, which is not on `commands`.
`Configuration` (`dockergento.go:1172-1175`) only loads compose YAML from `project.Root`, so it
reaches no daemon and needs no RED pin. But `:201-203` treats **any** error as "skip validation", so
against a fake project with a fictitious root the `unknown_service` branch at `:215-217` can never
fire.

**Decision: do not add it. The seam stays at three methods.**

| | Cost | Consequence |
|---|---|---|
| Add `Configuration` | ~40-50 lines: 1 in `engine.go`, ~10 in the fake (method + `configuration core.Compose` answer field), 1 routing line at `:200`, ~30 for the subtest with its `core.Compose`/`core.Service` fixtures and the hint assertion | One more branch proved, on a `logs` unit already at ~270-310, in a cycle already over the review budget in aggregate |
| Leave it | 0 | The branch stays unproved *in Go* |

What still covers it, checked rather than assumed:

- `tests/integration/logs_launch_version_test.sh:148-153` runs the **Go** binary (`run_in_lab`,
  `:18-24`) in a lab project, asserts exit 5 and that the refusal names `phpfpm`. It sits **above**
  the `docker info` gate at `:159`, so it runs with no daemon. It survives the deletion cycle.
- `tests/integration/go_orchestration_test.sh:176-181` additionally compares the Go and shell
  refusals as sorted JSON. This one dies with `logs.sh`, which is why the natural owner of
  `Configuration` is that deletion cycle — it is the change that removes the parity half.

One consequence must be written into `logs_test.go` as a comment, not discovered later: with
`Configuration` unrouted, a `logs <service>` test still reaches the real `engine()` inside
`checkServices`, which fails to load compose YAML at the fake project's root and skips validation.
That is deterministic **only while the test does not create compose files under `f.project.Root`**.

## Decision 2 — `list` is the one command with no RED-window net

`answering(t)` (`fake_engine_test.go:212-229`) pins six things because a RED run reaches the real
engine while a call site is unrouted.

**Superseded by the design's correction, and left here as the record of a wrong reason.** This
section originally claimed `Describe` and `Logs` were safe by construction because each calls
`e.Resolve(dir)` first and an empty temp cwd would fail them. `Resolve` does not fail:
`dockergento.go:127-129` says so in its own comment — "a directory that is not a project is not an
error… the answer is a project with no name". So `Describe` proceeds, and `app/describe.go:58-70`
fans out all seven of its questions concurrently, inspecting no error until `asking.Wait()` returns.
Two of those seven are `d.Engine.Containers()` and `d.Tooling.Xdebug(project.Name)`.

`describe`'s RED window therefore reaches the daemon exactly as `list`'s does, and worse: with a
nameless project, `toolinfo/tooling.go:84-85` matches any container whose `hm.project` label is
empty, so `Xdebug` can select an unrelated live `phpfpm` and `ContainerExecCreate` into it. The
conclusion below still holds — the `DOCKER_HOST` pin is sufficient and no new pin is needed — but it
holds for one reason rather than two, and nobody should skip that pin on the strength of `Resolve`.

**`Environments()` takes no directory at all.** `list.go:23` calls it with nothing, and
`dockergento.go:140-146` goes straight to `app.Inventory{Engine: dockerd.Engine{...}}.Environments()`
— whose first act is a Docker SDK `ContainerList` across the whole machine, scoped to no project.
Its only protection is the `DOCKER_HOST=unix:///nonexistent` pin at `:218`. On a machine with live
containers — which is the machine this runs on — an unpinned RED run for `list` reaches the real
daemon.

This is an acceptance criterion on the test-authoring, not trivia. See Success Criteria.

## Decision 3 — the `Dir` a test must expect

`logs` calls its engine method with `here()` again (`orchestrate.go:166-167`) rather than with the
`project.Root` the fake already answered from `projectOr` (`engine.go:84`). `here()`
(`engine.go:94-102`) is `os.Getwd()`, so the recorded `Dir` is the **test's working directory** —
the temp dir `answering` chdir'd into — and not `f.project.Root`.

`inside()` does the opposite, and `inside_test.go` asserts `Dir: root` accordingly. **Copying that
pattern here silently produces a passing test that asserts the wrong thing.** The expected `Dir`
must be read from `os.Getwd()` inside the test.

The three call logs differ, and each test asserts the whole log:

| Command | Recorded calls |
|---|---|
| `list` | `Environments` only — no `Resolve`, no dir |
| `describe` | `Describe` only — `describe.go` never calls `projectOr`, so `f.project` is irrelevant to it |
| `logs` | `Resolve`, then `Logs` — two entries |

## Capabilities

### New Capabilities

None. No user-visible behaviour changes.

### Modified Capabilities

- `go-entrypoint`: **"A ported command's engine interaction is provable without Docker"**
  (`openspec/specs/go-entrypoint/spec.md:192-347`) gains scenarios for `list`, `describe` and
  `logs`; its "Previously covered" roster (`:198-201`) grows by those three.
- `go-entrypoint`: **"A usage error returns before any engine call"** (`:348-374`) — the subject
  list at `:350-351` broadens to include the three. Each refuses above the engine call:
  `list.go:18-21` (any argument), `describe.go:19-22` (unknown option), `orchestrate.go:139-143`
  (unknown option), plus `logs`' `missing_value` at `:101-105` and the non-numeric `--index` at
  `:117-122`.

### Governing but not modified

- `go-entrypoint`: **"The test seam changes nothing a real invocation runs"** (`:376-390`) — the
  three new routings conform; `newEngine` still resolves to `engine` in production
  (`engine.go:36-38`).
- `go-entrypoint`: **"A command proved in Go has no shell twin"** (`:403-441`) — deliberately **not**
  amended. Its roster grows in the deletion cycle, not this one.
- `environment-introspection:6-30` (describe), `:44-68` (the machine inventory),
  `project-description:32`, `environment-orchestration:157-175` (reading the logs) already own the
  behaviour. Nothing about it changes, so restating it would be spec noise.

## Approach

Strict TDD, as all three predecessors did: the test is written against an interface method that does
not exist yet, and the seam grows to compile it. `var _ commands = (*dockergento.Engine)(nil)`
(`engine.go:32`) needs no edit — all three methods already exist on the real engine.

`answering(t)` needs **no new pin**: the six it already has cover these three, `DOCKER_HOST` doing
the load-bearing work for `list`. What each test pins:

- **`list`**: any argument is a usage error with an empty log; the happy path records exactly one
  `Environments` call; the text table and the `--json` document from a fixed `[]core.Environment`,
  including the empty case (`list.go:55-60`), the orphan and no-metadata markers (`:73-79`) and
  `orDash` (`:93-99`); an error answers `exitDocker`/`docker_unavailable` (`:25-27`).
- **`describe`**: `--with-secrets` sets `WithSecrets` and anything else is a usage error with an
  empty log; the `--json` document; the text rendering including the credentials block only when
  `description.Credentials != nil` (`describe.go:91-97`); an error answers
  `exitProject`/`project_not_configured` (`:27-29`).
- **`logs`**: every flag's effect on `core.LogOptions`, including `--tail`/`-n` sharing a field;
  positional arguments becoming services; `missing_value` and the non-numeric `--index`, both with
  an empty log; `Dir` equal to the test's cwd, per Decision 3.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/cli/engine.go:17-30` | Modified | `commands` gains `Environments`, `Describe`, `Logs` |
| `internal/cli/list.go:23` | Modified | Routes through `newEngine` |
| `internal/cli/describe.go:25` | Modified | Routes through `newEngine` |
| `internal/cli/orchestrate.go:166-167` | Modified | Routes through `newEngine`; `:200` deliberately unchanged |
| `internal/cli/fake_engine_test.go` | Modified | Three fake methods, two `call` fields, two answer fields |
| `internal/cli/list_test.go` | New | `list`'s tests |
| `internal/cli/describe_test.go` | New | `describe`'s tests |
| `internal/cli/logs_test.go` | New | `logs`' tests |
| `openspec/specs/go-entrypoint` | Modified | Delta, two requirements |

## Delivery

Three work units, one per command, each growing the interface by exactly what it needs, routing
exactly its own call site, and leaving the tree green on its own:

| # | Unit | Floor |
|---|---|---|
| 1 | `list` — the `DOCKER_HOST` note lands with it | ~230-260 |
| 2 | `describe` | ~280-330 |
| 3 | `logs` | ~270-310 |
|   | **Cumulative** | **~780** |

**These are floors, not estimates.** Every recent cycle in this repo overshot its own figure by
1.3-2x: `down-and-set-host-go-tests` 418 → 609, `wrapper-exit-codes` 50-90 → 184,
`down-json-and-flags` 120-180 → 302. The honest band is ~1000-1200.

Each unit is comfortably inside the 400-line review budget **alone**. The cumulative is not, and
saying otherwise would be a claim the arithmetic does not support. **The delivery decision therefore
belongs before apply, not mid-apply** — under the cached `ask-on-risk` strategy, the recommendation
is three chained PRs in the order above, each self-contained, rather than one `size:exception`.

Measured with `git diff --shortstat <parent>..HEAD -- . ':(exclude)openspec'` against **this
change's own parent** at the time it is applied. A fixed hash written into this document goes stale
the moment another change lands first, and has already silently measured unrelated commits twice in
this repository.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| An unpinned RED run for `list` enumerates the developer's live containers | High if `answering(t)` is skipped | Decision 2; the criterion below; a comment in `list_test.go` naming `Environments()`'s missing dir argument |
| `logs_test.go` copies `inside_test.go` and asserts `Dir: root` | **High** — it is the adjacent precedent | Decision 3, stated before the test is written, with the cwd/`project.Root` divergence named at `orchestrate.go:166-167` vs `engine.go:84` |
| A `logs <service>` test grows compose files at the fake root and silently starts exercising `checkServices` | Low | Criterion below forbids it; the reason is in the test file's own comment |
| The cumulative overruns the budget mid-apply | Certain at ~780+ | Three chained PRs decided **before** apply, not after a gate trips |
| A test agrees with the handler instead of with the engine | Med | Assert the arguments actually passed, read from the `dockergento.Engine` signatures cited above, not a rewritten expectation |
| Interface drift as `Engine` grows | Low | `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:32`) fails loudly at fifteen methods as it did at twelve |

## Rollback Plan

Up to three commits on `release/2.0.0`; `git revert` restores them in reverse order. Everything is
inside `internal/cli`, three of the four files are new, and the three routing edits are one line
each, so reverting removes the tests with nothing attached. No shell file is touched, so no
deletion has to be undone.

## Dependencies

None. **No impact on existing projects and no migration step for users**: no command's arguments,
output, `--json` document or exit codes change. Proven by the untouched Bash parity suites and
`go build ./...`.

## Success Criteria

- [ ] Each of `list`, `describe` and `logs` has a Go test that **fails first**, then passes, under
      `go test ./internal/cli -short` with no daemon, no network and no project beyond `t.TempDir()`.
- [ ] **Every test that can reach `list` runs under `answering(t)`**, and no subtest overrides
      `DOCKER_HOST`. `Environments()` takes no directory, so the `unix:///nonexistent` pin is the
      only thing between a RED run and this machine's live containers.
- [ ] `logs_test.go` expects `Dir` equal to the test's working directory read from `os.Getwd()`, and
      **not** `f.project.Root`. A comment at that assertion names why it differs from
      `inside_test.go`.
- [ ] No test creates a compose file under the fake project's root, so `checkServices` keeps
      skipping deterministically.
- [ ] `commands` has exactly fifteen methods; `Configuration` is **not** among them, and
      `orchestrate.go:200` still calls the concrete `engine(`.
- [ ] Exactly three call sites route through `newEngine`: `list.go`, `describe.go`,
      `orchestrate.go`'s `logs`. No other `engine(` site changed.
- [ ] No file under `console/`, `bin/run`, `data/` or `dockergento/` is modified, and no shell twin
      or bash suite is deleted.
- [ ] `go test ./... -short` passes; `tests/run.sh unit` passes, unmodified;
      `gofmt -l ./cmd ./internal` is empty; `go vet ./...` is clean. Baselines are re-read at apply,
      never carried from this document.
- [ ] `tests/integration/logs_launch_version_test.sh` and `go_orchestration_test.sh` are unmodified
      — they are what still covers `unknown_service`.
- [ ] Changed lines measured with the shortstat command above against this change's own parent, with
      the three-PR split applied, and no `size:exception`.
- [ ] Manual verification on a real project, all read-only: `hm list`, `hm list --json`,
      `hm describe`, `hm describe --json`, `hm logs --tail 5`, `hm logs no-existe` (still exit 5),
      and the usage errors `hm list algo`, `hm describe --tonteria`, `hm logs --tail`.
