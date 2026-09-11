# Proposal: The third step, for the three read-only commands

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis in `docs/research/2.0-arquitectura.md`. Same rationale as
`retire-ported-shell-commands` and its predecessors.

## Intent

`MIGRATION.md:62-66` states the rule in three steps: **a command ported → its test in Go → its shell
implementation and its bash test are deleted**. `list`, `describe` and `logs` finished step two in
`read-only-commands-go-tests`; all three are routed in Go (`internal/cli/run.go:53`, `:55`, `:63`)
and still carry a full shell twin — `console/commands/list.sh` (61), `describe.sh` (92), `logs.sh`
(70), **223 lines** confirmed by line count.

**The deletions are three `rm`s. The deliverable is the ten assertions that would pass for the wrong
reason once the files go.** `bin/run`'s `check_command_dir` (`bin/run:72-90`) answers
`command_moved_to_binary` with `HM_EXIT_USAGE=2` for any declared command with no file, so every
suite that drives `bin/run` for these three gets a refusal instead of a command — and a refusal is
exit 2, empty stdout, and a message containing the command's own name.

The previous cycle proved the cost of not looking: it retired `version.sh` without running
`tests/integration/logs_launch_version_test.sh`, which drove `version` through `bin/run`. Four
assertions were red for a day and two were green for the wrong reason — one expected exit 2 and got
it from `command_moved_to_binary`, the other looked for the tool's name and found it inside
`'version' is part of the hm binary now`. Repaired in `379d317`; the repair and its comment are
still visible at `logs_launch_version_test.sh:10-15` and `:59-63`.

## Correction to the exploration

The exploration's section C states that no parity oracle references these three. **That is wrong**,
verified by reading the files:

- `tests/integration/go_passthrough_test.sh:57-83` is the `describe` parity block and `:85-106` the
  `list` one — both drive `$SHELL_CLI` (`:13` = `bin/run`) through `both()` (`:45-48`).
- `tests/integration/go_orchestration_test.sh:165-181` is the `logs` parity block, driving
  `$SHELL_CLI` (`:14`) at `:166`, `:172` and `:177`.

Two of the ten wrong-reason passes live in the block the exploration did not open — the same class of
miss, in the same cycle shape, one iteration later. `openspec/specs/go-entrypoint/spec.md:586-588`
already anticipated this in writing: *"this comparison is lost once `logs.sh` is deleted, so the
cycle that deletes it inherits both `Configuration` and this comparison."* This change is that cycle.

`permissions_test.sh` remains a **false positive**, re-verified: `:22` and `:58` are `assert_contains`
over the literal string `Bash(hm describe:*)` in emitted config. No execution, no change.

## Scope

### In Scope

- **Repair ten wrong-reason assertions and four loud-failure sites** across seven suites, each with a
  named corrected proof (table below).
- **Retire three parity blocks** whose shell half ceases to exist, replacing each with a Go-only
  assertion against a fixed oracle rather than against the other implementation — the disposition
  `MIGRATION.md:64-66` already prescribes ("la prueba de paridad se queda en bash" only *while both
  halves exist*).
- **Delete `console/commands/list.sh`, `describe.sh`, `logs.sh`** (223 lines).
- **`tests/unit/command_safety_test.sh:69-70`**: the `RETIRED` list gains `list describe logs`.
- **`tests/integration/cli_contract_test.sh:71-94`**: one `command_moved_to_binary` case for
  `describe`, the pin the spec names at `go-entrypoint:776`.
- **`MIGRATION.md:157-176`**: a sibling block recording the third step for these three.

### Out of Scope

- No behaviour change: no argument, prompt, `--json` document, exit code or message moves.
- `data/command_descriptions.json` untouched — the entries are what keeps all three in `--help`.
- `console/helpers/print_help.sh:54-67` untouched — the declared∪glob union already generalises.
- `tests/unit/skills_test.sh:75-86` and `tests/unit/migration_status_test.sh` untouched, verified.
- No other `.sh` retired; `bin/run` keeps serving the rest unchanged.
- The `set -e` asymmetry (`logs.sh:2` has `set -uo pipefail`, the other two `-euo`) — investigated
  and accepted in the previous cycle's design; no `varnish-on`-shaped gap here.

## The ten wrong-reason passes, and what each corrected assertion proves

Strategy is decided **per case**, not per file. Three answers: *route* (point the call at `bin/hm`),
*tighten* (assert a fact a refusal cannot satisfy), *retire* (the shell half stops existing).

