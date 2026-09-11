# Tasks: Retire the shell twins of `list`, `describe`, `logs`

**Size note, declared**: the phase skill budgets tasks.md at 530 words. The launch prompt asks for
per-case today/tomorrow/break statements, verified file:line citations, three full commit messages,
and per-unit + cumulative verification commands. Those are incompatible; the prompt wins.

**Authority order**: `design.md` is authoritative. `specs/go-entrypoint/spec.md` (this change's own
delta, `openspec/changes/retire-read-only-shell-commands/specs/go-entrypoint/spec.md`) was
reconciled to the design after both were written and is the source of truth for case numbers and
file:line citations. Every citation below was independently re-verified against the live files in
this session (not merely copied) — see the note at the end.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | Unit 1: 87 · Unit 2: 86 · Unit 3: 248 · Cumulative nominal: 421 |
| 400-line budget risk | Low per unit · High cumulative (this repo's recorded 1.3-2x overshoot puts the combined total at 550-840) |
| Chained PRs recommended | Yes |
| Suggested split | Three sequential commits on `release/2.0.0` (no PRs in this repo) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main (direct sequential commits; this repo has no PR flow) |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low

Each unit is self-contained, independently green, and its own nominal estimate (87 / 86 / 248) has
margin under 400 even at this repo's overshoot rate; only the cumulative total is over budget, and
the three-commit split is exactly the mitigation already required. No further decision gate is
needed before apply.

### Suggested Work Units

| Unit | Goal | Commit | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Repair the twelve assertion sites, six suites, files still present | 1 | `tests/run.sh tests/integration/introspection_test.sh` (+ 5 more, listed below) | live shell + built `bin/hm`, Docker required for most cases | revert alone leaves suites green against the shell command |
| 2 | Retire the two parity comparisons | 2 | `tests/run.sh tests/integration/go_passthrough_test.sh` and `go_orchestration_test.sh` | live shell + built `bin/hm`, Docker required | revert alone restores the shell/Go comparison |
| 3 | Delete the three shell twins | 3 | `tests/run.sh unit` and `tests/run.sh tests/integration/cli_contract_test.sh` | none required (unit suite + a project-less binary call) | revert alone restores the three files, `RETIRED`, and the `cli_contract` case |

---

## Case ledger (twelve sites, eight suites — `specs/go-entrypoint/spec.md`, "A wrong-reason assertion is corrected to prove the fact it names")

Verified against the live files; line numbers are the pre-edit ones.

| # | File:line | Today | Becomes | Plausible break caught |
|---|---|---|---|---|
| 1 | `introspection_test.sh:57-59` | exit 2 only | exit 2 **and** `.error.type == "invalid_argument"` | `list` validates after the engine, or reuses code 2 for another type |
| 2 | `introspection_test.sh:115-116` | `.data.magento.version == ""` | gains `'.data.magento \| has("version")' == "true"` guard before the `== ""` check | the key disappears, or the command answers nothing (both read `""`) |
| 3 | `introspection_test.sh:165-166` | `assert_not_contains '"schema_version"'` | unchanged text, entry point only | `:161-163`'s `assert_contains "URLs"` on the same `$STDOUT` fails on empty output first |
| 4 | `logs_launch_version_test.sh:169-171` | exit 2 only | exit 2 **and** `.error.type == "missing_value"` | `--tail` leaves `takesValue` (`orchestrate.go:96`, read-only) |
| 5 | `logs_launch_version_test.sh:192-193` | `assert_not_contains "hello-from-phpfpm"` | unchanged text, entry point only | `:188-190`'s `assert_contains "hello-from-db"` on the same run guards it |
| 6 | `logs_launch_version_test.sh:208-209` | `assert_not_contains "schema_version"` | unchanged text, entry point only | `:204-206`'s `assert_contains "hello-from-db"` on the same run guards it |
| 7 | `project_name_test.sh:96-98` | `assert_equals "$(hm_name a)" "$(hm_name b)"` | both sides asserted equal to the literal `hmnametest` | derivation would win over the committed name (both sides degrade to `""` today and still agree) |
| 8 | `project_name_test.sh:114-115` | `assert_equals "$(hm_name principal)" "$(hm_name una-rama)"` | both sides asserted equal to the literal `principal` | the main checkout's own resolution drifts |
| 9 | `go_passthrough_test.sh:80-83` | `both describe --nonsense`; `assert_equals "$SHELL_STATUS" "$GO_STATUS"` | Go-only: exit 2 **and** `.error.type == "invalid_argument"` | as case 1 |
| 10 | `go_passthrough_test.sh:99-102` | `both list --nonsense`, same shape | Go-only: exit 2 **and** `.error.type == "invalid_argument"` | as case 1 |
| 11 | `worktree_test.sh:73-74` (main checkout) | `.data.project.worktree == ""` | unchanged text, entry point only | `:76-77`'s `.data.project.name == "$PROJECT"` on the same `$STDOUT` fails on a refusal |
| 12 | `logs_launch_version_test.sh:211-213` | `logs --json db`; exit **1** (Compose refuses it) | exit 2 **and** `.error.type == "invalid_argument"` (the binary's own usage error) | the option table stops rejecting unknown `-*` after the command name — the same bug shape that reads `--tail 3`'s `3` as a service name |

**Two sites repaired in unit 1 that are not in the twelve** (real behaviour differences the
routing exposes, not wrong-reason passes — `specs/go-entrypoint/spec.md`, "A validation-order
difference exposed by routing"):

- `introspection_test.sh:70-72` (`describe --nope --json`): today asserts exit **4**, because
  `bin/run:289-299` (read-only) runs `validate_docker_compose.sh` before `describe.sh` ever parses
  `--nope`. Routed to the binary, the ordering `go-entrypoint:594-601` (read-only) requires means
  the same call answers exit **2** and `.error.type == "invalid_argument"`. Fails when: `describe`
  resolves the project before parsing.
- `worktree_test.sh:65-67` (an unregistered worktree): today asserts `.data.project.worktree ==
  "wt"`. `app/describe.go:108-110` (read-only) names a worktree only when it is registered
  (`app/resolve.go:59-63`, read-only), so the same call now answers `""`. Guarded only by
  adjacency — `:69-70`'s `.data.project.root == "$LAB/main"` on the same `$STDOUT` — the same weak
  shape as case 2 and case 11, accepted for the same reason.

**Two things deliberately left unspecified** (no assertion is written that cannot fail):
`worktree_test.sh:151`'s `describe --json` call stays as a second allowed command with no new
assertion — once `describe` is the binary's, the next line's `doctor` call already proves the same
fact. No byte-for-byte golden replaces `go_passthrough_test.sh:75-78`'s dropped table comparison —
see unit 2.

---

## Unit 1 — Repair six suites, files still present

- [x] 1.1 `tests/integration/introspection_test.sh`: swap `HM` at `:8` from
  `"$COMMAND_BIN_DIR/bin/run"` to `"$COMMAND_BIN_DIR/bin/hm"` — this alone routes case 1, case 3,
  and the `:70-72` fix.
- [x] 1.2 `tests/integration/introspection_test.sh:57-59` — case 1: add
  `assert_json_field "$STDERR" '.error.type' "invalid_argument"`.
- [x] 1.3 `tests/integration/introspection_test.sh:70-72` — change the expected status from `"4"`
  to `"2"` and add `assert_json_field "$STDERR" '.error.type' "invalid_argument"`; rename the
  `test_case` to say it is refused before it looks for a project.
- [x] 1.4 `tests/integration/introspection_test.sh:115-116` — case 2: add
  `assert_json_field "$STDOUT" '.data.magento | has("version")' "true"` before the existing
  `== ""` assertion.
- [x] 1.5 `tests/integration/logs_launch_version_test.sh`: add `run_in_lab_go()` after
  `run_in_lab()` (`:26-32`) — same body, `"$GO_BINARY"` in place of `"$HM"` — with a comment saying
  `logs`/`describe` are the binary's now and `launch` keeps `run_in_lab`.
- [x] 1.6 `tests/integration/logs_launch_version_test.sh` — route the 8 sites driving `logs`/
  `describe` through `bin/run` onto `run_in_lab_go`: `:119` (`describe`), `:163`, `:170`, `:182`,
  `:189`, `:196`, `:205`, `:212` (`logs`). Leave the ~13 `launch` sites on `run_in_lab`.
- [x] 1.7 `tests/integration/logs_launch_version_test.sh:169-171` — case 4: add
  `assert_json_field "$STDERR" '.error.type' "missing_value"`.
- [x] 1.8 `tests/integration/logs_launch_version_test.sh:211-213` — case 12: change the expected
  status from `"1"` to `"2"`, add `assert_json_field "$STDERR" '.error.type' "invalid_argument"`,
  and reword the `test_case` to say the flag is a usage error, not Compose's.
- [x] 1.9 `tests/integration/project_name_test.sh`: add `GO_BINARY="$COMMAND_BIN_DIR/bin/hm"` near
  `HM=` (`:9`); `HM` itself stays `bin/run` — name derivation is not ported
  (`bin/run:103` vs. `dockergento/app/resolve.go:116`, both read-only), so `hm_name()` must not
  route to the binary.
- [x] 1.10 `tests/integration/project_name_test.sh:53-55` — change `hm_name()`'s command from
  `describe --json` / `.data.project.name` to `db list --json` / `.data.project`, with a comment
  citing `db.sh:171-174` (read-only) and explaining why derivation stays out of scope.
- [x] 1.11 `tests/integration/project_name_test.sh:80` — change the inline probe command from
  `describe --json` to `db list --json` (the refusal under test is `bin/run:106`'s
  `no_project_name`, raised before any command file runs, so any surviving shell command is an
  equally good window).
- [x] 1.12 `tests/integration/project_name_test.sh:96-98` — case 7: assert both
  `hm_name("$LAB/otro-directorio")` and `hm_name("$LAB/otro-directorio-copia")` equal the literal
  `hmnametest`, not each other.
- [x] 1.13 `tests/integration/project_name_test.sh:114-115` — case 8: assert
  `hm_name("$LAB/principal")` equals the literal `principal` (paired with the already-correct
  `:112` assertion for `una-rama`).
- [x] 1.14 `tests/integration/project_name_test.sh:157` — route the `"$HM" list --json` inventory
  check onto `"$GO_BINARY"` (behaviourally identical today; keeps this suite green once `list.sh`
  is gone in unit 3).
- [x] 1.15 `tests/integration/concurrent_safety_test.sh`: add
  `GO_BINARY="$COMMAND_BIN_DIR/bin/hm"` near `HM=` (`:11`); route the two `describe` calls at
  `:61` and `:64` from `"$HM"` to `"$GO_BINARY"` — `worktree add`/`clean` keep `bin/run` (`:11`
  unchanged for those).
- [x] 1.16 `tests/integration/mail_catcher_test.sh`: add `GO_BINARY="$COMMAND_BIN_DIR/bin/hm"`
  near `HM=` (`:181`); route the `describe` call at `:203` from `"$HM"` to `"$GO_BINARY"` —
  `doctor` stays shell (`:210`, `:216` unchanged).
- [x] 1.17 `tests/integration/worktree_test.sh`: add `GO_BINARY="$COMMAND_BIN_DIR/bin/hm"` near
  `HM=` (`:12`) and a `run_in_go()` helper mirroring `run_in()` (`:53-61`) but calling
  `"$GO_BINARY"`.
- [x] 1.18 `tests/integration/worktree_test.sh:65-67` — route to `run_in_go` and change the
  expected value from `"wt"` to `""`; rewrite the `test_case` per the ledger note above (an
  unregistered worktree with no environment of its own resolves to the main checkout).
- [x] 1.19 `tests/integration/worktree_test.sh:72-74` — case 11: route to `run_in_go`; expected
  value stays `""`.
- [x] 1.20 `tests/integration/worktree_test.sh:114-120` and `:150-152` — route the `describe`
  (`:115`, `:151`) and `list` (`:119`) calls to `run_in_go`; no assertion text changes.
- [x] 1.21 Verify unit 1 against the live shell command first (each corrected assertion must be
  observed passing/failing on today's behaviour before the entry point moves), then against the
  binary, with all three `.sh` files still in place.

Commit message:
```
test: repair assertions the retirement of list, describe, and logs would break

Six integration suites drive list, describe, or logs through bin/run and would
start passing for the wrong reason once their shell twins are gone, because
bin/run's check_command_dir answers command_moved_to_binary with the same exit
code a real refusal would use. Repair each corrected assertion while the three
shell files still exist, so every fix can be checked against live behaviour:

- introspection_test.sh: swap to bin/hm; tighten list --nope and the
  composer.lock version guard; move describe --nope --json's expectation from
  exit 4 to exit 2, matching argument validation ordering.
- logs_launch_version_test.sh: route logs and describe onto a new
  run_in_lab_go helper; tighten logs --tail's missing-value case and logs
  --json db's undeclared-flag case.
- project_name_test.sh: keep bin/run for name derivation (not yet ported),
  but change the probe command to db list and replace two self-comparing
  assertions with literal expectations.
- concurrent_safety_test.sh, mail_catcher_test.sh: route their single
  describe call each onto the binary.
- worktree_test.sh: route describe/list calls onto the binary; correct the
  unregistered-worktree case, which now resolves to the main checkout instead
  of naming a worktree it has no environment for.
```

---

## Unit 2 — Retire the two parity comparisons

**What each stops proving** (`design.md` §3):

- `go_passthrough_test.sh:63-78` (whole `describe --json` document, `--with-secrets`, and the
  `--no-json` table compared shell vs. Go): stops proving that a field nobody wrote an assertion
  for, and the table's exact rendering bytes (column widths, spacing), still agree between the two
  implementations. Kept: named field assertions already at `:256-263` plus a documented-keys check,
  a credentials-has-password check, and a section-headings check on the `--no-json` output.
- `go_passthrough_test.sh:91-97`, `:104-106` (`list` document and table, shell vs. Go): same loss,
  for `list`. Kept: the envelope-shape assertions `introspection_test.sh:37-52` already makes are
  not duplicated; this block collapses to `.schema_version`, `.command`, `.data.count`'s type, and
  the piped-output default.
- `go_orchestration_test.sh:165-168` (`logs --no-color`, sorted, shell vs. Go): stops proving the
  Compose `service-name |` prefix format agrees between implementations. Kept: both `listo phpfpm`
  and `listo nginx` (the fixture's own `command:` strings, `:65`, `:70`) appear in the Go output.
- `go_orchestration_test.sh:171-174` (`logs nginx`, character for character): stops proving the
  exact bytes agree. Kept: a paired positive/negative on one stdout — contains `listo nginx`, does
  not contain `listo phpfpm`.
- `go_orchestration_test.sh:176-181` (unknown-service refusal, shell vs. Go, whole JSON compared):
  stops proving the two refusals are byte-identical JSON. Kept: exit 5, `.error.type ==
  "unknown_service"`, and the refusal names both `phpfpm` and `nginx`. This is also the coverage
  `specs/go-entrypoint/spec.md`'s "What still covers the unknown-service refusal" requirement
  points at, since `checkServices` (`orchestrate.go:197-221`, read-only) cannot fire against a fake
  project in an `internal/cli` test.

- [x] 2.1 `tests/integration/go_passthrough_test.sh:80-83` — case 9: replace `both describe
  --nonsense` with a solo `"$GO_BINARY" describe --nonsense` call; keep exit 2 and
  `.error.type == "invalid_argument"`; drop the `assert_equals "$SHELL_STATUS" "$GO_STATUS"` line.
- [x] 2.2 `tests/integration/go_passthrough_test.sh:63-78` — replace the whole-document and table
  comparisons with the field/keys/section-heading assertions described above.
- [x] 2.3 `tests/integration/go_passthrough_test.sh:91-97`, `:99-106` — case 10: same treatment for
  `list`; replace `both list --nonsense` with a solo Go call; collapse `:91-97` to the envelope
  checks described above.
- [x] 2.4 `tests/integration/go_orchestration_test.sh:165-181` — replace the three shell-vs-Go
  comparisons with the three Go-only assertions described above (fixture-string containment for
  `:165-168`; paired positive/negative for `:171-174`; exit 5 + `.error.type` + both service names
  for `:176-181`).
- [x] 2.5 Verify unit 2: both suites still pass with the three `.sh` files present (the Go side
  answers on its own now; nothing here depends on a file being gone).

Commit message:
```
test: retire the shell/Go parity comparisons for describe, list, and logs

go_passthrough_test.sh and go_orchestration_test.sh compare the shell and Go
implementations of describe, list, and logs directly. Once list.sh,
describe.sh, and logs.sh are deleted, the shell side of each comparison stops
existing, so replace each with a Go-only assertion against a fixed oracle —
the fixture's own configuration or the container's own output — instead of
against the other implementation. This is weaker in a stated way: the exact
document, table rendering, and JSON byte-identity comparisons had a second
opinion and now have none; only the fields, keys, and facts an assertion
actually names are still proven.
```

---

## Unit 3 — Delete the shell twins

**Ordering constraint, verified against the live file**:
`tests/unit/command_safety_test.sh:69-70` holds the `RETIRED` list; `:74-77` walks it, and `:75` is
literally `[ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"`. A listed name that
still has a file **is** drift. Adding `list describe logs` to `RETIRED` while the three files still
exist makes this suite red. **The `RETIRED` edit belongs only in this unit, in the same commit as
the deletions, and nowhere else** — units 1 and 2 must not touch `command_safety_test.sh:69-70`.

- [x] 3.1 Delete `console/commands/list.sh` (61 lines).
- [x] 3.2 Delete `console/commands/describe.sh` (92 lines).
- [x] 3.3 Delete `console/commands/logs.sh` (70 lines).
- [x] 3.4 `tests/unit/command_safety_test.sh:69-70` — add `list describe logs` to the `RETIRED`
  list (only now that 3.1-3.3 landed in the same commit).
- [x] 3.5 `tests/integration/cli_contract_test.sh` — add one `command_moved_to_binary` case for
  `describe`, after the existing `version` case (`:87-94`), following the same shape: `run
  describe`, assert exit 2 and `.error.type == "command_moved_to_binary"`, then `run --no-json
  describe`, assert the hint contains `"hm describe"`.
- [x] 3.6 `MIGRATION.md` — add a sibling block after the existing third-step note (`:157-176`), in
  Spanish, matching that block's style: record that `list`, `describe`, and `logs` are the
  fifteenth-through-seventeenth commands retired this way, name the two parity suites whose
  comparisons were replaced, and cross-reference `command_safety_test.sh`'s `RETIRED` list gaining
  the direction check for these three.
- [x] 3.7 Verify unit 3: `rtk proxy fd -e sh . console/commands` finds none of the three files;
  `tests/run.sh unit` and `tests/run.sh tests/integration/cli_contract_test.sh` are green; `hm
  --help` (captured before unit 1, per the cumulative check below) is unchanged.
- [x] 3.8 (found during the pre-deletion check, not in the original plan) Two suites the
  eight-suite ledger missed also drive `describe`/`list` through `bin/run` unrepaired:
  `tests/integration/anonymisation_test.sh` (two `describe --json` probes reading
  `.data.data.anonymised`, needing the real Go `describe` — routed to a new `run_go` helper) and
  `tests/integration/validation_cache_test.sh` (`bin/run`'s own shell-only validation cache is the
  subject under test, so the entry point had to stay `bin/run`; the probe commands changed instead,
  `describe` → `db list` for the validated case, `list` → `permissions` for the excluded-from-
  validation case). A third gap, `tests/unit/tui_actions_test.sh`, asserted every dashboard-
  triggered command has a `.sh` file — true of `describe`/`list`/`logs` only before this commit —
  and was repaired the same way `command_safety_test.sh` already had been, accepting a routed name
  with no file as real.

Commit message:
```
chore: delete the shell twins of list, describe, and logs

list.sh, describe.sh, and logs.sh (223 lines) are now dead: all three are
routed through internal/cli/run.go, proved by their own engine-interaction
tests, and every suite that drove them through bin/run was repaired in the
previous two commits to expect the binary's answers. command_safety_test.sh's
RETIRED list gains the three names in this same commit, never earlier, since
its drift check fails the moment a listed name still has a file.
cli_contract_test.sh gains one command_moved_to_binary case for describe, and
MIGRATION.md records the third step for these three.
```

---

## Verification

**Baseline, captured before unit 1**:
```bash
BASE=$(\git rev-parse HEAD)
go test ./...                         # 378 tests, 24 packages, no -short
./tests/run.sh unit                   # exactly "unit"; 616 assertions
gofmt -l ./cmd ./internal ./dockergento   # empty
go vet ./...                          # clean
"$COMMAND_BIN_DIR/bin/run" --help > /tmp/help-before.txt
```

**Per unit** (run every suite that unit touches, by name — the discipline whose absence let the
`version` regression stand for a day):

- Unit 1: `tests/run.sh tests/integration/introspection_test.sh`,
  `tests/run.sh tests/integration/logs_launch_version_test.sh`,
  `tests/run.sh tests/integration/project_name_test.sh`,
  `tests/run.sh tests/integration/concurrent_safety_test.sh`,
  `tests/run.sh tests/integration/mail_catcher_test.sh`,
  `tests/run.sh tests/integration/worktree_test.sh`; then
  `\git diff --shortstat <unit-1-commit>^..<unit-1-commit> -- . ':(exclude)openspec'` (expect ~87).
- Unit 2: `tests/run.sh tests/integration/go_passthrough_test.sh`,
  `tests/run.sh tests/integration/go_orchestration_test.sh`; then
  `\git diff --shortstat <unit-2-commit>^..<unit-2-commit> -- . ':(exclude)openspec'` (expect ~86).
- Unit 3: `rtk proxy fd -e sh . console/commands` (expect no `list`/`describe`/`logs`),
  `tests/run.sh unit`, `tests/run.sh tests/integration/cli_contract_test.sh`; then
  `\git diff --shortstat <unit-3-commit>^..<unit-3-commit> -- . ':(exclude)openspec'` (expect ~248).

**Cumulative**:
```bash
go test ./...                         # still 378, 24 packages
./tests/run.sh                        # everything: Go + all shell suites, both reported
gofmt -l ./cmd ./internal ./dockergento
go vet ./...
"$COMMAND_BIN_DIR/bin/run" --help > /tmp/help-after.txt
diff /tmp/help-before.txt /tmp/help-after.txt   # empty — all three still listed
\git diff --shortstat "$BASE"..HEAD -- . ':(exclude)openspec'   # expect ~421 nominal, 550-840 plausible
```

## Note on verification of this document

Every file:line citation above was re-read from the live repository in this session — the six
suites in unit 1, both suites in unit 2, `command_safety_test.sh`, `cli_contract_test.sh`,
`MIGRATION.md:130-177`, `bin/run:72-109` and `:283-302`, and `internal/cli/orchestrate.go:60-229`
— and matched the design and the delta spec exactly; no discrepancy was found between design.md,
the delta spec, and the code. The one thing worth flagging: the *archived* (pre-change) spec at
`openspec/specs/go-entrypoint/spec.md` still says "the twelve retired" and still cites
`logs_launch_version_test.sh:148-153` — that file is intentionally unchanged until this change
archives; the corrected numbers live only in the change's own delta spec, which is what this
tasks.md follows.
