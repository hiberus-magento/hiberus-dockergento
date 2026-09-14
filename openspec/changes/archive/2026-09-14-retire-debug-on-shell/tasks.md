# Tasks: retire-debug-on-shell

Change: `retire-debug-on-shell`
Engram topic: `sdd/retire-debug-on-shell/tasks` (project `hiberus-dockergento`)
Branch: `release/2.0.0`
Spec delta: `openspec/changes/retire-debug-on-shell/specs/go-entrypoint/spec.md` (already written; this
phase sequences its merge, it does not re-derive it)

No design document exists for this change — nothing architectural is open. Every item below traces
to `proposal.md`'s Scope, corrected by its own "Correction to the exploration" section, plus one item
(6b) that corrects a claim in the previous cycle's archive report rather than anything in this
change's own proposal.

Two commits, gated in this order because each half of commit 1 is red until the other lands.

---

## Commit 1 — the deletion and the two assertions it makes true

Nothing in this commit may run against a live Docker daemon; every assertion added here refuses
before reaching one.

- [x] **1.1** Delete `console/commands/debug-on.sh` (38 lines).
  - Spec: `openspec/changes/retire-debug-on-shell/specs/go-entrypoint/spec.md` — "Nothing under the
    shell tree implements it".
- [x] **1.2** Delete `console/tasks/set_xdebug_host_property.sh` (19 lines).
  - Sourced only from `debug-on.sh:11`; disappears as a side effect, no retirement mechanism of its
    own, per the same spec scenario.
- [x] **1.3** `tests/unit/command_safety_test.sh` — add `debug-on` to the `RETIRED` array (currently
    `:69-71`, 15 names). Update the stale count in the comment at `:64` ("over the twelve retired by
    name") to sixteen while those exact lines are open.
  - Must land in the **same commit** as 1.1/1.2: `command_safety_test.sh:76`
    (`[ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"`) reports `debug-on(file)` and
    goes red if `debug-on` is listed while the file still exists.
  - Spec: "The RETIRED list gains its name only once the file is gone".
- [x] **1.4** `tests/integration/cli_contract_test.sh` — add one `debug-on` pair immediately after the
    `version` pair (`:87-94`, 8 lines), following that exact shape:
  - `test_case "and so does debug-on, once its own shell twin is gone"` / `run debug-on` /
    `assert_equals "2" "$STATUS"` / `assert_json_field "$STDERR" '.error.type' "command_moved_to_binary"`
  - `test_case "and the readable hint for debug-on names the binary command too"` /
    `run --no-json debug-on` / `assert_contains "$STDERR" "hm debug-on"`
  - Must land in the same commit: with `debug-on.sh` present, `bin/run debug-on` does not refuse —
    `check_command_dir` (`bin/run:72-81`) only answers `command_moved_to_binary` when the file is
    absent, and with it present the script would execute, reaching
    `start_service_if_not_running.sh` — a live-daemon touch this change forbids.
  - Spec: "Asking the shell entry point for a retired command".
- [x] **1.5** Merge the spec delta: fold
    `openspec/changes/retire-debug-on-shell/specs/go-entrypoint/spec.md`'s `MODIFIED Requirements`
    block into `openspec/specs/go-entrypoint/spec.md`, replacing the "A command proved in Go has no
    shell twin" requirement at `:1011-1063` (fifteen-name enumeration, singular-`describe` scenario
    title) with the sixteen-name version already drafted in the change folder. No other requirement
    in the live spec changes — `:604-615`'s vacuous-after-deletion GIVEN and `:466`/`:570`'s
    provenance mentions are deliberately left untouched (see Deliberate non-actions below).

### Mutation check for commit 1 (perform, do not skip)

- [x] **1.6** Restore `console/commands/debug-on.sh` from the parent commit
    (`git show <parent>:console/commands/debug-on.sh > console/commands/debug-on.sh`) with the
    `RETIRED` entry and both new assertions still in place. Run
    `./tests/run.sh unit` (not `tests/unit` — see Verification below) and confirm
    `command_safety_test.sh` reports `debug-on(file)` and the suite goes red.
- [x] **1.7** Remove the restored file again (`rm console/commands/debug-on.sh`, or `git checkout`
    only after confirming no other uncommitted change would be lost) and re-run
    `./tests/run.sh unit`; confirm it goes green again. This is the same check the previous cycle ran
    for its own three deletions (`command_safety_test.sh` reporting `list(file) describe(file)
    logs(file)` and clearing).

