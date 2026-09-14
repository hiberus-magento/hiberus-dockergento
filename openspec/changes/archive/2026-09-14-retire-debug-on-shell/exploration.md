# Exploration: retire-debug-on-shell

Change: `retire-debug-on-shell`
Engram topic: `sdd/retire-debug-on-shell/explore` (project `hiberus-dockergento`, observation #1050)
Date: 2026-09-13

The **third step** of `MIGRATION.md`'s rule — port, prove, delete — for `hm debug-on`, ported and
proved earlier today (`openspec/changes/archive/2026-09-13-debug-on-handler/`). Two files come out:
`console/commands/debug-on.sh` (38 lines) and `console/tasks/set_xdebug_host_property.sh` (19).

**Written to disk by the orchestrator.** The `sdd-explore` agent has no Bash tool, so it produced
the static pass and said plainly that it could not run the experiment the rule requires, marking its
own estimate provisional. The orchestrator then ran that experiment; §A records it.

## A. The move-aside experiment, actually run

This is the section the rule exists for, and the one the previous retirement cycle proved cannot be
replaced by any textual check.

**Method, and why it is shaped this way.** The full suite includes 47 integration suites, many of
which reach a Docker daemon. Running them against this machine's live daemon is forbidden — the user
has unrelated projects up. So both runs were made with `DOCKER_HOST` pointed at a nonexistent
socket, making the daemon unreachable **identically in both**, and the evidence is the *difference*
between them. Docker noise appears in both runs and cancels.

```
# baseline: files present
DOCKER_HOST=unix:///nonexistent-hm-retirement-probe ./tests/run.sh

# treatment: both files removed
rm console/commands/debug-on.sh console/tasks/set_xdebug_host_property.sh
DOCKER_HOST=unix:///nonexistent-hm-retirement-probe ./tests/run.sh
go test ./...
```

**Result: identical.**

| | Baseline | Treatment |
|---|---|---|
| Assertions failed | 59 of 1003 | 59 of 1003 |
| Failing suites | `cli_contract`, `concurrent_safety`, `go_engine`, `go_passthrough`, `logs_launch_version`, `tui_dashboard`, `worktree` | the same seven |
| `go test ./...` | 461 / 24 packages | unchanged, e2e included |

A line-by-line diff of the two full outputs, with paths and timings filtered, is **empty**. All 59
baseline failures are the pinned-unreachable daemon (`FAIL docker-daemon Docker daemon is not
running`) and a synthetic-project artefact (`The Magento directory './src' does not exist`), present
in both runs.

**Restoration** was by copying the backups back over the files — never `git checkout <file>`, which
in this repository has already silently reverted an entire uncommitted unit and left `git status`
clean. Byte-identity confirmed by `md5` before and after; `git status` clean.

### What this result means, and what it does not

It means the suite **cannot tell the difference** between these two files existing and not existing.
That is as strong as this method gets, and it is a materially different shape from the previous
retirement, where moving three files aside broke twelve assertions across eight suites.

It does **not** mean the deletion is proven safe in every sense. Everything being green with the
files gone proves that nothing we run fails. It does not prove there is nothing left to run.

And it carries an uncomfortable corollary worth stating rather than enjoying: **if removing the
entire shell implementation breaks nothing, nothing was testing it.** The green is the absence of
coverage, not the presence of proof. `debug-on`'s shell half went unexercised by 1003 assertions.

## B. The two files have different retirement mechanisms

- **`debug-on.sh` is a command.** `bin/run`'s `check_command_dir` answers `command_moved_to_binary`
  (exit 2) for any name with a `data/command_descriptions.json` entry but no file. That *is* the
  mechanism — `bin/run` has no separate list to edit.
- **`set_xdebug_host_property.sh` is a task**, `source`d only at `debug-on.sh:11`. Nothing else in
  the repository sources it, and `debug-off.sh` — still shell, out of scope — does not either.
  Tasks are never dispatched, so it needs no retirement mechanism: it disappears as a side effect.
- **`tests/unit/command_safety_test.sh:69-71`** keeps a hand-written `RETIRED` array, currently 15
  names, grown from 12 by the previous cycle. No assertion depends on it — it only self-checks its
  own entries — but it is the convention. Recommend adding `debug-on`, making 16.

## C. The four suites with a live dependency, each checked rather than assumed

| Suite | Dependency | Status |
|---|---|---|
| `tests/unit/skills_test.sh:75-86` | scans every `skills/*/SKILL.md` for `hm <command>`; `skills/dockergento-debugging/SKILL.md:24` has `hm debug-on`. Checks file-**or**-routed. | Passes unmodified — already generalised by an earlier cycle |
| `tests/unit/tui_actions_test.sh:30-39` | same file-or-route generalisation over `console/commands/tui.sh`'s action list | Not applicable: `tui.sh` never offers `debug-on` |
| `tests/unit/command_safety_test.sh:45-61` | driven from `command_descriptions.json` + `hm_routed_commands()` | Passes: `debug-on` is declared and routed (`internal/cli/run.go:126`) |
| `tests/unit/migration_status_test.sh` | same generic drive | Passes: `debug-on` tabled `go` at `MIGRATION.md:261` |

`tui_actions_test.sh` is named here deliberately: it was the suite the previous cycle's invocation
greps could never have found, because it asserts file existence and invokes nothing. It does not
apply this time — but it had to be looked at, not reasoned past.

## D. What the retirement stops proving: nothing

Stated as an explicit "none" rather than omitted, following the previous cycle's convention of
writing the empty answer to the question.

There is no parity oracle between the shell and Go implementations of `debug-on`. `wrappers_test.go`
proves the Go handler against `fakeEngine`, never against the script's real output, and no
integration or e2e test touches the command — xdebug needs a live container. So no comparison
retires here, because none ever existed. See §A's corollary.

## E. Help and descriptions are unaffected

`data/command_descriptions.json:244-247` keeps the `debug-on` entry, and
`console/helpers/print_help.sh:54-59` unions declared names lacking a file into `hm --help`. So
`debug-on` stays listed after deletion, exactly as `list`, `describe` and `logs` did.

## F. Dangling citations, a finding new to this cycle

`internal/cli/wrappers.go` (at `:376`, `:381`, `:468`, `:472`, `:474`, `:496`, `:519`) and
`wrappers_test.go` (`:1476`, `:1484`) carry comments citing `debug-on.sh:NN` as the provenance of a
ported behaviour. Nothing reads them; they break no test. After deletion they cite a file that is
gone from the tree, though still in history.

No precedent exists — the previous cycle did not edit analogous comments for `list`/`describe`/
`logs`. Recommend leaving them, on the same "true when written" principle that cycle applied to its
own stale comment, but naming the staleness in `tasks.md` so it is accepted rather than discovered.

## G. The architecture doc, deferred twice already

`architecture/05-state-and-configuration.md:207` and `:364-384` describe an xdebug host-detection
algorithm **neither implementation ever had**: the doc reads the default gateway where the script
reads `docker0`'s own inet address, gives the Mac default as `host.docker.internal` where the script
leaves it empty, and describes activation by renaming a `.ini.disabled` file where both
implementations use a `sed` comment/uncomment.

It has been carried forward unfixed by `capture-container-output` and `debug-on-handler`. It names
by hand the exact file this cycle deletes, which makes this the natural place to close it — but no
test enforces documentation, so it is a scope choice for the proposal rather than a requirement.

Deferring it a third time compounds a doc that is now wrong about a file that no longer exists.

## H. Size

- **Deletions**: 57 lines, both files read in full.
- **Repairs required by the experiment**: **none**. Nothing broke.
- **Additions by convention**, none forced by a failing assertion:
  - `command_safety_test.sh`'s `RETIRED` array: +1 name.
  - `cli_contract_test.sh`: one "and so does `debug-on`, once its shell twin is gone" pair, following
    the `set-host`/`version`/`describe` precedent at `:78-103` — roughly 15-20 lines.
  - `MIGRATION.md`: optional prose, not required by `migration_status_test.sh`.
  - The §G doc correction, if taken here: 15-25 lines.

**Estimate: 60-85 lines, or 75-110 with the doc fix.** Well under the previous retirement's 532,
and now on experimental rather than textual evidence — which is the difference that matters, because
that cycle's own write-up shows a grep-shaped estimate being wrong twice on this repository.

## Recommendation

Two commits, mirroring the established split: the additions first (`RETIRED` entry, the
`cli_contract_test.sh` pair, the doc fix if taken) with both files still present so every suite stays
green throughout; then the deletion.

## Open for the proposal

Whether to correct `architecture/05-state-and-configuration.md` here or defer it a third time.

## Risks

- §A's corollary: the clean result reflects an absence of coverage over the shell implementation, not
  an abundance of proof. Nothing about the deletion is risky; what it reveals about the two years
  that script ran untested is worth a moment's thought.
- Everything green with the files gone proves nothing we run fails. It does not prove there is
  nothing left to run.
