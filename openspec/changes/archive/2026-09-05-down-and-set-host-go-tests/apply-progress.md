# Apply progress: down-and-set-host-go-tests

## Status: BLOCKED on the 400-line review budget — implementation complete, all green, no commit made

All test-writing and GREEN-routing work for both commits is done and verified. What stopped short
of a commit is the size gate (task 2.4): the measured diff is **609 changed lines** against a 400
budget, and per the design's own "nothing remains to cut" instruction, no code was shrunk to force
a fit. This is reported to the orchestrator for a delivery-strategy decision, exactly as
tasks.md 2.4 and design.md's Size and Commits section both prescribe for this case.

## Step 0 (refactor, pre-RED) — green confirmation

`answering(t)` in `internal/cli/fake_engine_test.go` gained all six pins (HM_STATE_DIR existing;
HM_HOSTS_FILE, HM_NON_INTERACTIVE, DOCKER_HOST, HM_LEGACY_ROOT, `t.Chdir(t.TempDir())` new — in
that order, per the exact instruction given for this apply, which differs from design.md's
listed order (`t.Chdir` first there) but is behaviourally identical since all six are independent
statements executed before the fake is installed). A doc comment above `answering` cites the
production line for each: `app/hosts.go:186` (system hosts fallback), `:173` (`sudo cp` wired to
the real terminal), `select.go:39` (`choose()` refusal), an unrouted toolinfo dial with no
deadline, `app/hosts.go:49` (properties.json under the resolved root = cwd), and
`legacy/runner.go:94` (`bin/run` exec when `ShellRoot` is empty).

The ten `cwd := os.Getwd()` blocks were moved to `cwd := here()` **after** `answering(t)`, inside
each subtest for every table-driven test (`t.Chdir` moves the directory per-subtest, since
`answering` is called once per subtest, each with its own `t.TempDir()`). Two of the ten needed
more than a straight line move because their `want []call` (or `want []call` per case) literals
referenced `cwd` **before** any subtest ran: `TestACopyTheEngineRefusesIsReported`
(`wrappers_test.go`) and `TestWhatTheTestSuitesAsk` (`inside_test.go`) both had their `want` field
turned into `func(cwd string) []call`, called with the per-subtest `cwd` once known. This was not
anticipated by design.md, which assumed a uniform four-line-removed/one-line-added swap for all
ten sites, and is the largest single cause of the size-gate overage below.

Verify: `gofmt -l` on the three edited test files — clean. `go build ./...` — success.
`go test ./internal/cli -short -v` — **44/44 passed**, matching the pre-existing baseline exactly,
before any new test file existed. Confirmed with `rtk proxy go test ./internal/cli -short -v`
(raw, unfiltered output) as well as the summarized form.

## Commit 1 — `version` (RED → GREEN)

**RED** (`go test ./internal/cli -short -run 'TestWhatVersionReports|TestAnArgumentNobodyDeclaredIsAUsageError' -v`):

```
=== RUN   TestWhatVersionReports/--json
    version_test.go:73: version --json data (-want +got):
      - 	"branch": "release/2.0.0"   + "branch": ""
      - 	"commit": "abc1234"         + "commit": ""
      - 	"commits_ahead": 3          + "commits_ahead": 0
      - 	"dirty": true               + "dirty": false
      - 	docker.compose: "2.20.0"    + docker.compose: "2.34.0"   (the real, installed compose)
      - 	docker.version: "24.0.5"    + docker.version: ""
      - 	"path": "/opt/hm"           + "path": "<test binary's own build tmp dir>"
      - 	"tag": "v1.4.5"             + "tag": ""
      - 	"version": "1.4.5"          + "version": "unknown"
--- FAIL: TestWhatVersionReports (0.43s)
```

Confirms the design's decision 5: the real `Installed()` reached this checkout and the installed
Docker Compose (0.43s total, no hang — `DOCKER_HOST=unix:///nonexistent` failed the dial fast
rather than blocking it). The call log had no `Installed` entry (fake never asked), which is what
`cmp.Diff` against the fake's answers exposed.