### Verification for commit 1

- [x] **1.8** `go test ./...` — compare against the baseline (461 across 24 packages); expect no
    change, since no Go source is touched.
- [x] **1.9** `go vet ./...` — clean.
- [x] **1.10** `gofmt -l ./internal ./cmd` and `gofmt -l ./dockergento` — both empty, since no Go is
    written.
- [x] **1.11** `./tests/run.sh unit` — argument is exactly `unit`; `tests/unit` resolves to
    `tests/tests/unit` under `run.sh`'s own resolution (`:81` checks `-d "$TESTS_DIR/$target"`), which
    does not exist, silently falls through to `:83`, and runs the entire suite instead of the fast
    slice.
- [x] **1.12** `./tests/run.sh` (no argument, full suite) with `DOCKER_HOST` pinned at an unreachable
    socket, exactly as the move-aside experiment was run — never against the live daemon. Diff the
    output (paths and timings filtered) against the recorded baseline (59 of 1003 assertions failing,
    the same seven suites: `cli_contract`, `concurrent_safety`, `go_engine`, `go_passthrough`,
    `logs_launch_version`, `tui_dashboard`, `worktree`). Any new failure or a changed failure count is
    a regression this change introduced; the experiment already measured that none should appear.
- [x] **1.13** `migration_status_test.sh` still passes with `MIGRATION.md`'s counter unchanged at
    `34 de 65` — this cycle ports no command, and `debug-on` is already tabled `go` at
    `MIGRATION.md:261`, untouched by this cycle.

---

## Commit 2 — the documentation commit 1 makes true

Strictly after commit 1: both edits below assert the shell script is gone, and asserting that before
the commit that makes it true is the exact shape the previous cycle flagged as its own warning.

- [x] **2.1** `architecture/05-state-and-configuration.md:207` (the `XDEBUG_HOST` table row) — correct
    the example value. It currently reads `"host.docker.internal"`, presented as the general default;
    that is the WSL2 answer (`xdebug_host.go:62-64`, `hostFromProcVersion`'s `microsoft-standard`
    branch), not the general one. State that the value is host- and platform-dependent and point at
    §5.8.1 rather than giving one example as if it were universal.
- [x] **2.2** `architecture/05-state-and-configuration.md` §5.8.1 (`:364-389`, not `:364-384` — the
    section runs through the `hm debug-off` half of the activation snippet at `:386-388`, which is
    wrong in the same way as the rest and must be replaced too). Rewrite against
    `dockergento/adapters/machine/xdebug_host.go` (`Host.XdebugHost`, `hostFromProcVersion`,
    `hostFromIPAddr`, `hostFromIfconfig`) and `internal/cli/wrappers.go`'s `debugOn`:
  - Replace the "reads the default gateway" description with: not applicable outside Linux
    (`("", nil)`); on Linux, WSL1 and WSL2 each resolve to a fixed address read from `/proc/version`
    (`microsoft-standard` checked before plain `Microsoft`, deliberately, per `xdebug_host.go:57-60`);
    otherwise the address comes from `docker0`'s own `inet` line via `ip addr show docker0`, falling
    back to `ifconfig docker0` — never a default-gateway lookup, and never true in either
    implementation.
  - Replace the macOS `"host.docker.internal"` default with: macOS returns `("", nil)` — not
    applicable, nothing to patch. (The old shell script left `XDEBUG_HOST` empty for the same reason;
    Go makes the same non-applicability explicit as a typed return.)
  - Replace the `.ini.disabled`-rename description of activation with the actual mechanism: a `sed`
    comment/uncomment toggling the `zend_extension` line in
    `docker-php-ext-xdebug.ini`, run by `debugOn`, matching what `debug-on.sh:19` did (minus its
    stray inert `\;` before `zend_extension`).
  - Add the two WSL branches the current snippet has none of.
  - Note the two behaviors Go has that the shell never did: `ErrNoBridgeAddress` when `docker0`
    exists but no tool on the machine can resolve its address (`xdebug_host.go:11-19`), and the
    deliberate `microsoft-standard`-before-`Microsoft` check order.
  - State plainly that `console/commands/debug-on.sh` and
    `console/tasks/set_xdebug_host_property.sh` no longer exist.
  - Bound: 60 lines for this subsection; if the rewrite grows past that, trim rather than exceed.
