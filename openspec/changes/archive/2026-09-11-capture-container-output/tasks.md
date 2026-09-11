# Tasks: `capture-container-output`

Read together with `proposal.md` (scope, Decisions 1-4, Success Criteria) and `specs/README.md` (why
no delta spec is written). No `design.md` exists for this change — the user chose "specs only,
straight to tasks" because no user-visible fork remained open; the concrete shapes below are this
document's own design detail, not a separate design phase.

Scope reminder, not re-litigated here: `internal/cli` has no diff, no `commands` method, no
`fakeEngine` field. Everything lands in `dockergento/app` and `dockergento/dockergento.go`.

## Baseline (do first, sequential)

- [x] **T0.1** Record this change's own parent commit: `git rev-parse HEAD` on `release/2.0.0`
      before the first edit. Use it as `<parent>` in every size/diff check below via
      `git merge-base HEAD <parent>` at check time — never a hash copied from this or an earlier
      proposal, which goes stale. *(Verifiable here.)*

      Parent recorded: `89690483a95be6dcbfbc951fa36135a4633c4bbe`.
- [x] **T0.2** Confirm the tree is currently green: `go build ./...`, `go test ./...`, `gofmt -l
      ./cmd ./internal`, `go vet ./...`. This is the "before" baseline the later diff-based checks
      compare against. *(Verifiable here.)*

      Confirmed green: build clean, 408 tests passed in 24 packages, gofmt clean, vet clean.

## Slice A — generalised resolution + `Database.container` refactor

Requirement link: proposal Decision 1 (shared resolution, no third drifting copy) and Success
Criteria "`dockergento/app/database_test.go`'s five tests pass byte-unchanged." Independent,
self-contained slice: leaves the tree green and complete on its own, and is the clean commit
boundary the proposal asked to keep ready.

No new test-writing step here under Strict TDD: this slice introduces no new *observable* behaviour
— it is a behaviour-preserving extraction, and its proof is the five existing tests staying green
throughout, unedited. Where Strict TDD's RED step applies is Slice B, which is genuinely new
behaviour.

