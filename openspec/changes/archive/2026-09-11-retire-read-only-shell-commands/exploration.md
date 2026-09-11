# Exploration: retire-read-only-shell-commands

Change: `retire-read-only-shell-commands`
Engram topic: `sdd/retire-read-only-shell-commands/explore` (project `hiberus-dockergento`, observation #833)
Date: 2026-09-11

Step three of `MIGRATION.md`'s rule — port, prove, delete — for `list`, `describe` and `logs`. They
finished step two yesterday in `read-only-commands-go-tests`. Their twins are
`console/commands/list.sh` (61 lines), `describe.sh` (92) and `logs.sh` (70): 223 lines.

**The deletions are three `rm`s. The change is the eight suites that invoke these commands.**

## What makes this cycle non-trivial

`bin/run`'s `check_command_dir` (`bin/run:72-90`) answers `command_moved_to_binary` with
`HM_EXIT_USAGE=2` for any declared command that has no file. So the moment the three files go, every
suite driving `bin/run` for them starts getting a refusal instead of a command.

The previous retirement cycle hit exactly this and did not notice: it retired `version.sh` without
running `logs_launch_version_test.sh`. Four assertions failed for a day, and two kept *passing for
the wrong reason* — one expected exit 2 and got it from `command_moved_to_binary` rather than
`invalid_argument`, the other looked for the tool's name and found it inside the sentence
`'version' is part of the hm binary now`. Repaired in `379d317`.

## A. Which entry point each suite drives

| Suite | `HM=` | Reaches `bin/run` for | Left alone in the same file |
|---|---|---|---|
| `introspection_test.sh` | `bin/run` (:8) | `list`, `describe` — **the whole file** | nothing |
| `project_name_test.sh` | `bin/run` (:9) | `describe` via `hm_name()` (:53-55, used 8×), `list` (:157) | `start`, `down`, `worktree` |
| `concurrent_safety_test.sh` | `bin/run` (:11) | `describe` (:61, :64) | `worktree add`, `clean` |
| `mail_catcher_test.sh` | `bin/run` (:181) | `describe` (:203) | `doctor` — still shell |
| `logs_launch_version_test.sh` | `bin/run` (:8) **and** `bin/hm` (:15) | `logs` (8 sites), `describe` (:119) | `launch` still shell; `version` already on `bin/hm` |
| `go_web_test.sh` | `bin/hm` (:13) | — | `list` (:72) already safe |
| `permissions_test.sh` | `bin/run` (:8) | **nothing** | see below |

**`permissions_test.sh` is a false positive** in the orchestrator's original risk table, verified:
`:22` and `:58` are `assert_contains` over a `jq` of the config the `permissions` command emits,
checking the literal string `Bash(hm describe:*)`. No execution. No change needed.

## B. The eight assertions that would pass for the wrong reason

This is the part worth the cycle. Seven unconditional, one dormant:

| # | Where | Why it would still pass |
|---|---|---|
| 1 | `introspection_test.sh:57-59` | expects exit 2 for `list --nope`; `invalid_argument` and `command_moved_to_binary` are both 2 |
| 2 | `introspection_test.sh:115-116` | expects `.data.magento.version == ""`; the refusal goes to stderr, stdout is empty, and `jq -r` on empty input also yields `""` |
| 3 | `introspection_test.sh:165-166` | `assert_not_contains "$STDOUT" '"schema_version"'` holds trivially on empty stdout (dormant — guarded by `HM_TEST_PROJECT`) |
| 4 | `logs_launch_version_test.sh:169-171` | `logs --tail` expects 2; `missing_value` and `command_moved_to_binary` are both 2 |
| 5 | `logs_launch_version_test.sh:192-193` | `assert_not_contains "$STDOUT" "hello-from-phpfpm"` holds because stdout is empty, not because the service filter worked |
| 6 | `logs_launch_version_test.sh:208-209` | same shape, on the JSON-envelope check |
| 7 | `project_name_test.sh:96-98` | `assert_equals "$(hm_name a)" "$(hm_name b)"` — both sides degrade to `""`, so two broken oracles agree |
| 8 | `project_name_test.sh:114-115` | same shape |

Two sub-classes here are new to this repo's list, and both are worth naming in the design:

- **Two calls to the same broken helper compared with each other** (#7, #8). Equality holds without
  the property under test ever being exercised. Nothing about the assertion looks wrong.
- **`assert_json_field` with an empty expected value** (#2). `jq -r` on empty stdin and `jq -r` on a
  refused command both answer `""`, and `assert_json_field` (`tests/lib/assert.sh:146-155`) swallows
  jq's own stderr.

`concurrent_safety_test.sh` and `mail_catcher_test.sh` fail loudly instead — their expected values
are non-empty — but they still need rewriting or the suites go red.

## C. What else blocks the deletion

| Item | Verdict |
|---|---|
| `tests/unit/command_safety_test.sh:69-70` `RETIRED` list | **must gain the three names**, or its "no file to drift from" case fails |
| `tests/unit/skills_test.sh:75-86` | already file-OR-route, fixed in a prior cycle — no change |
| `tests/unit/migration_status_test.sh` | no change; all three already tabled `go` |
| `console/helpers/print_help.sh:54-67` | union already generalized — no change |
| `data/command_descriptions.json` | no change; entries stay, which is what keeps them in `--help` |
| a parity oracle like `go_wrappers_test.sh` | none references these three — unlike the twelve-command cycle, there is nothing to rewrite |
| `MIGRATION.md:157-176` | should gain a short block for these three; no test requires it |

## D. Shell versus Go

`list.sh` and `describe.sh` carry `set -euo pipefail`; `logs.sh` has `set -uo pipefail` without
`-e`. That asymmetry was already investigated and accepted in the previous cycle's design: the only
failure that could matter is a command substitution assigned to a variable, which `-e` does not
catch anyway, and the script ends in `exec`. No `varnish-on`-shaped gap found.

Go already matches the shell's format strings, flag table (`--tail`/`-n`/`--since`/`--until`/
`--index`/`-f`/`-t`/`--no-color`/`--no-log-prefix`) and error shapes, all pinned yesterday by
`list_test.go`, `describe_test.go` and `logs_test.go`.

## E. Size and units

Deletions 223. Suite repairs: `introspection_test.sh` is a one-line binary swap since the whole file
is `list`/`describe`; `project_name_test.sh`, `concurrent_safety_test.sh` and `mail_catcher_test.sh`
each need a `GO_BINARY` var and their own call sites rewritten (~10-20 each), following the
`version` precedent of keeping `bin/run` for what still belongs to it;
`logs_launch_version_test.sh` needs a `run_in_lab`-shaped helper on the binary and eight call sites
(~40-60, the largest); `command_safety_test.sh` gains three names; `MIGRATION.md` gains prose.

Roughly **320-370**, and this repo has overshot every recent estimate by 1.3-2x (170→273, 50-90→184,
120-180→302, 841→1364), so plan for **400-550**.

**Recommended split, two commits:**

1. **Repair the suites first, with the three `.sh` files still in place.** Every assertion that would
   have passed for the wrong reason is corrected while the old behaviour is still there to compare
   against, and the suites stay green throughout. This is the discipline the `version` regression
   argues for.
2. **Delete the three files**, extend `command_safety_test.sh`'s `RETIRED` list, add the
   `MIGRATION.md` note.

It separates the judgment-heavy edits from the mechanical ones, and it means no commit in the
history has a red suite.