- [x] **2.3** `MIGRATION.md` — add the third-step note for `debug-on`, beside the existing ones at
    `:157-188` (the twelve-command note and the `list`/`describe`/`logs` note), following their shape:
    name both deleted files and their line counts, state that `debug-on`'s own engine-interaction test
    already covers its behavior (from the earlier `debug-on-handler` cycle), and that
    `command_safety_test.sh` gained the name in the same commit that deleted the file.
- [x] **2.4** `MIGRATION.md` — add the move-aside rule itself, beside the note for the twelve
    (near `:62-66`, where the three-step port/prove/delete rule already lives, or immediately
    following the twelve-command note at `:157-167` — either is "beside" it). This is a correction,
    not new process: `openspec/changes/archive/2026-09-11-retire-read-only-shell-commands/archive-report.md:57-59`
    already claims this rule was "recorded in `MIGRATION.md` beside the note for the twelve." It was
    not — `MIGRATION.md:62-66` carries only the three-step rule, and the words "aside," "apartar,"
    "mover," and "gemelo" (the last of which does appear, but never in this context) turn up nothing
    resembling it. Record:
  - The rule: before deleting a shell twin, move it aside, run the full suite, note what breaks,
    restore, repair, then delete. A textual check is worth running first to know where to look; it is
    never a proof of absence.
  - Why it exists: three progressively better textual checks each proved insufficient
    (`archive-report.md:39-59`) — no check at all, a plain grep by command name (mostly false
    positives, since `list` is also a subcommand of `db`/`worktree`/`proxy`/`snapshot`), and a grep
    plus resolving the entry-point variable, written up mid-cycle as "the check that works," which
    still missed `anonymisation_test.sh` and `validation_cache_test.sh` because both call a helper
    named plain `run` — an enumeration of the wrappers you already know cannot find the one you do
    not. Only moving the files aside and running everything found both, plus a fourth gap no
    invocation grep could ever see: `tui_actions_test.sh`, which asserts every dashboard-offered
    command has a `.sh` file and invokes nothing at all.
  - This change's own use of the rule: both files moved aside, full suite plus `go test ./...` run,
    identical to baseline (59 of 1003 assertions, same seven suites, empty line-by-line diff),
    restored by copy and `md5`-verified — recorded as confirmation the rule still holds, not as a
    substitute for stating it.

### Verification for commit 2

- [x] **2.5** No test enforces prose. Confirm by rereading `xdebug_host.go` and `wrappers.go`'s
    `debugOn` against the new §5.8.1 text line by line; nothing else catches a documentation error
    here.
- [x] **2.6** `migration_status_test.sh` still passes (the two new notes are prose outside the table
    the test parses; the table row for `debug-on` at `:261` and the `34 de 65` counter are unchanged).
- [x] **2.7** `go test ./...`, `go vet ./...`, `gofmt -l ./internal ./cmd`, `gofmt -l ./dockergento` —
    unchanged from commit 1's results, since commit 2 touches no source or test file.

---

## Deliberate non-actions (record so they are accepted, not discovered)

- **Twelve dangling citations stay.** `internal/cli/wrappers.go:376, :381, :468, :472, :474, :496,
  :519`; `internal/cli/wrappers_test.go:1476, :1484`; `openspec/specs/go-entrypoint/spec.md:466, :570,
  :606` — all twelve verified at those exact lines. Each cites `debug-on.sh:NN` as provenance for a
  ported behavior and remains true after deletion: it asserts what the shell did, which deletion does
  not falsify. The line numbers are what make each claim checkable via `git show <sha>:console/
  commands/debug-on.sh`. Cost, stated plainly: a reader's first instinct on hitting one of these is to
  open the file, and that now fails twelve times. Paid once, by the third-step note in commit 2
  (2.3), which is where a puzzled reader following one of these citations into nothing should end up.
- **`spec.md:606`'s premise goes vacuous, and is left that way.** Its GIVEN — "nothing in this
  repository sets `XDEBUG_HOST` except `set_xdebug_host_property.sh` itself" — sits inside the
  ~600-line "A ported command's engine interaction is provable without Docker" requirement
  (`:253-859`), spanning `debug-off`, `magento`, `logs`, and others. The `MODIFIED` workflow requires
  copying the entire requirement block to touch any one clause in it; doing that here for one premise
  whose `WHEN`/`THEN`/test are unaffected — and whose conclusion the deletion strengthens, not weakens
  — is disproportionate. Left vacuous, not edited.