- [x] **A1** Add the shared resolution helper in `dockergento/app`, in a new file
      (`dockergento/app/resolve.go` suggested — a name distinct from the existing `container`
      *methods* on `Database` and `Transfer`, to avoid a same-name function/method pair reading as
      a typo). Signature shape:

      **Correction: not `resolve.go`.** That filename was already taken by a pre-existing,
      unrelated `Resolver` type (project/worktree resolution, with its own `resolve_test.go`,
      predating this change). Writing over it would have clobbered ~120 lines of unrelated code —
      caught via a build error (`undefined: app.Resolver`) before it was committed, and the
      original file was restored via `git checkout`. Landed instead as
      `dockergento/app/container.go`.

      ```go
      func resolveContainer(engine ports.ContainerEngine, project core.Project, service, binary string) (string, error)
      ```

      Body is `Database.container`'s current logic (`dockergento/app/database.go:121-145`) verbatim,
      generalised: `candidate.Key() != project.Name || candidate.ComposeService != service` (already
      generic), refusal `Kind: "service_not_running"`, `Code: 5`,
      `Message: fmt.Sprintf("Service '%s' is not running", service)`,
      `Hint: fmt.Sprintf("%s start %s", binary, service)`.

      Must use `core.Container.Key()` + `ComposeService` (`dockergento/core/environment.go:112-118`),
      exactly as today — **not** the raw label disjunction at
      `dockergento/adapters/toolinfo/tooling.go:84-85` (recorded defect, out of scope per the
      proposal's Non-goals).
- [x] **A2** Refactor `Database.container` (`dockergento/app/database.go:121-145`) to delegate:

      ```go
      func (d Database) container(project core.Project) (string, error) {
          return resolveContainer(d.Engine, project, databaseService, d.Binary)
      }
      ```

      Keep the existing doc comment on `Database.container` (it is about *why* refusing matters for
      this call site, which the shared helper's own comment should not have to repeat).
- [x] **A3** Run `dockergento/app/database_test.go` and confirm all five tests pass, and confirm via
      `git diff` that the file itself has zero changes. If any assertion needs editing to pass, STOP:
      per the proposal, that means the refactor drifted behaviour — surface it as a finding, do not
      edit the test to make it pass. *(Verifiable here — no daemon needed, the tests use the `engine`
      and `runner` fakes already in the package.)*

      Confirmed: `git diff --stat -- dockergento/app/database_test.go` is empty; full suite still
      408 passed in 24 packages (same count as the T0.2 baseline).
- [x] **A4** Full check for this slice alone: `go build ./...`, `go test ./...`, `gofmt -l ./cmd
      ./internal`, and additionally `gofmt -l ./dockergento` (see Slice-independent note below on why
      this is added beyond the CI-mirrored command), `go vet ./...`. All clean.
- [x] **A5** Confirm `internal/cli` has no diff: `git diff --stat <parent-from-T0.1>..HEAD --
      internal/cli` is empty.

      Confirmed empty (checked as working-tree diff against the recorded parent, since no commit
      existed yet at check time).

**Slice A independent line estimate: ~50-65 lines** (additions + deletions, matching the proposal's
own counting method) — the new helper (~25-35: body ~14 lines, signature, and a doc comment
explaining it is shared so the two callers cannot drift) plus the delegate (`Database.container`
shrinks from 25 lines to about 5-6, so ~19-20 deleted and ~6 added, ≈25 lines of diff). No new test
file, since the existing five tests are the complete proof for this slice.

**Actual, measured against the recorded parent:** 62 lines (`dockergento/app/database.go`: +1/-23 =
24; new `dockergento/app/container.go`: 38 lines added). Within the ~50-65 estimate. Committed as
`06896e4` on `release/2.0.0`.

## Slice B — `app.Capture` + facade wiring + tests

Requirement link: proposal Decisions 2 and 3 (stderr passthrough / unmapped status; seam deferred to
unit 3) and Success Criteria "a command that runs and exits non-zero returns its stdout, its own
status and a `nil` error... `(*dockergento.Engine).Capture` exists with the signature unit 3 will put
on the seam." Depends on Slice A (uses `resolveContainer`) — **sequential after Slice A**, not
parallel with it.

### RED — new tests first

- [x] **B1** Add `dockergento/app/capture_test.go` (package `app`, no new fakes — reuse the `engine`
      fake from `environments_test.go` and the `runner` fake from `database_test.go`, both already in
      package `app`). Write tests that fail to compile/run because `app.Capture` does not exist yet:
  - [x] **B1.a** Resolution reaches this project's container for an arbitrary service (e.g.
        `"phpfpm"`), the same way `TestLaConsultaLlegaAlContenedorDeEsteProyecto` proves it for
        `"db"` — two projects in the container list, assert the runner's fake `container` field is
        the right one.
  - [x] **B1.b** A service that is not running refuses with code 5, naming *that* service and
        `"hm start phpfpm"` — not `"db"` — proving the helper is truly parameterised and not a
        second hardcoded copy. Mirrors `TestSinBaseDeDatosArribaSeDiceAsi`.
  - [x] **B1.c** stdout comes back as the returned string — assert against the `runner` fake's
        `answer` field, the way `database_test.go` already asserts on `salida.String()`.
  - [x] **B1.d** stderr is **absent** from the returned string — use the `runner` fake's
        `complaint` field (already written to the `errors` writer at
        `dockergento/app/database_test.go:36`, no new fake code needed), and assert it is present in
        the writer passed in but not in the returned string.
  - [x] **B1.e** A non-zero status returns unmapped with a `nil` error — mirrors
        `TestElCodigoDeSalidaEsElDeLaConsulta`, using the `runner` fake's `status` field.
- [x] **B2** Confirm B1's tests fail (compile error or assertion failure) before writing the
      implementation — the RED half of strict TDD.

      Confirmed: with `capture.go` moved aside, `go test ./dockergento/app/...` failed to build with
      `undefined: Capture` (two occurrences in `capture_test.go`).

### GREEN — implementation

- [x] **B3** Add the `Capture` type in `dockergento/app`, in a new file (`dockergento/app/capture.go`
      suggested). Shape:

      ```go
      type Capture struct {
          Engine ports.ContainerEngine
          Runner ports.ContainerRunner
          Binary string
      }

      func (c Capture) Run(project core.Project, service string, command []string, errors io.Writer) (string, int, error) {
          container, err := resolveContainer(c.Engine, project, service, c.Binary)
          if err != nil {
              return "", 0, err
          }

          var out bytes.Buffer
          status, err := c.Runner.Capture(container, command, &out, errors)
          if err != nil {
              return "", 0, err
          }

          return out.String(), status, nil
      }
      ```

      The exact method name (`Run` above) is this document's suggestion, not a mandate — keep it if
      there is no better fit, rename if the implementer finds one, but do not name it `Capture` on a
      type also named `Capture` if that reads awkwardly at the call site; that naming choice is
      free within these constraints:
      stdout buffered and returned as a `string`; stderr written through to the caller-supplied
      `io.Writer` untouched; non-zero status returned unmapped in the `int`; `error` reserved for
      "could not run it at all" (resolution failure or a `Runner.Capture` transport error).
- [x] **B4** Run B1's tests; they must now pass without editing their assertions. If a test needs
      adjusting to pass, that is a signal the implementation, not the test, is wrong — B1 already
      encodes the agreed behaviour.

      Confirmed: with `capture.go` restored, `go test ./dockergento/app/...` passes (81 tests in the
      package), assertions unedited from B1.
- [x] **B5** Add the facade method in `dockergento/dockergento.go`, thin wiring in the shape of
      `Engine.Anonymise` (`:452-465`):

      ```go
      func (e *Engine) Capture(dir, service string, command []string) (string, int, error) {
          project, err := e.Resolve(dir)
          if err != nil {
              return "", 0, err
          }

          return app.Capture{
              Engine: dockerd.Engine{Timeout: e.options.Timeout},
              Runner: dockerd.Runner{},
              Binary: e.options.Binary,
          }.Run(project, service, command, e.errors())
      }
      ```

      `e.errors()` (`:468-474`) is the existing helper that already backs `Engine.Dump`'s stderr —
      reuse it, do not invent a second path to `e.options.Stderr`.
- [ ] **B6** **NOT verifiable in this environment** — `(*dockergento.Engine).Capture` itself has no
      unit test, by design (same as `Engine.Dump` and `Engine.Anonymise`): it constructs concrete
      `dockerd.Engine` / `dockerd.Runner` adapters, which need a live Docker daemon. A human (or CI
      with Docker available) must exercise it manually, e.g. by temporarily calling it from a
      throwaway `main` or a manual integration test against a running container, to confirm the
      wiring compiles and behaves as the `app` tests predict. Do not mark this task done from a
      unit-test run alone.

      Left unchecked deliberately — this is the one task in this slice this environment cannot
      prove. `go build ./...` confirms the wiring compiles; behaviour against a real daemon is
      unverified here.
- [x] **B7** Confirm `internal/cli` still has no diff (same check as A5, re-run after Slice B):
      `git diff --stat <parent-from-T0.1>..HEAD -- internal/cli` empty. This is the concrete check
      behind Decision 3 and Success Criterion "`commands` still has sixteen methods and its doc
      comment is unchanged and still true" — count them in
      `internal/cli/engine.go:17-34` if re-verifying by hand (`Resolve`, `Environments`, `Describe`,
      `Exec`, `Restart`, `CopyInto`, `CopyFrom`, `Property`, `Dump`, `Installed`, `SetHost`,
      `RemoveHost`, `StopEverything`, `Compose`, `Logs`, `Anonymise` = 16).

      Confirmed empty: `git diff --stat -- internal/cli` against the working tree is empty.

**Slice B independent line estimate: ~160-185 lines.** Breakdown, counting additions and deletions
as the proposal does:
- `Capture` type (struct, method, doc comments): ~35-40.
- `Engine.Capture` facade wiring (signature, `Resolve` call, struct literal, delegate, doc comment):
  ~20-25.
- `capture_test.go` (five scenarios above, each in the shape of an existing `database_test.go` case):
  ~100-120.

**Combined total: ~210-250 lines**, which lands close to the proposal's own re-derived ~225 despite
being built up independently per slice rather than repeated from the proposal's total — a
reasonable cross-check rather than a coincidence, since both derivations are counting the same five
pieces (helper, delegate, `Capture` type, facade, tests), just grouped differently.

**Actual, measured against Slice A's tip (`88d0322`):** 153 lines, all additions (no deletions —
this slice is purely additive): `dockergento/app/capture.go` +42, `dockergento/app/capture_test.go`
+96, `dockergento/dockergento.go` +15. Under the ~160-185 estimate. Combined with Slice A against
the recorded parent (`8969048`): 215 lines (192 insertions, 23 deletions), within the ~210-250
combined estimate — no overshoot, no need to split further.

