# Tasks: What `list`, `describe` and `logs` ask of the engine

Authority order for this file: `design.md` overrules `proposal.md` and `exploration.md` wherever they
disagree — it corrected a safety claim both of them made (see design's "Where the proposal and
exploration turned out to be wrong"). All line numbers below were re-verified against the working
tree at the current `HEAD` (`382247c`) on 2026-09-10; no drift was found between the design and the
code for any citation checked (`list.go`, `describe.go`, `doctor.go`, `engine.go`, `orchestrate.go`,
`fake_engine_test.go`, `dockergento.go`, `app/describe.go`, `app/environments.go`,
`toolinfo/tooling.go`, `gitvcs/vcs.go`, `down_test.go`, `version_test.go`, `set_host_test.go`,
`inside_test.go`, `console/commands/describe.sh`, `go_passthrough_test.sh`).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | Unit 0: 104 · Unit 1 (`list`): 217 · Unit 2 (`describe`): 273 · Unit 3 (`logs`): 247 · **Cumulative: 841** (design's counting convention: a one-line replacement counts as 2 changed lines) |
| 400-line budget risk | **High for the cumulative** (841 > 400). Each unit alone is inside the budget. Every recent cycle in this repo overshot its own floor by 1.3–2x; applied here: unit 0 135–208, `list` 282–434, `describe` 355–546, `logs` 321–494 |
| Chained PRs recommended | **Yes** |
| Suggested split | Four self-contained deliveries, strictly in this order: hint fix → `list` → `describe` → `logs`. This repo's recorded convention on `release/2.0.0` is direct sequential commits rather than PRs (see prior cycles); if that convention still holds, treat the four units as four sequential direct commits instead of four PRs — either way, **not** one combined delivery and **not** a `size:exception` |
| Delivery strategy | `ask-on-risk` (cached) — the cumulative overrun is exactly the risk this strategy asks about |
| Chain strategy | pending — surfaced to the user before apply, per the design's own "Order, and what each unit can land on its own" |

**Decision needed before apply: Yes.** The cumulative (841) exceeds the 400-line review budget. The
design's own recommendation is four chained PRs in the fixed order below, each self-contained; this
matches the orchestrator's request to raise it with the user before Phase 1 starts.

### Suggested Work Units

| Unit | Goal | Likely delivery | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 0 | Fix four hardcoded `--help` hints; add `list_test.go`, `describe_test.go`, `doctor_test.go` holding only the unknown-option refusal (no fake, no seam) | Commit/PR 1 | `go test ./internal/cli -short -run 'TestListRefusesAnyArgument\|TestDescribeRefusesAnUnknownOption\|TestDoctorRefusesAnUnknownOption' -v` | none (no Docker, no project) | Revert `list.go`, `describe.go`, `doctor.go` (hint strings), `list_test.go`, `describe_test.go`, `doctor_test.go` — restores four literal strings, nothing else depends on it |
| 1 | `Environments()` on the seam; `list_test.go` gains the engine-log, rendering and error tests; `list.go:23` routed | Commit/PR 2 (depends on Unit 0 for the hint `list_test.go` was written against) | `go test ./internal/cli -short -run 'TestWhatListAsks\|TestListRefusesAnyArgument\|TestWhatListPrints\|TestWhatListReportsAsJSON\|TestListReportsAnUnreachableDaemon' -v` | none — DOCKER_HOST pinned dead by `answering(t)` | Revert `engine.go` (`Environments`), `list.go:23`, `fake_engine_test.go` (`Environments` growth), `list_test.go`'s new subtests — independent of Units 2–3 |
| 2 | `Describe(dir, withSecrets)` on the seam; `describe_test.go` gains the flag, renderings and error tests; `describe.go:25` routed | Commit/PR 3 (depends on Unit 0 for the hint `describe_test.go` was written against; independent of Unit 1's interface addition, but edits the same `fake_engine_test.go` file, so it must land after Unit 1 in the working tree) | `go test ./internal/cli -short -run 'TestWhatDescribeAsks\|TestDescribeRefusesAnUnknownOption\|TestWhatDescribeReportsAsJSON\|TestWhatDescribePrintsAsText\|TestDescribeReportsAnUnresolvableProject' -v` | none — same pins | Revert `engine.go` (`Describe`), `describe.go:25`, `fake_engine_test.go` (`Describe` growth), `describe_test.go`'s new subtests |
| 3 | `Logs(dir, services, options)` on the seam; new `logs_test.go`; `orchestrate.go:166-167` routed | Commit/PR 4 | `go test ./internal/cli -short -run 'TestWhatLogsAsks\|TestLogsRefusesAFlagWithNoValue\|TestLogsRefusesANonNumericIndex\|TestLogsRefusesAnUnknownOption\|TestLogsReportsAFailure' -v` | none — `t.Chdir` is the load-bearing pin here, not `DOCKER_HOST` | Revert `engine.go` (`Logs`), `orchestrate.go:166-167`, `fake_engine_test.go` (`Logs` growth), `logs_test.go` (new file) |

**Sequencing constraint, stated once rather than four times:** all four units edit
`internal/cli/fake_engine_test.go`, so they are **strictly sequential commits**, never parallel
branches, regardless of PR-vs-direct-commit delivery. `list_test.go` and `describe_test.go` are each
touched twice — created in Unit 0 holding only the refusal test, modified in Unit 1/2 respectively to
add the engine-log, rendering and error tests. `doctor_test.go` and `logs_test.go` are each touched
once (Unit 0 and Unit 3 respectively).

## Phase 0: Scaffolding

- [x] 0.1 Before any edit in Unit 0, run `git rev-parse HEAD` and record the hash. This is **this
      change's own parent commit** for the Phase 5 size check — do not hardcode a fixed hash anywhere
      in this file, a commit message, or a follow-up note. Two other artifacts in this repository have
      already gone stale that way today, and the design's own Size-and-Commits section names this
      explicitly. (Known at the time this file was written: `382247c`, but re-capture it fresh —
      other commits may land on `release/2.0.0` before this change is applied.)
      Recorded at apply time: `63fae9bd00f732d4098d1ea6ed0e21d1d4f04861`.

## Phase 1: Unit 0 — the hint fix (`fix(cli): hints that name a real invocation instead of a --help that refuses`)

This is the only unit that changes production behaviour. It builds no engine and reaches no adapter:
every refusal here returns during parsing, the same shape `down_test.go:11-18` established. It must
land first: any later ordering ships `list_test.go`/`describe_test.go` assertions that pin the
circular `--help` hint as a requirement, which then has to be edited a day later — the exact pattern
this unit exists to stop (design, Decision 6).

- [x] 1.1 RED — create `internal/cli/list_test.go` with `TestListRefusesAnyArgument`, a two-row
      table, no fake and no `answering(t)`, only `t.Chdir(t.TempDir())` per row:
      - row 1: `t.Setenv("COMMAND_BIN_NAME", "hm")`, args `["--help"]` → `exitUsage`, hint (read from
        the decoded `--json` fault's `contract.Fault.Hint`, not the text rendering) equal to
        `"hm list"`. **Fails today**: today's hint is the literal `"hm list --help"`.
      - row 2: `t.Setenv("COMMAND_BIN_NAME", "hd")`, args `["--tonteria"]` → hint `"hd list"`.
        **Fails today**: today's hint is a literal that ignores `COMMAND_BIN_NAME` entirely, so a
        renamed binary would still be told to run `hm list --help`, a command that does not exist for
        it.
      Pin `COMMAND_BIN_NAME` explicitly on both rows (the `version_test.go:86` precedent) — `bin/run:4`
      exports `COMMAND_BIN_NAME="hm"`, so an ambient value would otherwise decide the default row by
      accident rather than by assertion, and the `hd` row is what can only pass once `binaryName()`
      (`engine.go:106-112`) is genuinely called.
      Verify RED: `go test ./internal/cli -short -run TestListRefusesAnyArgument -v` → **FAILS** on
      both rows with a hint-equality diff. This is unit 0's one legitimate failure signature — nothing
      else should be alarmed about, because unit 0 never builds an engine.
- [x] 1.2 RED — create `internal/cli/describe_test.go` with `TestDescribeRefusesAnUnknownOption`, same
      shape: `hm`/`--help` → hint `"hm describe [--with-secrets]"`; `hd`/`--tonteria` → hint
      `"hd describe [--with-secrets]"`. **Fails today** on both rows for the same two reasons (closed
      loop; hardcoded binary name).
      Verify RED: `go test ./internal/cli -short -run TestDescribeRefusesAnUnknownOption -v` →
      **FAILS**, hint-equality diff, same signature as 1.1.
- [x] 1.3 RED — create `internal/cli/doctor_test.go` with `TestDoctorRefusesAnUnknownOption`, same
      shape: `hm`/`--help` → hint `"hm doctor [--only=ports]"`; `hd`/`--tonteria` → hint
      `"hd doctor [--only=ports]"`. `ports` is a real check id (`tests/integration/go_passthrough_test.sh:145`
      runs `doctor --only=ports`). File header states plainly why `doctor` appears in a cycle
      otherwise about three other commands: the string is character-for-character the same defect as
      the other three, it is one line, and leaving it would read as a deliberate exception to a
      convention it simply violates (design, Decision 6). This is `doctor`'s only Go test in this
      cycle — it gets no engine-seam test.
      Verify RED: `go test ./internal/cli -short -run TestDoctorRefusesAnUnknownOption -v` → **FAILS**,
      hint-equality diff, same signature.
- [x] 1.4 Run all three RED tests together and confirm the alarm check: `go test ./internal/cli -short -run 'TestListRefusesAnyArgument|TestDescribeRefusesAnUnknownOption|TestDoctorRefusesAnUnknownOption' -v`.
      Every failure must be a hint-equality diff. Any other failure shape (a panic, a wrong exit code,
      a compile error unrelated to the hint) means a test itself is wrong, not that this unit made
      progress — stop and fix the test before proceeding.
- [x] 1.5 GREEN — `internal/cli/list.go:20`: replace the hardcoded `"hm list --help"` hint with
      `binaryName()+" list"`. `list` takes no options at all (`list.go:18` refuses any argument), so
      this is exactly `version.go:20`'s shape (`binaryName()+" version"`).
- [x] 1.6 GREEN — `internal/cli/describe.go:21`: replace `"hm describe --help"` with
      `binaryName()+" describe [--with-secrets]"` — `describe` takes exactly one option
      (`describe.go:16-18`), the `clean [--force]` shape.
      Also `internal/cli/describe.go:29`: replace the hardcoded `"hm setup"` with
      `binaryName()+" setup"`, making it identical to `engine.go:88`'s hint for the same
      `project_not_configured` refusal three files away. **Knowing exception**: this line's hint fires
      only when `Describe` returns an error, which needs the fake — it ships here in unit 0 but is not
      exercised by a passing test until unit 2's `TestDescribeReportsAnUnresolvableProject`. Leaving it
      unproven for one commit is deliberate (design, Decision 6), not an oversight.
- [x] 1.7 GREEN — `internal/cli/doctor.go:24`: replace `"hm doctor --help"` with
      `binaryName()+" doctor [--only=ports]"` — `doctor` takes `--only=<id>` (`doctor.go:17`).
- [x] 1.8 Re-run 1.4's command; confirm all three tests **PASS**.
- [x] 1.9 Regression: `go test ./internal/cli -short` (whole package, green); `go test ./... -short`
      (24 packages, green, no new red beyond this unit's own new tests); `./tests/run.sh unit`
      (exactly `unit`, not `tests/unit` — the latter silently runs everything); `gofmt -l ./cmd ./internal ./dockergento`
      empty; `go vet ./...` clean.
- [x] 1.10 Confirm parity is unaffected: no file under `console/`, `bin/run`, `data/`, or `dockergento/`
      changed. The shell twins still say `--help` (`console/commands/describe.sh:87`, verified
      unmodified); `go_passthrough_test.sh:80-83, 99-102, 189-192` assert only the exit code for these
      three refusals, so the Go/shell hint divergence introduced here is harmless until the deletion
      cycle removes the twins.
- [x] 1.11 Commit: `fix(cli): hints that name a real invocation instead of a --help that refuses`. (landed as `a670e53`; the message written differs from the one sketched here)
      Files: `internal/cli/list.go`, `internal/cli/describe.go`, `internal/cli/doctor.go`,
      `internal/cli/list_test.go` (new), `internal/cli/describe_test.go` (new),
      `internal/cli/doctor_test.go` (new), `openspec/changes/read-only-commands-go-tests/specs/go-entrypoint/spec.md`
      (the hint scenarios only). Message must state plainly that this is the one unit in the change
      that alters production behaviour and why it goes first (see design, "Order, and what each unit
      can land on its own").

## Phase 2: Unit 1 — `list` (`test(cli): what list asks of the engine, and the one command with no RED net`)

- [x] 2.1 **RED-window safety, named rather than assumed.** State and confirm before writing any
      subtest: this machine has the user's `rabatrepo` project running with 9 live containers.
      `Environments()` takes no directory argument at all (`list.go:23` calls it with nothing), so it
      cannot be protected by `t.Chdir` the way a project-scoped call could — it goes straight to
      `app.Inventory{...}.Environments()` (`dockergento.go:140-146`) → `dockerd.Engine.Containers()` →
      a Docker SDK `ContainerList` across the **whole machine**. Its only protection is the
      `DOCKER_HOST=unix:///nonexistent` pin (`fake_engine_test.go:218`), verified load-bearing because
      `dockerd.Endpoint()` gives `DOCKER_HOST` precedence over the context store and `client.FromEnv`
      reads it too. Every subtest that exercises `list` in this unit runs under `answering(t)`, and
      none overrides `DOCKER_HOST`. This is stated as a file-level comment in `list_test.go`, not left
      to be discovered.
- [x] 2.2 RED — extend `commands` in `internal/cli/engine.go` (currently 12 methods, `:18-29`) with
      `Environments() ([]core.Environment, error)`, matching `dockergento.go:140`'s signature exactly.
      Add to `internal/cli/fake_engine_test.go`: the `environments []core.Environment` field (the
      `Resolve`/`project` shape — answer field + `outcomeFor`) and the `Environments()` method,
      returning a **nil** slice (not `[]core.Environment{}`) on error, matching
      `app/environments.go:26-28`'s own real-engine behaviour.
      Modify `internal/cli/list_test.go` (created in Phase 1) to add:
      - `TestWhatListAsks` — happy path with `fake.environments` set to a small fixture; asserts
        `cmp.Diff([]call{{Method: "Environments"}}, fake.calls)` — no `Resolve` call, because
        `list.go` never calls `projectOr`.
      - one more row on `TestListRefusesAnyArgument`: `algo` under `answering(t)` (in addition to
        unit 0's no-fake `--help` row) — asserts `exitUsage` and `len(fake.calls) == 0`, catching a
        future loosening of `list.go:18-21` that would let a positional reach the engine.
      - `TestWhatListPrints` — table over: empty inventory (bare header + `hm setup` suggestion,
        `list.go:55-60`); one ordinary row (column widths `%-26s`/`%-9s`/`%-7s`/`%-22s`); a row that is
        orphan, has no metadata, and has empty `Branch`/`Root` (`orDash`, `:93-99`; the two markers at
        `:73-79`); a row with a `Worktree` (`name (wt: branch)` composition, `:69-71`). Every want is a
        hand-written literal string, never a `fmt.Sprintf` of `list.go`'s own format string.
      - `TestWhatListReportsAsJSON` — two environments; decode `--json` output to `map[string]any`,
        `cmp.Diff` against a hand-written literal (never a re-marshalled fixture).
      - `TestListReportsAnUnreachableDaemon` — `outcomes[0].err` set; both text and `--json`; asserts
        `exitDocker` and the decoded fault's `type == "docker_unavailable"`.
      Verify RED: `go test ./internal/cli -short -run 'TestWhatListAsks|TestListRefusesAnyArgument|TestWhatListPrints|TestWhatListReportsAsJSON|TestListReportsAnUnreachableDaemon' -v`.
      **The one legitimate failure signature**: `list = 3, want exitOK` (i.e. `exitDocker` instead of
      the expected `exitOK`) on every subtest that expects success. `list.go:23` still calls the real
      `engine(`, not `newEngine(`, so `Environments()` reaches the real engine; `answering(t)`'s
      `DOCKER_HOST=unix:///nonexistent` pin makes that real dial fail fast (`ENOENT`) instead of
      hanging or succeeding, which is what produces `exitDocker` rather than a hang or a live result.
      **Any other failure signature is an alarm**: in particular, a call-log diff at `exitOK` —
      `want [{Environments}]`, `got []` succeeding — means `Environments()` actually reached this
      machine's live containers. **Stop the run and investigate before repeating it.** Do not add a
      guard subtest asserting `os.Getenv("DOCKER_HOST") == "unix:///nonexistent"` as a substitute check
      — that assertion cannot fail and gives false confidence (design, Decision 2).
- [x] 2.3 GREEN — route `internal/cli/list.go:23` from `engine(` to `newEngine(`.
      Verify GREEN: re-run 2.2's command → **PASSES**, all subtests.
- [x] 2.4 Regression: `go test ./internal/cli -short -v` (whole package, green); `go test ./... -short`
      (green, 24 packages); `./tests/run.sh unit`; `gofmt -l ./cmd ./internal ./dockergento` empty;
      `go vet ./...` clean; confirm `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:32`)
      still compiles unedited — the interface grew by exactly the method the routed call site needs,
      and `*dockergento.Engine` already has it.
- [x] 2.5 Commit: `test(cli): what list asks of the engine, and the one command with no RED net`.
      Files: `internal/cli/engine.go`, `internal/cli/list.go`, `internal/cli/fake_engine_test.go`,
      `internal/cli/list_test.go`.

## Phase 3: Unit 2 — `describe` (`test(cli): what describe asks of the engine`)

- [x] 3.1 **RED-window safety, named rather than assumed — this is the one that reaches into a live
      container, not just a container list.** State and confirm before writing any subtest: `Resolve`
      does **not** fail on a directory that is not a project (`dockergento.go:129-131`: "a directory
      that is not a project is not an error … the answer is a project with no name"; outside a
      repository `gitvcs.Git.Resolve` returns `(dir, false, "", nil)`, `gitvcs/vcs.go:32`). So an
      unrouted `describe` proceeds into `app.Describer.Describe`'s seven concurrent questions
      (`app/describe.go:58-70`) **before inspecting any error**, and two of them are
      `d.Engine.Containers()` and `d.Tooling.Xdebug(project.Name)`. With a nameless project,
      `toolinfo/tooling.go:84-85` matches:
      ```go
      matches := item.Labels["hm.project"] == project ||
          (item.Labels["hm.project"] == "" && item.Labels["com.docker.compose.project"] == project)
      ```
      — with `project == ""` the first disjunct is true of every container lacking the `hm.project`
      label, so combined with the `com.docker.compose.service == "phpfpm"` test
      (`toolinfo/tooling.go:87`), `Xdebug` selects the first unlabelled `phpfpm` container **on the
      whole machine** and runs `ContainerExecCreate`/`ContainerExecAttach` **inside it**. This machine
      has the user's `rabatrepo` project running with 9 live containers, at least one `phpfpm` among
      them. **`DOCKER_HOST=unix:///nonexistent` is the only thing between this unit's RED run and
      execing into somebody else's running container.** Every subtest that exercises `describe` in
      this unit runs under `answering(t)`, and none overrides `DOCKER_HOST`. This is stated as a
      file-level comment in `describe_test.go`, not left to be discovered.
- [x] 3.2 RED — extend `commands` in `internal/cli/engine.go` with
      `Describe(dir string, withSecrets bool) (core.Description, error)`, matching
      `dockergento.go:152`'s signature exactly. Add to `internal/cli/fake_engine_test.go`: the
      `description core.Description` field (same answer-field shape) and the `Describe(dir, withSecrets)`
      method; add `WithSecrets bool` to `call` (recorded, not acted on — what the credentials are
      withheld from is `app.Describer`'s decision one layer below this seam).
      Modify `internal/cli/describe_test.go` (created in Phase 1) to add:
      - `TestWhatDescribeAsks` — no arguments → `[{Describe, Dir: cwd, WithSecrets: false}]` (the
        highest-value case in the cycle: `withSecrets` defaulting to `true` would volunteer the
        database password into every log, issue and agent context); `--with-secrets` →
        `WithSecrets: true`, nothing else in the log changes. No `Resolve` call — `describe.go` never
        calls `projectOr`, so `f.project` plays no part in this command at all.
      - one more row on `TestDescribeRefusesAnUnknownOption`: a bare `algo` under `answering(t)` →
        `exitUsage`, empty log.
      - `TestWhatDescribeReportsAsJSON` — a full `core.Description` fixture built field-by-field with
        a small local builder (anonymous nested structs, `core/environment.go:128-131,207-246`, mean
        no single composite literal), `Credentials` nil → decode to `map[string]any`, literal
        comparison, and the `credentials` key must be **absent**, not null (`omitempty` regression
        guard); repeat with `Credentials` set to the fixture sentinels below.
      - `TestWhatDescribePrintsAsText` — `Credentials` nil (also the nil-guard/panic test for
        `describe.go:91`); `Credentials` set (four labels: db name, user, password, root password, in
        order, under the `warning("Credentials")` heading); empty `Version`/`Mode` and empty `URLs`
        entries (`orUnknown`, skipped blank URL lines).
      - `TestDescribeReportsAnUnresolvableProject` — `outcomes[0].err = errors.New("compose file is invalid")`
        → `exitProject`, `project_not_configured`, message contains the engine's own error text
        (`describe.go:27-29` must interpolate `err`, not swallow it); a second row with
        `COMMAND_BIN_NAME=hd` → hint `"hd setup"` — **this is unit 2's half of unit 0's knowing
        exception**: `describe.go:29`'s corrected hint shipped in unit 0 and is only now exercised by
        a passing test, because reaching this branch needs the fake.
      **Fixture, load-bearing on two counts**: `fake.project = core.Project{Name: "shop", Root: "/code/shop"}`
      is **not needed** for the two `Describe`-asking tests above (`describe.go` never calls
      `projectOr`, so `f.project` is irrelevant to it) — this fixture only matters for `logs` in unit
      3. Do not add it here; adding it would suggest `describe` depends on `f.project`, which it does
      not.
      **Credential sentinels**: the four `core.Credentials` values are
      `"db-name-fixture"`, `"db-user-fixture"`, `"db-password-fixture"`, `"db-root-password-fixture"` —
      never the tool's own documented defaults (`magento`/`magento`, `password`), which would read like
      a real pair and would make a grep for them hit a test file.
      Verify RED: `go test ./internal/cli -short -run 'TestWhatDescribeAsks|TestDescribeRefusesAnUnknownOption|TestWhatDescribeReportsAsJSON|TestWhatDescribePrintsAsText|TestDescribeReportsAnUnresolvableProject' -v`.
      **The one legitimate failure signature**: `describe = 4, want exitOK` (i.e. `exitProject` instead
      of the expected `exitOK`) on every subtest that expects success. `describe.go:25` still calls the
      real `engine(`; `Resolve` succeeds against the pinned temp cwd with a nameless project, so
      `Describe` proceeds into the seven-way fan-out, `Compose.Load` fails to find a compose file at
      the temp root, and that `configErr` becomes `exitProject`/`project_not_configured` before any
      other question's answer matters. **Any other failure signature is an alarm** — in particular a
      call-log diff at `exitOK` means the daemon (or worse, a live container's exec) answered. **Stop
      and investigate before repeating the run.**
- [x] 3.3 GREEN — route `internal/cli/describe.go:25` from `engine(` to `newEngine(`.
      Verify GREEN: re-run 3.2's command → **PASSES**, all subtests, including the `TestDescribeReportsAnUnresolvableProject`
      row proving the unit-0 hint fix.
- [x] 3.4 Regression: same battery as 2.4 — `go test ./internal/cli -short -v`; `go test ./... -short`;
      `./tests/run.sh unit`; `gofmt -l ./cmd ./internal ./dockergento` empty; `go vet ./...` clean;
      `engine.go:32`'s assertion still compiles unedited.
      Verified at apply time: `go test ./... -v` (no `-short`) green — 24 packages, 361 `=== RUN`
      entries (347 before this unit, +14 net: `TestDescribeRefusesAnUnknownOption` gained one row,
      four new test functions were added); `./tests/run.sh unit` — 616 assertions, unchanged
      (this unit touches no shell file); `gofmt -l ./cmd ./internal ./dockergento` empty; `go vet ./...`
      clean; `engine.go:32`'s assertion compiles unedited; `commands` now holds fourteen methods
      (`Logs` joins in unit 3).
- [x] 3.5 Budget watch (informational, not a gate): if this unit's measured diff approaches or exceeds
      400 changed lines, cut in this order and no other (design, "The budget, and the one unit at
      risk"): (1) merge the two `--json` subtests into one table row-pair; (2) drop the bare-positional
      row this unit adds to `TestDescribeRefusesAnUnknownOption` — unit 0 already proves that branch
      with two rows; (3) split the two rendering tests into a fifth commit, leaving the log, the flag
      and the error in this one. Never cut: the `WithSecrets: false` case, the credentials-absent case,
      or anything from unit 0. If the total still exceeds 400 after those three cuts, stop and report
      it rather than shrinking anything else.
      Measured at apply time: `git diff --shortstat 2bd6f1d -- . ':(exclude)openspec'` → 4 files
      changed, 487 insertions(+), 4 deletions(-), **491 changed lines** — inside the design's own
      predicted overshoot band for this unit (355–546) and roughly 1.8x its 273-line floor, which
      matches "every recent cycle here overshot its own floor by 1.3–2x". Reported rather than cut,
      per this task's own instruction that the watch is informational and the delivery decision
      belongs to whoever ships it, not to this apply pass.
- [x] 3.6 Commit: `test(cli): what describe asks of the engine`. (landed as `f194df1`; the message written differs from the one sketched here)
      Files: `internal/cli/engine.go`, `internal/cli/describe.go`, `internal/cli/fake_engine_test.go`,
      `internal/cli/describe_test.go`.

## Phase 4: Unit 3 — `logs` (`test(cli): what logs asks of the engine`)

- [x] 4.1 **RED-window safety, named rather than assumed — the one command genuinely safe by
      construction.** State and confirm before writing any subtest: `projectOr` (`engine.go:77-92`) is
      already routed through `newEngine`, so the fake answers `Resolve` regardless of this unit's own
      routing state. The unrouted half is only the real `Logs(cwd, …)` call: `Resolve(cwd)` succeeds
      (nameless project), `orchestrator.open` builds a docker CLI and `Initialize`s it **without
      dialling**, then `load` (`composelib/orchestrator.go:204-215`) fails to find a compose file at
      the temp cwd. No daemon call happens at all on this path — `DOCKER_HOST` is belt-and-braces here,
      and `t.Chdir(t.TempDir())` is what actually bounds it. State this contrast explicitly in
      `logs_test.go`'s header: `list` and `describe` need `DOCKER_HOST`; `logs` does not, and saying so
      is itself the record that this was checked rather than assumed (design, Decision 2).
- [x] 4.2 **Fixture, load-bearing on two counts — write both, and say why in the test file.** Every
      `logs` test that reaches the engine sets
      `fake.project = core.Project{Name: "shop", Root: "/code/shop"}` (`inside_test.go:24`'s value
      verbatim):
      - `Name` must be non-empty, or `projectOr` (`engine.go:85`: `if err != nil || project.Name == ""`)
        refuses before `Logs` is ever called — a fixture with only `Root` set would die at
        `exitProject` with the log holding `Resolve` alone, before the engine method under test runs.
      - `Root` must be a path that can never equal a `t.TempDir()` cwd — this is what makes the `Dir`
        assertion below **able to discriminate**; leaving it empty would let the assertion pass without
        proving anything.
- [x] 4.3 **The `Dir` trap — state which applies where, do not copy `inside_test.go` verbatim.**
      `orchestrate.go:166-167` calls `Logs(here(), …)` — `os.Getwd()` — and **not** `project.Root`;
      `inside()` does the opposite, and `inside_test.go:24,35,65,122` assert `Dir: root` accordingly.
      Copying `inside_test.go`'s pattern into `logs_test.go` would produce a test that passes but
      silently asserts the wrong value. Write the comment verbatim at the `Logs` entry of every `want`
      in this file:
      ```go
      // Dir is the test's working directory, not f.project.Root: orchestrate.go:166 asks about
      // here() (os.Getwd()) and never about the project projectOr just resolved. inside_test.go
      // asserts Dir: root for the opposite reason — inside() passes project.Root — so this is where
      // the two precedents diverge. If this line ever fails, the want is right and the change to
      // orchestrate.go is the defect.
      ```
      The expected `Dir` is read from `os.Getwd()` **inside the test itself**, never hardcoded and
      never copied from the fixture's `Root`.
- [x] 4.4 RED — extend `commands` in `internal/cli/engine.go` with
      `Logs(dir string, services []string, options core.LogOptions) error`, matching
      `dockergento.go:322`'s signature exactly. Add to `internal/cli/fake_engine_test.go`: the
      `Logs(dir, services, options)` method (outcome-only, the `Restart` shape — no answer field
      needed); add `LogOptions core.LogOptions` to `call` (compared whole, so a flag that starts
      writing a different field shows up as a diff rather than nothing). `call.Services` is **reused**
      from `Restart` — no new field for it.
      Create `internal/cli/logs_test.go` with:
      - `TestWhatLogsAsks` — no arguments → log `[{Resolve, Dir: cwd}, {Logs, Dir: cwd, Services: []string{}, LogOptions: core.LogOptions{}}]`
        (`Services` **must** be asserted as `[]string{}`, not left as the zero value — `cmp` distinguishes
        a nil slice from an empty one, and `orchestrate.go:92` initializes it non-nil); one flag-mapping
        case covering `-f`/`--follow`, `-t`/`--timestamps`, `--no-color`, `--no-log-prefix`
        (`:131-138`); two rows with an **identical** want for `--tail 100` and `-n 100` (`:111-112`
        share one field); `--since 1h`/`--until 5m` (`:113-116`); `--index 2`
        (`strconv.Atoi`, `:117-124`); `phpfpm nginx` as positionals → `Services: []string{"phpfpm", "nginx"}`,
        order preserved; everything at once, to prove flag order does not matter.
      - `TestLogsRefusesAFlagWithNoValue` — table over `--tail`, `-n`, `--since`, `--until`, `--index`,
        each as the last argument with nothing after it → `exitUsage`, `missing_value`, empty log
        (`takesValue`, `orchestrate.go:96`).
      - `TestLogsRefusesANonNumericIndex` — `--index nada` → `exitUsage`, `invalid_argument`, empty log
        (`:117-122`).
      - `TestLogsRefusesAnUnknownOption` — `--tonteria` → `exitUsage`, empty log (`:140-143`).
      - `TestLogsReportsAFailure` — `outcomes[1].err` set (the `Logs` call, **not** the preceding
        `Resolve` call) → `exitDocker`, `docker_failed`, call log ends at `Logs` (`Resolve` succeeded
        first).
      File-level comment, required and not optional (design, Decision 4 / proposal Decision 1): a row
      that names a service still reaches the **real** `engine(nil, nil, jsonOutput).Configuration(project)`
      inside `checkServices` (`orchestrate.go:198-203`) — off this seam by design. With
      `f.project.Root` set to `/code/shop`, the compose loader finds nothing there, returns an error,
      and `:201-203` treats **any** error as "skip validation". That is deterministic **only while no
      test in this file creates a compose file under `f.project.Root`** — and since that root does not
      exist on any machine, nothing here can. `unknown_service` is not a planned case in this file for
      that reason; it stays covered by `tests/integration/logs_launch_version_test.sh:148-153` and
      `tests/integration/go_orchestration_test.sh:176-181`, both unmodified.
      Verify RED: `go test ./internal/cli -short -run 'TestWhatLogsAsks|TestLogsRefusesAFlagWithNoValue|TestLogsRefusesANonNumericIndex|TestLogsRefusesAnUnknownOption|TestLogsReportsAFailure' -v`.
      **The one legitimate failure signature**: `logs = 3, want exitOK` (`exitDocker` instead of the
      expected `exitOK`) on every subtest that expects success — `orchestrate.go:166-167` still calls
      the real `engine(`, `Resolve` succeeds, and `load` fails to find a compose file at the pinned
      temp cwd, reported as a Docker failure before `Logs` is ever reached through the fake. **Any
      other failure signature is an alarm** — in particular, a hang would mean `open()` reached a
      socket it should never have reached. **Stop and investigate before repeating the run.**
- [x] 4.5 GREEN — route `internal/cli/orchestrate.go:166-167` from `engine(` to `newEngine(`.
      `orchestrate.go:200` (`Configuration`, inside `checkServices`) is **deliberately left unchanged**
      — it stays off the seam per Decision 4/1.
      Verify GREEN: re-run 4.4's command → **PASSES**, all subtests.
- [x] 4.6 Regression: same battery — `go test ./internal/cli -short -v`; `go test ./... -short`;
      `./tests/run.sh unit`; `gofmt -l ./cmd ./internal ./dockergento` empty; `go vet ./...` clean;
      `engine.go:32`'s assertion still compiles unedited; confirm `commands` now has exactly fifteen
      methods and `Configuration` is not among them.
- [x] 4.7 Confirm `tests/integration/logs_launch_version_test.sh` and
      `tests/integration/go_orchestration_test.sh` are unmodified — they are what still covers
      `unknown_service` in Go.
- [x] 4.8 Commit: `test(cli): what logs asks of the engine`. (landed as `6f0edc5`; the message written differs from the one sketched here)
      Files: `internal/cli/engine.go`, `internal/cli/orchestrate.go`, `internal/cli/fake_engine_test.go`,
      `internal/cli/logs_test.go` (new).

## Phase 5: Size gate and cumulative verification

- [x] 5.1 Measure the cumulative diff against **this change's own parent commit captured in 0.1** (not
      a hash written into this file): `git diff --shortstat <parent-from-0.1>..HEAD -- . ':(exclude)openspec'`.
      Design's floor is 841 changed lines (unit 0: 104, `list`: 217, `describe`: 273, `logs`: 247);
      every recent cycle here overshot its own floor by 1.3–2x. This number is **not** a gate to pass —
      it is what confirms the Review Workload Forecast's "Decision needed before apply: Yes" and the
      four-way split were the right call, and it is reported to whoever makes the delivery decision
      rather than acted on unilaterally.
- [x] 5.2 Cumulative regression, all four units applied: `go test ./... -short` (24-package baseline
      319, plus this cycle's new tests, no red); `./tests/run.sh unit` (616-assertion baseline, plus
      this cycle's additions — exactly `unit`, not `tests/unit`); `gofmt -l ./cmd ./internal ./dockergento`
      empty; `go vet ./...` clean; `var _ commands = (*dockergento.Engine)(nil)` compiles;
      `commands` has exactly fifteen methods.
- [x] 5.3 Confirm the non-goals held end to end: no file under `console/`, `bin/run`, `data/`, or
      `dockergento/` was modified anywhere in this change except the four production hint/routing
      edits already listed per unit; no shell twin or bash suite was deleted; `answering(t)` and its
      doc comment in `fake_engine_test.go` were not edited beyond the three method/field additions
      named per unit.
- [ ] 5.4 NOT DONE — no disposable Magento project was available in this session, and `rabatrepo` is explicitly excluded. Left open deliberately; the automated coverage is thorough, but this is real unfinished work rather than a box nobody ticked. Manual verification, real project, all read-only (never `rabatrepo` — a **disposable**
      project or a lab directory only): `hm list`, `hm list --json`, `hm describe`,
      `hm describe --json`, `hm logs --tail 5`, `hm logs no-existe` (still exit 5 via
      `checkServices`), and the usage errors `hm list algo`, `hm describe --tonteria`,
      `hm logs --tail`.

## Notes

- Out of scope, no tasks: deleting any shell twin (`list.sh`, `describe.sh`, `logs.sh` and their bash
  suites stay); `bash`, `exec`, `masquerade` (their own cycle — declared `"dangerous"`, joined at
  `execute()`, and `masquerade`'s decline-path test needs `os.Stdin` redirection); adding
  `Configuration` to the seam; making `--help` a per-command flag; `app.Describer`'s own unit-test gap
  (`app/describe.go:156-158`, one layer below this seam — no test here can close it).
- `answering(t)` (`fake_engine_test.go:212-229`) is not edited beyond adding the three new fake methods
  and fields named per unit — its six pins and its doc comment stay as they are.
- No documentation task added beyond the `go-entrypoint` spec delta: no command's arguments, `--json`
  document, or exit codes change anywhere in units 1–3; unit 0 changes only hint text, which the spec
  delta already covers.
- Verified during this phase, not carried from the design unchecked: every line citation above was
  re-read against the current working tree before being written into a task. None had drifted from
  `design.md`'s own citations.