- **`data/command_descriptions.json` is not touched.** `debug-on` must stay declared:
  `print_help.sh:54-59` unions declared names lacking a file into `hm --help`, which is exactly how
  `list`, `describe`, and `logs` stayed listed after their own retirement. Removing the entry would
  drop `debug-on` from help and turn `bin/run debug-on` from `command_moved_to_binary` into
  `command_not_found`.
- **No source code is written or edited.** `internal/cli/` and `dockergento/` are unchanged in every
  respect; the Go implementation was proved in its own earlier cycle. This change only removes what it
  superseded and corrects documentation about it.

## The corollary (a note, not a task)

The move-aside experiment (both files removed, full suite plus `go test ./...` run) came back
identical to baseline: 59 of 1003 assertions failing in both runs, the same seven suites, an empty
line-by-line diff. No repair work is owed — none was found, and no task above hunts for breakage that
was measured at zero. The verification tasks above (1.6-1.7, 1.12) exist to confirm that result holds
at commit time, not to find something new.

Read plainly: if removing an entire shell implementation breaks nothing, nothing was testing it. The
green here is an absence of coverage over `debug-on`'s shell half, not evidence that it was safe all
along — the two are different claims, and only the first is what this change measured.

---

## Size

**Estimate: roughly 145-165 changed lines**, higher than the proposal's own 115-145.

| Unit | Lines | Note |
|---|---|---|
| The two deletions (1.1, 1.2) | 57 | Both files read in full |
| `RETIRED` entry + `:64` comment count (1.3) | 4-6 | Matches proposal |
| `cli_contract_test.sh` pair (1.4) | 8-12 | 8 for the bare pair per the `version` precedent at `:87-94`, verified; a few more if a comment is added, as the existing pairs each carry one |
| Architecture doc rewrite + table row (2.1, 2.2) | ≈50-56 | ≈24 removed, ≈28-30 added at §5.8.1, plus the one-line table row |
| `MIGRATION.md` third-step note (2.3) | 8-12 | Shorter than the list/describe/logs note (`:177-188`, ~12 lines) since it names one command, not three |
| `MIGRATION.md` move-aside rule (2.4) | 18-28 | New — not priced in the proposal's own size table, which budgeted only the third-step note |

The difference from the proposal's 115-145 is item 2.4: the proposal's Affected Areas and size table
account for a single "third-step note beside the existing ones," not the separate move-aside-rule
correction this tasks phase adds as its own item (per the brief that produced this file, and confirmed
against source: `archive-report.md:57-59` claims the rule was recorded, `MIGRATION.md:62-66` shows it
was not). That is a real, distinct block of prose — the three-checks history alone is worth ten-plus
lines if it is to explain *why* the rule exists rather than just stating it — and it was not in either
the exploration's or the proposal's size accounting.

**Do I agree with the proposal's "at-or-under" framing?** Yes, for the units the proposal priced —
the deletions, the `RETIRED` entry, the `cli_contract_test.sh` pair, and the architecture rewrite all
have the same mechanical bounds the proposal gave them, and the move-aside experiment already measured
the unplanned-repair risk at zero, which is what usually drives this repository's 1.3-2x overshoot.
The move-aside-rule addition (2.4) does not change that reasoning — it is not discovered repair work,
it is a known, boundable piece of prose — but it does mean the correct total sits somewhat above the
proposal's own ceiling of 145, for a reason the proposal did not have in view when it wrote that
number. I'd bound the whole change at 175 rather than 145, and expect it to land in the 145-165 range
stated above.

## Constraints carried through execution

- No source code is written or edited in either commit.
- Nothing in this change runs against a live Docker daemon; `hm debug-on` and `bin/run debug-on` are
  never invoked directly. The `DOCKER_HOST`-pinned method from the move-aside experiment (1.12) is the
  only way the full suite runs.
- Any size or diff check derives its base at check time (`HEAD`, `git merge-base`), never a hardcoded
  commit hash — the mutation check (1.6) restores from "the parent commit," resolved when it is run.