| # | Where | Passes because | Decision | What the corrected assertion proves |
|---|---|---|---|---|
| 1 | `introspection_test.sh:57-59` | `invalid_argument` and `command_moved_to_binary` are both 2 | **route + tighten** | `list` rejects an undeclared option: exit 2 **and** `.error.type == "invalid_argument"` on stderr |
| 2 | `introspection_test.sh:115-116` | `assert_json_field` with an **empty expected value** — `jq -r` answers `""` on empty stdin and on a refusal alike, and the helper swallows jq's stderr (`tests/lib/assert.sh:146-155`, the `2>/dev/null` at `:149`) | **route + tighten** | add a `'.data.magento \| has("version")' == "true"` guard before the `== ""` check: the key exists and is empty for want of `composer.lock`, which an empty document cannot satisfy |
| 3 | `introspection_test.sh:165-166` | `assert_not_contains '"schema_version"'` holds trivially on empty stdout (dormant — guarded by `HM_TEST_PROJECT`) | **route only** | `:163` already asserts `assert_contains "$STDOUT" "URLs"` on the **same** stdout, so empty output fails loudly there; the negative needs no tightening once it is paired |
| 4 | `logs_launch_version_test.sh:169-171` | `missing_value` and `command_moved_to_binary` are both 2 | **route + tighten** | `--tail` without a value is a missing-value usage error: `.error.type == "missing_value"`. Exactly the repair `379d317` made for `version` at `:59-63` |
| 5 | `logs_launch_version_test.sh:192-193` | `assert_not_contains "hello-from-phpfpm"` holds on empty stdout | **route only** | `:190` asserts `hello-from-db` on the same stdout of the same run; the pair proves the service filter narrowed, not that nothing printed |
| 6 | `logs_launch_version_test.sh:208-209` | same shape, JSON-envelope check | **route only** | `:206` is the same-stdout positive; same reasoning |
| 7 | `project_name_test.sh:96-98` | **two calls to the same broken helper compared with each other** — `hm_name()` (`:53-55`) degrades to `""` on both sides and two broken oracles agree | **route + tighten** | assert the shared value equals the literal `hmnametest`, not that two oracles match: a configured name is inherited by a copy, with a value that exists |
| 8 | `project_name_test.sh:114-115` | same shape | **route + tighten** | assert both equal the literal `principal` (already the expected at `:112`): a worktree does not invent a second environment |
| 9 | `go_passthrough_test.sh:80-83` | `both describe --nonsense` → `assert_equals "$SHELL_STATUS" "$GO_STATUS"`; shell's 2 becomes `command_moved_to_binary`, Go's stays `invalid_argument`. **Byte-for-byte the `version` regression shape** | **retire** | keep the Go-only facts — exit 2 **and** `.error.type == "invalid_argument"`; drop the comparison whose other side no longer exists |
| 10 | `go_passthrough_test.sh:99-102` | `both list --nonsense`, identical shape | **retire** | same: `list` refuses an undeclared option with a usage error of its own |