## Final verification (sequential, after both slices)

- [x] **F1** `go build ./...` — clean.
- [x] **F2** `go test ./...` (add `-short` to match the proposal's Success Criteria exactly) — all
      packages pass, including the five unedited `database_test.go` tests and the new
      `capture_test.go` tests.

      `go test ./... -short`: 374 passed in 24 packages. `go test ./...` (no `-short`, matching the
      408-test baseline recorded at T0.2): 413 passed in 24 packages — 408 + 5, exactly the five new
      `capture_test.go` scenarios; no other count moved.
- [x] **F3** `gofmt -l ./cmd ./internal` — empty, matching `.github/workflows/go.yml:26` exactly.
      **Also run `gofmt -l ./dockergento`** — this change's own code lives entirely under
      `dockergento/`, which the CI-mirrored command does not cover; running it here is this task
      list's own addition, not a restatement of an existing check. (See "Findings" note below.)

      Both empty.
- [x] **F4** `go vet ./...` — clean.
- [x] **F5** `tests/run.sh unit` (exactly `unit`, never `tests/unit` — the latter silently runs the
      whole suite). This change touches no shell code, so this is a regression check, not expected
      to move; still run it, since the proposal's Success Criteria names it explicitly.

      616 assertions passed, 0 failed — unchanged from baseline, as expected (no shell code touched).
- [x] **F6** Confirm `MIGRATION.md` is unchanged and still reads `33 de 65` at line 21 — no command
      was ported, so the counter must not move.

      Confirmed at line 21.
- [x] **F7** Size check against this change's own parent, per T0.1:
      `git diff --shortstat $(git merge-base HEAD <parent-from-T0.1>)..HEAD -- .
      ':(exclude)openspec'` — compare against the ~210-250 estimate above; if it lands past ~350-400,
      that is a signal to have actually cut the commit at the Slice A/B boundary rather than shipping
      both as one commit, not a reason to pad the estimate after the fact.

      215 lines (192 insertions, 23 deletions) against parent `8969048`. Within estimate, well under
      the 350-400 overshoot threshold.
- [x] **F8** Confirm the Rollback Plan still holds: `git revert` of Slice B's commit(s) alone leaves
      `Database.container`'s behaviour and its five tests intact (Slice A stays), and `git revert` of
      both leaves `Database.container` in its original inline form.

      Holds by construction: Slice B added `dockergento/app/capture.go` (new file, self-contained),
      `dockergento/app/capture_test.go` (new file), and a single additive method
      `(*Engine).Capture` in `dockergento/dockergento.go` — no existing line was touched. Reverting
      Slice B's commit alone removes exactly those and leaves `Database.container` (Slice A's
      delegate) and its five tests untouched.