**GREEN**: `commands` gained `Installed() (core.Installation, core.Tooling)`; `version.go:23`
routed from `engine(` to `newEngine(`. Re-run: **all of `TestWhatVersionReports` (2 subtests) and
`TestAnArgumentNobodyDeclaredIsAUsageError` (2 subtests) pass**, plus every pre-existing test in
the package (52 total after this commit's content).

## Commit 2 — `set-host` (RED → GREEN)

**RED** (`go test ./internal/cli -short -run 'TestWhatSetHostAsks|TestRemovingAHostAsksNothingAboutTheProject|TestARefusedHostEditIsReported|TestASetHostOptionNobodyDeclaredIsAUsageError' -v`):

```
=== RUN   TestWhatSetHostAsks/the_default_asks_for_the_database_write
    set_host_test.go:37: set-host [shop.test] = 3, want exitOK
=== RUN   TestRemovingAHostAsksNothingAboutTheProject/a_domain
    set_host_test.go:89: set-host --remove shop.test = 3, want exitOK
=== RUN   TestRemovingAHostAsksNothingAboutTheProject/--remove_with_no_domain_forwards_an_empty_one
    set_host_test.go:133: set-host --remove = 2, want exitOK (the fake never refuses)
=== RUN   TestARefusedHostEditIsReported
    set_host_test.go:162: set-host refused = 3, want 2 (the refusal's own code, not exitDocker)
--- PASS: TestASetHostOptionNobodyDeclaredIsAUsageError
```

Every case that reaches the real `wrappers.go:281`/`:296` failed at `exitDocker` (3) — the real
`Hosts.Remove`/`Set` hit `os.ReadFile` on the missing pinned temp hosts file — except the
"no domain given to `--remove`" case, which failed at 2: the real `Hosts.Remove("")` refuses
**before** the file read (`app/hosts.go:110-118`), which is a legitimate RED result for the same
unrouted reason, just a different failure point in the real code. `TestASetHostOptionNobodyDeclaredIsAUsageError`
already passed at RED, correctly: `-x` never reaches the engine either way.

**GREEN**: `commands` gained `SetHost(dir, domain string, database bool) error` and
`RemoveHost(domain string) error`; `wrappers.go:281` (RemoveHost) and `:296` (SetHost) routed from
`engine(` to `newEngine(`. Re-run: **all four new test functions pass** (one with 3 subtests, one
with 2, one with 3, one plain), plus every prior test (60 total after this commit's content).

## Regression (after both commits' content)

| Command | Result |
|---|---|
| `go test ./internal/cli -short` | 60 tests, all green (44 baseline + 8 new: 2+2 from `version`, 3+1 from `set-host`, counting subtests as one `t.Run` group each) |
| `go test ./... -short -count=1` | 22 packages, all green (`dockergento`, its 8 adapters with tests, `app`, `core`, `internal/cli`, `test/e2e`; 6 packages report "no test files" as before) |
| `tests/run.sh unit` | 612 assertions passed, unchanged |
| `gofmt -l ./cmd ./internal` | empty |
| `go vet ./...` | clean |
| `var _ commands = (*dockergento.Engine)(nil)` | compiles (confirmed via the successful build and vet above) |

## Size gate — measured, and why it is over

```
$ git diff --shortstat 7417863 -- . ':(exclude)openspec'
 8 files changed, 499 insertions(+), 110 deletions(-)
```

**609 changed lines, 209 over the 400 budget.** Per-file:

| File | + | - | Total | Commit |
|---|---|---|---|---|
| `internal/cli/engine.go` | 3 | 0 | 3 | split (see below) |
| `internal/cli/version.go` | 1 | 1 | 2 | 1 |
| `internal/cli/fake_engine_test.go` | 58 | 3 | 60 | split (see below) |
| `internal/cli/wrappers_test.go` | 21 | 44 | 65 | 1 (cwd-only) |
| `internal/cli/inside_test.go` | 60 | 60 | 120 | 1 (cwd-only) |
| `internal/cli/version_test.go` | 152 | 0 | 152 | 1 |
| `internal/cli/wrappers.go` | 2 | 2 | 4 | 2 |
| `internal/cli/set_host_test.go` | 202 | 0 | 202 | 2 |
| **Total** | **499** | **110** | **609** | |

**Approximate per-commit split** (engine.go and fake_engine_test.go are interleaved single hunks;
these are hand-counted from the diff content, not from an actual split commit, so treat as
approximate — see "Two-commit file/hunk lists" below for the exact line-level ownership within
each shared file):

| Commit | Files | Approx. changed lines |
|---|---|---|
| 1 (`version`, pins, cwd refactor) | `engine.go` (+1), `version.go` (2), `fake_engine_test.go` (~42), `wrappers_test.go` (65), `inside_test.go` (120), `version_test.go` (152) | **≈382** |
| 2 (`set-host`) | `engine.go` (+2), `wrappers.go` (4), `fake_engine_test.go` (~16), `set_host_test.go` (202) | **≈224** |

≈382 + ≈224 = ≈606, within hand-counting rounding of the exact measured 609.

**Why the estimate (≈315) missed by 209, not the ≈85 the design itself flagged as a risk**:

1. The two closure-conversion sites (`TestACopyTheEngineRefusesIsReported`,
   `TestWhatTheTestSuitesAsk`) were not in design.md's estimate at all — it assumed a uniform
   four-line-removed/one-line-added swap for all ten `cwd` sites, but two of them build `want`
   literals as case-table fields **before** any subtest runs, which is before `cwd` is knowable
   under the new `t.Chdir`-per-subtest scheme. Wrapping those literals in `func(cwd string) []call`
   adds real lines (open/close of the closure, plus re-indentation of every literal inside it)
   that the estimate's "56 changed lines total in commit 1" did not include. This alone accounts
   for most of `wrappers_test.go`'s 65 and a large share of `inside_test.go`'s 120 (its table has
   six cases, each now wrapped).
2. `version_test.go` (152 vs ≈85 estimated) and `set_host_test.go` (202 vs ≈120 estimated) both
   came in at roughly double the estimate. Both follow the design's decision-4 case table
   literally — full JSON-envelope decoding plus a `cmp.Diff`'d map or struct per case, plus a
   full call-log assertion per case — and that discipline, multiplied across 2 (version) and 4
   (set-host) test functions with 2-3 subtests each, is heavier than a single-page estimate
   captured.
3. The `answering` doc comment (19 lines, one bullet per pin with its own file:line citation) is
   longer than design.md's own terse per-pin table implied when priced into "the changed lines
   in the tests that already exist" — but it is exactly what tasks.md 1.1 asked for ("a doc
   comment beside each of the six pins stating its reason").

Nothing was shrunk to force a fit: no comment, blank line, doc-comment citation, or test case was
cut. Per the design's own words, "the estimate was wrong ... and that is worth knowing before the
next change repeats it" — here it was wrong by considerably more than the ≈85 lines the design
flagged as its own worst case, primarily because of finding #1 above, which the design's own
"Consequence for the tests that already exist" section did not anticipate.

**This is a blocker requiring an orchestrator decision.** The context for this apply states
`delivery_strategy: single-pr ... no size exception`, which this diff does not fit under as one
PR. The two commits are already independent, self-consistent work units (each compiles, each
passes `go test ./internal/cli -short` and `go test ./... -short` on its own, per design.md's
"Both are self-consistent" claim) — their approximate individual sizes above (≈382 and ≈224)
suggest that shipping them as two chained PRs, rather than one, would likely bring each under the
400 budget without cutting anything. That is one option; accepting a `size:exception` for a
single PR is another. Both are the orchestrator's call, not this phase's.

## Two-commit file/hunk lists (for whichever split is chosen)

Both `internal/cli/engine.go` and `internal/cli/fake_engine_test.go` carry adjacent hunks for both
commits. Exact line-level ownership within each:

### `internal/cli/engine.go` (single 3-line hunk at the `commands` interface, all additions)

```go
 	Property(project core.Project, key string) string
 	Dump(dir, path string) error
+	Installed() (core.Installation, core.Tooling)   // Commit 1
+	SetHost(dir, domain string, database bool) error // Commit 2
+	RemoveHost(domain string) error                  // Commit 2
 }
```

### `internal/cli/version.go` — Commit 1 only, one line

```go
-	installed, tooling := engine(stdout, stderr, jsonOutput).Installed()
+	installed, tooling := newEngine(stdout, stderr, jsonOutput).Installed()
```

### `internal/cli/wrappers.go` — Commit 2 only, two separate hunks

```go
 	if remove {
-		if err := engine(stdout, stderr, jsonOutput).RemoveHost(domain); err != nil {
+		if err := newEngine(stdout, stderr, jsonOutput).RemoveHost(domain); err != nil {
...
-	if err := engine(stdout, stderr, jsonOutput).SetHost(here(), domain, database); err != nil {
+	if err := newEngine(stdout, stderr, jsonOutput).SetHost(here(), domain, database); err != nil {
```

### `internal/cli/fake_engine_test.go` — interleaved; ownership by symbol

| Addition | Commit |
|---|---|
| `import "path/filepath"` | 1 |
| `call.Domain`, `call.Database` fields | 2 |
| `fakeEngine.installed`, `fakeEngine.tooling` fields + their doc comment | 1 |
| `func (f *fakeEngine) Installed()` | 1 |
| `func (f *fakeEngine) SetHost(...)` | 2 |
| `func (f *fakeEngine) RemoveHost(...)` | 2 |
| `answering`'s replaced doc comment (six-pin rationale) | 1 |
| `answering`'s five new `t.Setenv`/`t.Chdir` lines | 1 |

A byte-exact `git apply --cached --recount` patch per commit was not produced in this pass: given
the size gate is over budget, further commit-preparation work was deferred to the orchestrator's
decision rather than invested ahead of it. The table above and the verbatim diff (available via
`git diff 7417863 -- internal/cli/fake_engine_test.go internal/cli/engine.go`) are enough to build
either an exact split or a single combined commit once the delivery strategy is decided.

### `internal/cli/wrappers_test.go`, `internal/cli/inside_test.go`, `internal/cli/version_test.go` — Commit 1 only, whole files as diffed

### `internal/cli/set_host_test.go` — Commit 2 only, whole new file

## Commit messages (as designed, unused pending the decision above)

1. `test(cli): what version asks of the engine, and where RED is allowed to reach`
2. `test(cli): what set-host asks of the engine`

## Tasks completed / pending

- [x] 1.1, 1.2, 1.3, 1.4, 1.5
- [ ] 1.6 (commit — orchestrator-executed, intentionally left unticked)
- [x] 2.1, 2.2, 2.3, 2.4 (measured and reported, over budget)
- [ ] 2.5 (commit — orchestrator-executed, intentionally left unticked)
- [ ] 3.1, 3.2 (manual verification — orchestrator-executed, intentionally left unticked)

## Deviations disclosed

1. Pin order in `answering(t)` follows this apply's explicit instruction order (HM_STATE_DIR,
   HM_HOSTS_FILE, HM_NON_INTERACTIVE, DOCKER_HOST, HM_LEGACY_ROOT, then `t.Chdir`), not
   design.md's listed order (`t.Chdir` first). Both are behaviourally identical: all six are
   independent statements that run unconditionally before the fake is installed.
2. Two cwd-refactor sites needed a `func(cwd string) []call` closure rather than the plain
   `cwd := here()` line move design.md assumed for all ten sites — see the size-gate analysis
   above. No test case, assertion, or comment was cut to compensate.
3. Size gate exceeded (609 vs 400 budget) — reported per tasks.md 2.4's own instruction, not
   worked around. See above for the full analysis and the two-commit approximate split.

## Risks

- The 209-line overage blocks a single-PR delivery under the stated `no size exception` policy;
  the orchestrator must choose between accepting a `size:exception`, splitting into two chained
  PRs along the existing commit boundaries (which already exist as clean, independent units), or
  another remediation. No code was written or held back to hide this — everything above is
  green and ready either way.

## Delivery and manual verification record (orchestrator, 2026-09-05)

Commits on release/2.0.0: `415073b` `test(cli): what version asks of the engine, and where RED is allowed to reach` (6 files, 275+/108-; staged with `git apply --cached --recount` from a partial patch of engine.go and fake_engine_test.go plus the version and refactored test files; in isolation `go build ./...` ok, `go vet` clean, `go test ./internal/cli -short` 50 passed) and `991dfb1` `test(cli): what set-host asks of the engine` (4 files, 224+/2-). Total `git diff --shortstat 7417863..991dfb1 -- . ':(exclude)openspec'`: 8 files, 499+/110- = 609, delivered under a user-accepted size:exception; the runtime ledger objective was reset by maintainer decision.

Manual verification (tasks 3.1 and 3.2), both binaries: `bin/hm` (991dfb1) versus `hm-pre` built from 7417863 into the same `bin/` directory.

| Check | Result |
|---|---|
| `hm --json version` on the running `rabatrepo` | exit 0 on both; documents identical after deleting `data.binary` (which names each build). |
| `hm version` (text) | identical after dropping the binary line. |
| `hm --json set-host --remove nothing.test` with `HM_HOSTS_FILE` pointing at a temporary file | exit 0 on both, identical `{"removed": "nothing.test"}`, identical resulting file. The temporary file's test line did not carry the tool's own marker, so there was nothing to remove: parity of the command, not an observed deletion. |
| `hm --json set-host nothing.test --no-database` from a temporary directory that is not a project | identical refusal, exit 4, no `config/` written. The default path was not run against a real project because `Hosts.Set` writes `DOMAIN` into the project's `config/docker/properties.json` (app/hosts.go:49). |
| Real hosts file | `/etc/hosts` mtime unchanged (Jul 13). Every `set-host` invocation carried `HM_HOSTS_FILE`. |
