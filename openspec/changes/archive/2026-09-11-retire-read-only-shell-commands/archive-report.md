# Archive report: retire-read-only-shell-commands

Archived: 2026-09-11
Engram: `sdd/retire-read-only-shell-commands/archive-report` (project `hiberus-dockergento`)
Branch: `release/2.0.0`, on top of `850e861`

## What shipped

The third step of `MIGRATION.md`'s rule — port, prove, delete — for `list`, `describe` and `logs`.
`console/commands/list.sh` (61 lines), `describe.sh` (92) and `logs.sh` (70) are gone; 50 files
remain under `console/commands/`. All three still answer through the binary, `bin/run` refuses them
with `command_moved_to_binary` and exit 2, and `hm --help` still lists them, because
`data/command_descriptions.json` keeps their entries and `print_help.sh` unions declared names with
files.

| Commit | Unit | Lines |
|---|---|---|
| `d270500` | twelve assertion sites repaired across six suites, twins still present | 115 |
| `a79784a` | the three shell-versus-Go parity comparison blocks retired | 133 |
| `37c3ae8` | the three files deleted, `RETIRED` extended to fifteen, three more suites repaired | 284 |

532 changed lines against a 421 estimate.

## The deletions were not the work

Twelve assertions would have kept passing once `bin/run` began answering `command_moved_to_binary`,
because `exitUsage` is shared by `invalid_argument`, `missing_value` and the retirement refusal. Each
now asserts the reason rather than the number.

Two shapes were new to this repo's catalogue:

- **`assert_equals` comparing two calls to the same helper.** `project_name_test.sh:96-98` and
  `:114-115` compared `hm_name a` against `hm_name b`. Once the helper broke, both sides degraded to
  `""` and agreed. Nothing about the assertion looked wrong.
- **`assert_json_field` with an empty expected value.** `introspection_test.sh:115-116` expected
  `.data.magento.version == ""`. `jq -r` answers `""` for empty stdin and for a refused command
  alike, and the helper swallows jq's own stderr.

## Three checks, each insufficient, and the one that worked

This is the durable finding, and it corrects a claim the orchestrator made mid-cycle.

1. **No check.** The previous retirement cycle deleted `version.sh` without running
   `logs_launch_version_test.sh`. Four assertions failed for a day and two passed for the wrong
   reason.
2. **Grep by command name.** 21-34 files, mostly false positives, because `list` is a subcommand of
   `db`, `worktree`, `proxy` and `snapshot`.
3. **Grep plus resolving the entry-point variable.** The orchestrator wrote this up as "the check
   that works" and concluded eight suites were the complete set. **It was also wrong.** It enumerated
   the wrapper names somebody already knew — `$HM`, `$SHELL_CLI`, `$GO_BINARY`, `run_in_lab`,
   `run_in`, `both` — and `anonymisation_test.sh` and `validation_cache_test.sh` call a helper named
   plain `run`. An enumeration of the wrappers you know cannot find the one you do not.
4. **Moving the files aside and running everything.** This found both missed suites, and it found a
   third gap no invocation grep could ever have seen: `tests/unit/tui_actions_test.sh` asserted that
   every dashboard-offered command has a `.sh` file, and invokes nothing at all.

**The rule, recorded in `MIGRATION.md` beside the note for the twelve**: before deleting a shell
twin, move it aside, run the full suite, note what breaks, restore, repair, then delete. A textual
check is worth running first to know where to look. It is never a proof of absence.

## What the retired parity comparisons stopped proving

Stated per block in the suites themselves rather than papered over. A comparison between two
implementations catches everything they disagree about, including what nobody thought to assert; an
assertion against a literal catches only what it names.

| Block | No longer catches |
|---|---|
| `describe` whole document | a field nobody names being dropped |
| `describe --with-secrets` | the credential *value* diverging |
| `describe` table | column widths, ordering, spacing |
| `list` document and envelope | table rendering, an unlisted field |
| `logs` whole stream | the `service |` prefix format |
| `logs` unknown service | the two refusals being byte-identical JSON |

## Verification

PASS with one non-blocking warning. All three deletions were mutation-checked: restoring each `.sh`
from `850e861` made `command_safety_test.sh` report `list(file) describe(file) logs(file)` and go
red; removing them again made it green.

The warning: a comment added in `d270500` said these commands had "no shell implementation any more"
two commits before they were deleted — the same *documentation claims a fact before the commit that
makes it true* shape this cycle was repairing elsewhere. True at HEAD, so not amended.

The verifier stated plainly that it used the reasoning method rather than the authoritative
move-aside-and-run-everything, and said what that can and cannot prove. Worth repeating: everything
being green with the files gone proves that nothing we run fails. It does not prove there is nothing
left to run.

**Final state, by execution**: `go test ./...` 378 across 24 packages; `tests/run.sh unit` 616;
introspection 20, logs_launch_version 35, project_name 23, worktree 26, concurrent_safety 9,
go_passthrough 67, go_orchestration 55, anonymisation 11, validation_cache 9, cli_contract 23;
`gofmt` and `go vet` clean. `mail_catcher_test.sh` keeps three failures from a sandbox network
restriction, unrelated and unchanged throughout the cycle.

## Carried forward

- **Project-name derivation was never ported.** `bin/run:100-103` derives `COMPOSE_PROJECT_NAME` from
  the directory when the property is absent; `dockergento/app/resolve.go:116` reads the property and
  stops. Verified live: the binary answers `{"ok":true,"name":""}` for a project without the key. An
  empty name is also the condition that makes `toolinfo/tooling.go:84-85` match any container lacking
  an `hm.project` label. This is why `project_name_test.sh` still asks `bin/run`. Needs its own
  backlog entry.
- `bash`, `exec` and `masquerade` need their own retirement cycle — `bash` and `exec` are joined at
  `execute()`, and `exec` has no Go test at all.
- `Configuration` joins the `commands` seam whenever a cycle needs `logs`'s `unknown_service` branch
  proved at the seam rather than in integration.

## A correction to this report's own genre

The archive agent reported this file as a delivered artifact and did not write it; the orchestrator
found it missing by listing the directory against the two previous archives. Recorded here because it
is the same class as everything above — a report asserting that something exists.