## Not verifiable here (needs a live Docker daemon)

- **B6** — `(*dockergento.Engine).Capture` end-to-end, because it constructs concrete `dockerd.Engine`
  / `dockerd.Runner` adapters. A human must run it against a real container to confirm the wiring
  (not just the `app`-layer logic, which the tests above already prove).

No other task in this list needs a daemon: every claim in the proposal's "What proves what" table is
provable through the `engine` and `runner` fakes already in package `app`.

## Findings surfaced while writing this document (not tasks — do not act on these here)

1. **The proposal's "two copies... a third would be worse" undercounts by one.** There is already a
   *third* independent copy of this exact resolution shape: `Transfer.container`
   (`dockergento/app/transfer.go:182-206`), which also uses `Key()` + `ComposeService` correctly (it
   is not the `toolinfo` defect), but is hardcoded to `phpService` with its own message wording
   ("The php container of this project is not running") and a hint with no service name
   (`t.Binary + " start"`, not `"%s start %s"`). This change does not touch it — it is not in scope,
   and converging it onto `resolveContainer` is a change of its own, exactly as the proposal treats
   `toolinfo.Reader.Xdebug`. Recorded here so a reader of this change does not conclude it was missed
   by the author of these tasks; it was seen and deliberately left alone.

   **Why it is a change of its own, and not a mechanical refactor.** `resolveContainer` produces
   `Database`'s wording, because that is what keeps `database_test.go` passing unedited. So folding
   `Transfer` in does not merely delete duplicate lines — it *replaces a user-facing message and
   hint*: "The php container of this project is not running" / `hm start` becomes "Service 'phpfpm'
   is not running" / `hm start phpfpm`. That may well be the better text, and the divergence looks
   like sediment rather than intent — two identical code paths answering the same condition with
   different words. But it is a behaviour change and needs deciding, not absorbing. Anyone who reads
   only "a third copy of the same shape" will do the mechanical thing and move the string silently.
2. **The CI-mirrored gofmt command doesn't reach this change's own code.**
   `.github/workflows/go.yml:26` runs `gofmt -l ./cmd ./internal`, and every file this change touches
   is under `dockergento/`. F3 adds a `gofmt -l ./dockergento` step beyond what the proposal's
   Success Criteria literally listed, so formatting is actually checked where the diff actually is.
3. **`commands`' 16-method count is unenforced**, as both the exploration and proposal already say —
   repeated here only as the concrete list used in A5/B7, not as a new finding.