**Two shapes this repo has not named before**, both worth carrying into the spec phase: `assert_equals`
between two calls to the same now-broken helper (#7, #8 — equality holds without the property under
test ever being exercised), and `assert_json_field` with an empty expected value (#2).

### The four that fail loudly, and still need rewriting

| Where | Fix |
|---|---|
| `concurrent_safety_test.sh:61`, `:64` (`describe`) | add `GO_BINARY`; `worktree add` and `clean` keep `bin/run` (`:11`) |
| `mail_catcher_test.sh:203` (`describe`) | add `GO_BINARY`; `doctor` is still shell and keeps `bin/run` (`:181`) |
| `go_passthrough_test.sh:63-78`, `:91-97`, `:104-106` | parity comparisons retired; the whole document, the table and the withheld credentials are kept as Go-only assertions |
| `go_orchestration_test.sh:165-181` | parity retired; the unknown-service refusal keeps exit 5 and the name `phpfpm` against the container's own fixed output, not against `bin/run` |

**Routing convention**: where the *whole file* is one of the three (`introspection_test.sh`), swap
`HM` at `:8` — one line. Where the file is mixed, add a `GO_BINARY` variable and rewrite only the
affected call sites, following the precedent `logs_launch_version_test.sh:10-15` already set for
`version`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `go-entrypoint` — **"A command proved in Go has no shell twin"** (`spec.md:749-756`): the requirement
  enumerates *"the twelve retired in this change"* by name and must become fifteen, adding `list`,
  `describe`, `logs`. Its pin sentence (`:765-767`) reads "walks the twelve by name" — same edit.
- `go-entrypoint` — **"What still covers the unknown-service refusal"** (`spec.md:576-591`): this is the
  obligation the spec explicitly deferred to this cycle (`:586-588`). The delta must say where the
  Go/shell comparison goes. It must **also correct a stale reference**: `:583-585` claims
  `logs_launch_version_test.sh:148-153` "runs the Go binary" — it does not. That case now sits at
  `:162-167` and runs `bin/run` through `run_in_lab` (`:26-32`, `HM` at `:8`). Line numbers drifted;
  the claim drifted with them.
- `go-entrypoint` — **"A usage error returns before any engine call"** (`spec.md:594-601`) already names
  `list`, `describe` and `logs`. No requirement text changes; cases #1, #4, #9 and #10 become its
  integration-level evidence, which it currently lacks.

`go-entrypoint`'s **"A ported command's shell entry point still answers by delegating"**
(`spec.md:788-797`) is **not** touched: these three get no delegating stub, and `:793-795` already
routes that case to the no-shell-twin requirement.

## Approach: two commits, and why the order is not arbitrary

**Commit 1 — repair the suites, with the three `.sh` files still present.**
Every corrected assertion is written and run while the old behaviour still exists, so each one can be
checked against what it is supposed to prove: assert `invalid_argument`, watch it pass on the live
shell command; the same assertion against a refusal would say `command_moved_to_binary`. The parity
blocks are retired here too — they stop comparing, but the Go side still answers, so the suites stay
green. Nothing in this commit depends on a file being gone.

**Commit 2 — delete the three files**, extend `RETIRED`, add the `cli_contract_test.sh` case and the
`MIGRATION.md` block.

**Reversed, this breaks in three ways.** Deleting first makes every affected suite red before a
single repair is written, so a repair that genuinely works is indistinguishable from one that merely
stops the suite complaining. The ten wrong-reason cases become invisible: with the old behaviour
already gone there is nothing left to compare a corrected assertion against, and a green assertion
proves nothing about *which* exit 2 it saw. And the intermediate commit is red in history — which is
not hypothetical. It is what happened to `version`: retired without running the suite that drove it,
four assertions red for a day, two green for the wrong reason, repaired only in `379d317`.

**One hard exception, and it runs the other way.** `command_safety_test.sh:69-70`'s `RETIRED` list —
confirmed at those exact lines — is walked at `:74-77`, where `[ -f "$COMMANDS_DIR/$name.sh" ]` is
itself drift. Adding `list describe logs` while the files still exist makes the suite RED. **That edit
belongs to commit 2, in the same commit as the deletions**, not commit 1. Its "and every entry has a
file or a route" case (`:53-61`) stays green throughout because all three are routed.

**The cheap check that would have caught `version`.** Before deleting, for each command:
`rtk proxy rg -n '\blist\b|\bdescribe\b|\blogs\b' tests/` — then, for **every** hit, resolve which
entry-point variable the invocation actually uses. A name-only grep is necessary and not sufficient:
it returns 35 files, most of them prose, and the two suites this exploration missed hide the call
behind `$SHELL_CLI`, while two it caught hide it behind a `run_in`/`run_in_lab` wrapper. The check is
grep **plus** resolving the variable.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `console/commands/list.sh`, `describe.sh`, `logs.sh` | Removed | 223 lines, commit 2 |
| `tests/integration/introspection_test.sh:8`, `:57-59`, `:115-116` | Modified | binary swap + two tightenings |
| `tests/integration/logs_launch_version_test.sh` (8 `logs` sites, `:119` describe) | Modified | `logs`/`describe` to the binary; `launch` stays on `bin/run` |
| `tests/integration/project_name_test.sh:53-55`, `:80`, `:96-98`, `:114-115`, `:157` | Modified | helper to the binary; `start`/`down` (`:151`, `:159`) stay |
| `tests/integration/concurrent_safety_test.sh:61`, `:64` | Modified | `describe` to the binary |
| `tests/integration/mail_catcher_test.sh:203` | Modified | `describe` to the binary |
| `tests/integration/go_passthrough_test.sh:57-106` | Modified | `describe`/`list` parity retired, Go-only assertions kept |
| `tests/integration/go_orchestration_test.sh:165-181` | Modified | `logs` parity retired, Go-only refusal kept |
| `tests/unit/command_safety_test.sh:69-70` | Modified | `RETIRED` gains three names — **commit 2** |
| `tests/integration/cli_contract_test.sh:71-94` | Modified | one `command_moved_to_binary` case for `describe` |
| `openspec/specs/go-entrypoint/spec.md:576-591`, `:749-767` | Modified | delta specs |
| `MIGRATION.md:157-176` | Modified | third step recorded for these three |

## Size and delivery

The exploration's 320-370 nominal predates `go_passthrough_test.sh` and `go_orchestration_test.sh`.
Revised:

| Commit | Contents | Nominal |
|---|---|---|
| 1 | seven suites repaired, three parity blocks retired | ~190-250 |
| 2 | 223 deleted + `RETIRED` + `cli_contract` case + `MIGRATION.md` | ~245 |

**Each commit fits the 400-line budget on its own — yes, I believe that**, with margin in both: commit
2 is 223 lines of wholesale file deletion plus ~22 of edit, and commit 1's largest single item is
`logs_launch_version_test.sh` at ~40-60. **Cumulatively ~435-500 nominal, and at this repo's recorded
1.3-2x overshoot (170→273, 50-90→184, 120-180→302, 841→1364) plausibly 550-700 — over budget.** So the
delivery decision belongs **before apply, not during**: `sdd-tasks` must forecast it and `ask-on-risk`
must resolve it up front.

Contingency if commit 1 alone forecasts over 400: split the two parity retirements into a commit of
their own. That is a natural seam — they are removals of comparisons, not repairs of assertions.

Measured **against this change's own parent**, never a literal hash: record
`BASE=$(\git rev-parse HEAD)` before the first commit, then
`\git diff --shortstat "$BASE"..HEAD -- . ':(exclude)openspec'`; per commit,
`\git diff --shortstat <commit>^..<commit> -- . ':(exclude)openspec'`. A hardcoded base has silently
measured unrelated commits twice in this repo.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Another suite drives these three through `bin/run` and is still unfound | Med | The grep-plus-resolve-the-variable check above, run for each of the three before commit 2; the exploration already missed two files by grepping names alone |
| A corrected assertion is itself vacuous | Med | Each of the ten names the fact it proves in the table above; `sdd-verify` checks each by breaking the Go behaviour on purpose and confirming it goes red |
| `RETIRED` edited in commit 1 | Certain to break if done | Stated as a hard exception; `command_safety_test.sh` is red the moment a `RETIRED` name still has a file |
| Retiring parity loses coverage nobody replaces | Med | Each retired comparison is replaced by a Go-only assertion against a fixed oracle in the same commit, and `go-entrypoint:576-591` is amended to say where the guarantee now lives |
| Commit 1 overshoots 400 | Med | The three-commit contingency above |
| Someone scripted `console/commands/<name>.sh` directly | Low | `hm <name>` and `bin/run <name>`'s `command_moved_to_binary` message both keep answering |

## Rollback Plan

Two commits on `release/2.0.0`; `\git revert` in reverse order restores the three files, the `RETIRED`
list, the `cli_contract` case and every suite edit. Reverting commit 2 alone leaves commit 1
consistent and green — the repaired suites drive the binary, which does not care whether the shell
twins exist.

## Dependencies

`read-only-commands-go-tests` (archived): `list_test.go`, `describe_test.go` and `logs_test.go` are
what makes step three legal. No external dependency.

## Success Criteria

- [ ] `rtk proxy fd -e sh . console/commands` finds no `list.sh`, `describe.sh` or `logs.sh`.
- [ ] `tests/run.sh` green at **both** commits, including `introspection_test.sh`,
      `logs_launch_version_test.sh`, `project_name_test.sh`, `concurrent_safety_test.sh`,
      `mail_catcher_test.sh`, `go_passthrough_test.sh`, `go_orchestration_test.sh`,
      `command_safety_test.sh` and `cli_contract_test.sh`.
- [ ] Each of the ten corrected assertions fails when the Go behaviour it names is broken on purpose
      — checked one by one, not asserted collectively.
- [ ] No surviving assertion in `tests/` compares two invocations of the same helper to each other,
      and no `assert_json_field` in the touched files carries an empty expected value without a
      `has()` guard beside it.
- [ ] `bin/run describe` exits 2 with `.error.type == "command_moved_to_binary"` and the hint
      `hm describe`, with no Docker validation first.
- [ ] `hm --help` still lists all three, proven by capturing `bin/run --help` before commit 1 and
      diffing after commit 2.
- [ ] `go test ./... -short` passes, `gofmt -l ./cmd ./internal` empty, `go vet ./...` clean.
- [ ] `MIGRATION.md` records the third step for these three, and `go-entrypoint`'s retired list names
      fifteen commands, not twelve.
