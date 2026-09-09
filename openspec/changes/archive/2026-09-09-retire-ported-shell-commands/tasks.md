# Tasks: retire-ported-shell-commands

Delivery decision (fixed, not reopened here): **one SDD cycle, delivered as four commits**, matching
`design.md`'s final split exactly, with a recorded size exception for the cumulative 881 lines against
the 400-line budget. Each commit is under 400 lines on its own — see the per-unit totals below, taken
from `design.md`'s size table (§"Size, and an honest forecast").

Strict TDD throughout: every behavioural task states its RED first (the test that must fail, and
why it fails for the *right* reason — because the described route or file does not exist yet, not
because of an unrelated typo or an unrouted command). Tables cite `design.md` §5 "What RED means
here" and §6/§7 directly.

Order within a unit never leaves the tree broken: a caller stops pointing at a file in the same
commit the file is deleted in, never before.

---

## Unit 1 — The setup door becomes the binary; nothing is deleted yet

**Components:** c13 (`selfrun` + its test) + c14 (wiring + order tests) + c15 (bridge case) ≈ **170 lines**

Nothing under `console/commands/` is touched in this unit. `set-host.sh` still exists and is still
unreachable through `hm set-host` (already routed in Go); `install` and `ssl` still bridge to shell,
which is exactly what task 1.5 proves.

### 1.1 — [x] RED: the adapter package does not exist

- Write `dockergento/adapters/selfrun/runner_test.go` with:
  - `TestItRunsTheExecutableItWasGiven` — a recorder script written to `t.TempDir()` that records its
    own argv and exits 7; assert `Runner{Executable: <recorder>}.Run([]string{...})` returns exit 7
    and the recorder saw exactly those arguments.
  - `TestAnExecutableThatIsNotThereIsAnError` — a resolved path that does not exist (or is not
    executable) is an error, never an exec attempt.
- RED because: the package `dockergento/adapters/selfrun` does not exist yet, so the test file does
  not compile. That is the right kind of RED — not a routing or fixture problem.
- Follow the shape of `dockergento/adapters/legacy/runner_test.go` (`shellTree` helper style,
  `t.TempDir()`, exit-code assertions) for consistency with the sibling adapter.

### 1.2 — [x] GREEN: `dockergento/adapters/selfrun/runner.go`

- Implement `Runner` as a `ports.Legacy` (`Run(args []string) (int, error)`, per `ports.go:310-314`).
- Resolution order, fail-safe, in this exact sequence:
  1. `Executable` field, if set (lets a test or a caller pin exactly what runs)
  2. `HM_SELF` environment variable, if set
  3. `$HM_LEGACY_ROOT/bin/run`, **only when `HM_LEGACY_ROOT` is set** — deliberately third and not
     first, so redirecting the shell tree (`HM_LEGACY_ROOT`) and redirecting the binary (`HM_SELF`)
     stay two separate questions; see `design.md` §5 "Why `$HM_LEGACY_ROOT/bin/run` is in that order,
     and why it is third" for the full argument — a suite/test that redirects the tree without
     redirecting the binary must not silently fall through to the real `os.Executable()`.
  4. `os.Executable()` as the last resort.
- A resolved path that is not executable is an error, not an exec attempt (mirrors
  `hm_run_binary`'s `[ ! -x "$binary" ]` refusal in the shell half, kept consistent across both
  layers).
- Stdio inherited exactly as `legacy/runner.go:48-50` does it (`os.Stdin`/`os.Stdout`/`os.Stderr`
  passed straight through) — this is what keeps `set-host`'s `sudo` prompt on the caller's terminal.
- Exit code handling identical to `legacy.Runner.Run` (`legacy/runner.go:53-59`): an `*exec.ExitError`
  becomes its exit code with no error; any other error is returned as an error.
- Confirm 1.1's tests now pass.

### 1.3 — [x] RED: the wiring still points at `legacy.Runner`

- Add `TestTheInstallDoorIsTheBinary` in `dockergento/dockergento_test.go`, on `motor(t)` (the
  existing engine-construction test helper used by the file's other tests, e.g.
  `TestAProjectThatDoesNotUseTheProxy`) — assert `Engine.setup().Legacy` is a `selfrun.Runner`
  (e.g. by asserting on its resolved `Executable`/behaviour, not by touching an unexported field
  from outside the package if that is not possible — use whatever public-surface assertion the
  existing tests in this file already use for asserting a port's concrete adapter).
- RED because: `dockergento.go:601` still reads `Legacy: legacy.Runner{Root: e.options.ShellRoot}`,
  so the assertion fails against the wrong adapter type.

### 1.4 — [x] GREEN: one line at `dockergento.go:601`

- Change `dockergento/dockergento.go:601` from
  `Legacy: legacy.Runner{Root: e.options.ShellRoot},` to the `selfrun.Runner` equivalent (wiring the
  same fail-safe resolution order from 1.2; production wiring naturally resolves through
  `os.Executable()` since neither `Executable` nor `HM_SELF` is set in normal use).
- Leave every other `legacy.Runner` wiring untouched: `dockergento.go:534` (`hosts()`), `:1182`
  (`Shell()`) and `:1380` keep `legacy.Runner` — they mean the shell tree, not the install door.
- Add the `dockergento/adapters/selfrun` import; remove nothing from the existing `legacy` import
  since the other three call sites still use it.
- Confirm 1.3 now passes.

### 1.5 — [x] Characterisation pin: the install steps run in one order through one door

- This one is **not RED** — the step list at `dockergento/app/setup.go:516-530` already exists and is
  not being edited. It is a characterisation pin, written in this commit, because neither
  `go test ./...` nor `tests/run.sh tests/unit` exercises `go_setup_test.sh`, so a silent reorder
  introduced later would land green without it.
- Create `dockergento/app/setup_test.go` with `TestTheInstallStepsRunInOneOrderThroughOneDoor`: a
  recording fake `ports.Legacy` capturing every `Run(args)` call in order; subtests with and without
  a dump, asserting the exact step sequence at `setup.go:516-530` (`start`, `composer install`,
  optionally `mysql -i <dump>` + `mysql -q "DELETE FROM admin_user;"`, `install`, `magento
  setup:upgrade`, `magento deploy:mode:set developer`, `restart`, `ssl <domain>`,
  `set-host <domain> --no-database`).
- Do **not** edit `setup.go:516-530` itself — the list stays byte-for-byte, and the comment at
  `:507-509` stays true word for word.

### 1.6 — [x] RED: nothing watches the real fall-through once the door is the binary

- Add case **"a step the binary does not route reaches the bridge"** to
  `tests/integration/go_setup_test.sh`, reusing the existing `HM_LEGACY_ROOT="$LAB/fake"` recorder
  (`:39-46`) already in that file — no new hook needed:
  ```bash
  : > "$HM_FAKE_LOG"
  "$GO_BINARY" --no-json install
  ```
  and assert the log is exactly `--no-json install`: `run.go` hands `Shell()` the arguments
  untouched, and `bin/run` splits its own globals off itself.
- Use `install`, not `composer install` — `composer` has a router case (`run.go:67-72`) and bridges
  only on macOS via `mirrorsVendor`, so a `composer` assertion would prove nothing on Linux CI.
- RED because: before 1.4's wiring change, `Setup.install`'s recorder in `go_setup_test.sh` *replaces*
  the child entirely (it fakes the whole shell tree), so no assertion anywhere watches the real
  binary hand an unported step to `bin/run` through its own router. Confirm this case passes once
  1.2/1.4 land (the binary's router misses `install`, and `Shell()` runs
  `$HM_LEGACY_ROOT/bin/run install`).

### Verify unit 1

- `go build -o bin/hm ./cmd/hm`
- `go test ./...`
- `./tests/run.sh tests/unit`
- `./tests/run.sh tests/integration/go_setup_test.sh`

### Commit message (unit 1)

```
refactor(setup): the install door is the binary, and one test watches the order
```

---

## Unit 2 — The groundwork, and `set-host`

**Components:** c4 (`binary.sh`) + c5 (`magento_installation.sh`) + c7 (`print_help.sh`) + c8
(`bin/run`) + c9 (`tests/lib/router.sh`) + c10 (`command_safety_test.sh` "a file or a route",
`migration_status_test.sh`, `concurrent_safety_test.sh`) + c12 (`exit_codes_test.sh`) + c1
(`set-host.sh` deletion) ≈ **254 lines**

This is the moment `command_safety_test.sh`'s "and every entry has a file" must relax to "a file
**or** a route" — not before (unit 1 has no deletions yet) and not later (unit 3/4 would then find
orphans this test cannot yet tolerate).

### 2.1 — [x] RED: `hm_run_binary` does not exist, and its refusal is untested

- Add to `tests/unit/exit_codes_test.sh`, after its existing `source "$HELPERS_DIR/exit_codes.sh"`:
  **"hm_run_binary refuses when the binary is not installed"** — `source
  "$HELPERS_DIR/binary.sh"`, point `COMMAND_BIN_DIR` at an empty `mktemp -d`, call
  `hm_run_binary <some-command>`, and assert exit `1` (`$HM_EXIT_ERROR`); in JSON mode
  `.error.type` is `binary_missing` and the message names the command that needed it.
- RED because: `console/helpers/binary.sh` does not exist yet, so `source` fails.

### 2.2 — [x] GREEN: `console/helpers/binary.sh`

- Create with exactly `hm_run_binary`:
  ```bash
  hm_run_binary() {
      local binary="$COMMAND_BIN_DIR/bin/hm"

      if [ ! -x "$binary" ]; then
          hm_fail "$HM_EXIT_ERROR" "binary_missing" \
              "$1 needs the ${COMMAND_BIN_NAME} binary, and it is not installed here" \
              "Reinstall ${COMMAND_BIN_NAME}, or build it with: cd $COMMAND_BIN_DIR && go build -o bin/hm ./cmd/hm"
      fi

      "$binary" "$@"
  }
  ```
  — the refusal text mirrors `worktree.sh:20-24` word for word, with `$1` (the command name, also
  the binary's own first argument) in place of the stub's hardcoded name.
- Guard its own dependency: `declare -F hm_fail >/dev/null 2>&1 || source "$HELPERS_DIR"/exit_codes.sh`
  — callers like `debug-on.sh` and `magento_installation.sh` do not source `exit_codes.sh` today.
- It runs, it does not `exec`: `composer.sh` has work left to do after the call (lines `:52-63`);
  `exec` would end the script at the first copy.
- Confirm 2.1 now passes.
- Do **not** touch the three existing delegation stubs (`worktree.sh`, `docker-compose.sh`,
  `docker-stop-all.sh`) — they `exec` and this helper does not; folding them on is an explicitly
  deferred open question in `design.md`.

### 2.3 — [x] GREEN: `magento_installation.sh:32` points at the binary

- Add `source "$HELPERS_DIR"/binary.sh` beside `console/tasks/magento_installation.sh`'s other
  sources.
- Change line 32 from `"$COMMANDS_DIR"/set-host.sh "$DOMAIN" --no-database` to
  `hm_run_binary set-host "$DOMAIN" --no-database`.
- `"$DOMAIN"` stays quoted, unchanged.

### 2.4 — [x] RED: `bin/run` still says a retired command was never found

- Add the `command_moved_to_binary` cases to `tests/integration/cli_contract_test.sh`, beside the
  existing `command_not_found` cases (`:24-45`): `run version` exits 2 with `.error.type` =
  `command_moved_to_binary`; and `--no-json version` names `hm version` in the hint.
- RED because: `bin/run`'s `check_command_dir` (`bin/run:72-82`) has only two branches today (native
  file, then custom file, else `command_not_found`) — `version.sh` still exists as a file at this
  point in the unit, so this RED specifically needs the test to run **after** 2.6's deletion, or
  against a name that has no file yet. Sequence this task so the assertion is written now (RED) and
  turns GREEN once 2.5 (branch) and 2.6 (`set-host.sh` deletion) both land in this same commit —
  `version` itself is not deleted until unit 4, so use `set-host` as the RED/GREEN pair here instead:
  `run set-host` (with no file after 2.6) exits 2 with `command_moved_to_binary`, hinting
  `hm set-host`. Add the `version` case in unit 4, when `version.sh` is actually gone.

### 2.5 — [x] GREEN: `bin/run`'s third branch

- In `bin/run`'s `check_command_dir` (`:72-82`), add a third branch before the existing
  `command_not_found` refusal:
  ```bash
  local name="${command_name%.sh}"

  if [ "$(jq -r --arg n "$name" 'has($n)' "$DATA_DIR/command_descriptions.json")" == "true" ]; then
      hm_fail "$HM_EXIT_USAGE" "command_moved_to_binary" \
          "'$name' is part of the $COMMAND_BIN_NAME binary now" \
          "$COMMAND_BIN_NAME $name"
  fi
  ```
- Exit code stays `$HM_EXIT_USAGE` (2); position stays before `docker_env_variables`, so nothing asks
  Docker anything first.
- `Command not found: <name>` survives byte-for-byte for a name that is neither a file nor declared.
- Confirm 2.4's `set-host` case now passes.

### 2.6 — [x] GREEN: `set-host.sh` deleted, in the same commit as its only caller's fix

- Delete `console/commands/set-host.sh` (109 lines) — now that 2.3 no longer points at it and 2.5
  gives `bin/run` something to say about it.
- Confirm 2.4's `set-host` RED case is now GREEN (both the `bin/run` branch and the missing file are
  in place).

### 2.7 — [x] RED: the help still asks the filesystem, not the declared names

- No new assertion is needed beyond the existing `help_test.sh` (kept unchanged, per `design.md`'s
  proof strategy) and a manual before/after capture: `bin/run --help > /tmp/help.before` (captured on
  the developer's own platform, before this unit's first commit lands).
- The RED here is behavioural, not a new test: with `set-host` gone from the filesystem (2.6),
  `print_help.sh`'s current glob-only `names` loop (`:42-46`) would silently drop it from `hm --help`
  unless the union is added. `help_test.sh:90-99`'s existing "declared, no file" case
  (`selftestungrouped.sh`) is the mechanical pin that already exercises this shape.

### 2.8 — [x] GREEN: `print_help.sh` unions files with declared-no-file names

- Modify `console/helpers/print_help.sh:41-46`: keep the glob, add a second pass reading
  `command_descriptions.json`'s keys (excluding those starting with `_`), adding `<name>.sh` to
  `names` only when `$command_path/<name>.sh` does not already exist (union by construction — never
  `sort -u`, which can silently drop a name that merely *compares* equal under a locale that ignores
  punctuation).
- Names carry `.sh` all the way through the collection, because that is the string `sort(1)` orders
  on; sort once with plain `sort` (not `jq`'s code-point sort) so the ambient locale's `strcoll`
  matches the glob's own order exactly (`dbeaver.sh` before `db.sh`, `mysqldump.sh` before
  `mysql.sh`, on glibc).
- Update the table `jq` by one token: `map(select(length > 0))` becomes
  `map(select(length > 0) | rtrimstr(".sh"))`.
- Only the native pass (`print_commands_info` without `"custom"`) gains the union; the custom pass
  keeps globbing only — a project's own commands are never in `command_descriptions.json`.
- Verify: `bin/run --help` byte-identical to `/tmp/help.before` from 2.7 (`diff`), and
  `./tests/run.sh tests/unit/help_test.sh` green.

### 2.9 — [x] RED: "and every entry has a file" is now false, on purpose, and needs to become the union rule

- `tests/unit/command_safety_test.sh:52-57` ("and every entry has a file") fails as soon as 2.6 lands
  — `set-host` is declared with no file. This is the expected, checklist-tracked RED for this unit
  (not a bug to route around): the assertion's own text is what has to change.

### 2.10 — [x] GREEN: shared `hm_routed_commands`, and "a file or a route"

- Create `tests/lib/router.sh` with `hm_routed_commands`, extracted from the `routed()` function
  already in `tests/unit/migration_status_test.sh:78-80` (the `sed` over
  `internal/cli/run.go`'s `case "..."` lines, excluding `_`-prefixed keys, sorted).
- Source `tests/lib/router.sh` from both `command_safety_test.sh` and `migration_status_test.sh`;
  remove the now-duplicated inline `routed()` from `migration_status_test.sh` (call
  `hm_routed_commands` in its place at every existing call site, `:79-105`).
- Change `command_safety_test.sh:52-57` ("and every entry has a file") to "and every entry has a
  file **or** a route": an entry is fine if it has a `.sh` file **or** appears in
  `hm_routed_commands`.
- Confirm this test is green again with `set-host` declared, routed, and file-less.
- Confirm `migration_status_test.sh`'s "the partial exception is still routed and still tabled
  shell" (`:97-105`, over `db`/`proxy`) still passes unchanged — this is the proof from
  `design.md` §6 that an empty/broken extractor cannot pass quietly (it would make `db`/`proxy`
  vanish from `routed()` and fail that case loudly).

### 2.11 — [x] GREEN: `concurrent_safety_test.sh`'s hosts-file cases retired with a pointer

- Remove both cases at `tests/integration/concurrent_safety_test.sh:105-109` ("the entries this tool
  adds carry a marker" and "and there is a way to remove its own"), which read
  `console/commands/set-host.sh`'s source directly. Replace with a comment pointing at
  `dockergento/core/hosts.go:19-24`, pinned by `TestOnlyWhatThisToolAddedIsRemoved` and
  `TestTheEntryIsItsOwnLine` in `core/hosts_test.go` (already existing coverage — no new Go test
  needed here).
- Leave `:111-115` ("the collection command reports them without touching the file") untouched — it
  tests the tool's behaviour, not a file's contents.

### Verify unit 2

- `go build -o bin/hm ./cmd/hm`
- `go test ./...`
- `./tests/run.sh tests/unit` (includes `command_safety_test.sh`, `migration_status_test.sh`,
  `exit_codes_test.sh`, `help_test.sh`)
- `./tests/run.sh tests/integration/concurrent_safety_test.sh`
- `./tests/run.sh tests/integration/cli_contract_test.sh`
- `rg -l "set-host.sh" console/ bin/` finds nothing

### Commit message (unit 2)

```
refactor(console): set-host has one implementation, and bin/run says where it went
```

---

## Unit 3 — `copy-to-container` and `copy-from-container`

**Components:** c6 (`composer.sh` + `debug-on.sh` call sites) + c2 (both deletions) ≈ **117 lines**

Three by-path callers, two files. All three callers stop pointing at a file in this commit, and both
files go in the same commit.

### 3.1 — [x] GREEN: the three call sites, through the binary

- `console/commands/composer.sh`: add `source "$HELPERS_DIR"/binary.sh` beside its existing sources.
  - `:39` — `"$COMMANDS_DIR"/copy-to-container.sh "vendor"` → `hm_run_binary copy-to-container
    "vendor"`.
  - `:51` — `"$COMMANDS_DIR"/copy-to-container.sh $default_files_magento` → `hm_run_binary
    copy-to-container $default_files_magento`. **`$default_files_magento` stays unquoted,
    deliberately** — it is `jq -r 'keys | join(" ")'` over `data/default_files_magento.json`
    (`composer.sh:49`), one string of 31 names the shell must split into 31 arguments; quoting it
    would hand the binary one argument with spaces and `copy-to-container` would look for a file by
    that literal name — the same bug class the `test-unit` parity case (unit 4) exists to catch.
- `console/commands/debug-on.sh:22` — add `source "$HELPERS_DIR"/binary.sh`; change
  `"$COMMANDS_DIR"/copy-from-container.sh generated` → `hm_run_binary copy-from-container generated`.
- No new RED here: `hm_run_binary` and both Go-side commands are already proven (unit 2's helper,
  and the twelve are already routed/tested in Go per the proposal — this task only rewires the
  by-path callers). Manual confirmation only, since both call sites are macOS-only paths
  (`composer.sh`'s `copy_vendor_to_container`/`sync_all_from_container_to_host` run under
  `"$MACHINE" == "mac"`, `debug-on.sh:20` is the same guard) and Linux's `composer install` is
  answered entirely in Go (`mirrorsVendor`, `internal/cli/php.go:99-101`), so `composer.sh` is not
  even reached there.

### 3.2 — [x] GREEN: both files deleted

- Delete `console/commands/copy-to-container.sh` (64 lines) and
  `console/commands/copy-from-container.sh` (46 lines) — 110 lines total, in the same commit as
  3.1's rewired callers.
- `rg -l "copy-to-container.sh|copy-from-container.sh" console/ bin/` finds nothing.

### Verify unit 3

- `go build -o bin/hm ./cmd/hm`
- `go test ./...`
- `./tests/run.sh tests/unit`
- Manual, safe-by-construction verification named in `proposal.md`'s Success Criteria (only if the
  developer already has a project up, and only against one they can rebuild): `hm composer install`
  on macOS exercises `copy-to-container`; `hm debug-on` exercises `copy-from-container`.

### Commit message (unit 3)

```
refactor(console): composer and debug-on copy through the binary
```

---

## Unit 4 — The last nine twins go, the parity tests stand on their own, and the drift test

**Components:** c3 (nine deletions) + c11 (routed→no-file direction) + c16 (`go_wrappers_test.sh`
rewrite) + c17 (`MIGRATION.md`) ≈ **340 lines**

`varnish-off.sh:18` runs `"$COMMANDS_DIR"/purge.sh` — the fifth by-path caller, needing no fix
because both files are among the twelve; it only constrains the order: `purge` and `varnish-off`
must die in the same commit (they are both here, the last one).

### 4.1 — [x] RED: the `bin/run` message for a command whose file is actually gone

- Add the deferred `version` case from task 2.4 to `tests/integration/cli_contract_test.sh`: `run
  version` exits 2 with `.error.type` = `command_moved_to_binary`; `--no-json version` names
  `hm version` in the hint.
- RED because: `console/commands/version.sh` still exists at this point in the unit (it is deleted
  in 4.2), so `bin/run`'s file check would still find it and never reach the third branch added in
  unit 2 — a check against `version` written before 4.2 fails for the right reason (the file has not
  gone yet), not because the branch is missing (it already landed in unit 2).

### 4.2 — [x] GREEN: the nine remaining deletions, `purge` and `varnish-off` together

- Delete, in one commit:
  - `console/commands/version.sh` (85 lines)
  - `console/commands/varnish-off.sh` (21 lines)
  - `console/commands/varnish-on.sh` (18 lines)
  - `console/commands/mysqldump.sh` (13 lines)
  - `console/commands/purge.sh` (11 lines)
  - `console/commands/n98-magerun.sh` (10 lines)
  - `console/commands/test-unit.sh` (10 lines)
  - `console/commands/npm.sh` (4 lines)
  - `console/commands/test-integration.sh` (4 lines)
  - 176 lines total. `purge` and `varnish-off` must land together because `varnish-off.sh:18` calls
    `purge.sh` by path — with both gone there is no dangling caller.
- Confirm 4.1's `version` case is now GREEN.
- `rg -l "version.sh|varnish-off.sh|varnish-on.sh|mysqldump.sh|purge.sh|n98-magerun.sh|test-unit.sh|npm.sh|test-integration.sh" console/ bin/` finds nothing (excluding the deleted files' own former paths, i.e. run after `git rm`).

### 4.3 — [x] RED: "what the binary owns outright has no file to drift from" does not exist yet

- Add the new case to `tests/unit/command_safety_test.sh` (the reverse direction §6 of `design.md`
  notes is untested: `:44-50` says file→entry, the old `:52-57` said entry→file, neither said
  routed→no-file):
  ```bash
  RETIRED="copy-to-container copy-from-container varnish-on varnish-off purge npm
           n98-magerun test-unit test-integration mysqldump version set-host"

  drift=""
  routed="$(hm_routed_commands)"
  for name in $RETIRED; do
      [ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"
      grep -qx "$name" <<< "$routed" || drift="$drift ${name}(not-routed)"
  done
  assert_empty "$drift"
  ```
- Scoped to these twelve **by name**, not "every routed command": twenty other routed commands
  legitimately still have a file (`magento`, `start`, `mysql`, the partials `db`/`proxy`, the three
  delegating stubs), and a blanket rule would be false today and unfixable.
- This lands here, in the last commit, because it must run against all twelve with no files — before
  unit 4 lands, eleven of the twelve still have files and this case would legitimately fail.
- Confirm GREEN once 4.2's deletions are in place (uses `hm_routed_commands` from unit 2's
  `tests/lib/router.sh`).

### 4.4 — [x] RED: the six `go_wrappers_test.sh` parity cases still compare shell to shell

- Before rewriting, confirm each of the six comparison-based cases would still pass today as a
  vacuous `assert_equals "$X" "$X"` once `$SHELL_CLI` still exists — this is the defect the rewrite
  fixes, so note it rather than re-derive it: `:95-96` (npm), `:98-101` (n98-magerun), `:137-144`
  (test-unit), `:179-181` (mysqldump), `:190-192` (version `--json`), `:201-203` (version text).
  These are the RED-by-absence cases: each has to fail when the binary's behaviour is broken on
  purpose, and today none of them can, because they only prove the two CLIs agree with each other,
  never that either is right.

### 4.5 — [x] GREEN: `npm` and `n98-magerun` — recorder makes the container's own failure the assertion

- Replace `:95-96` (npm): a recorder `npm` at `/usr/local/bin/npm` in the container writes `pwd` and
  its argv in the unambiguous `$#` / `[bracketed]` form, then exits 42. Assert the recorded directory
  is `/var/www/html`, the argv is exactly `1` / `[--version]`, and `hm npm --version` exits **42** —
  the container's status is the tool's status.
- Replace `:98-101` (n98-magerun): `wrappers.go:55` builds
  `[]string{"bash", "-c", "n98-magerun " + strings.Join(args, " ")}` — three elements, the third
  already joined. A recorder that logs `"$@"` or `"$*"` alone would render that identically whether
  the arguments were passed separately or already joined, so it must count arguments explicitly. A
  recorder at `/usr/local/bin/bash`:
  ```sh
  #!/bin/sh
  printf '%s\n' "$#"        >  /tmp/argv
  printf '[%s]\n' "$@"      >> /tmp/argv
  exit 43
  ```
  Assert `/tmp/argv` is exactly:
  ```
  2
  [-c]
  [n98-magerun --version]
  ```
  and `hm n98-magerun --version` exits **43**. (`$#` is 2, not 3: `$0` is the recorder's own name.
  Had the arguments been passed separately the file would read `3` / `[-c]` / `[n98-magerun]` /
  `[--version]`, so this now fails on the regression it exists for.) The `npm` recorder reuses the
  same two `printf` lines — one shape, written once.
- Remove `$SHELL_CLI` usage from these two cases.

### 4.6 — [x] GREEN: `test-unit` — exact equality of the entire recorded invocation

- Replace `:137-144`: assert exact equality of the whole recorded invocation —
  `/var/www/html` and `--config ./dev/tests/unit/phpunit.xml.dist`, nothing appended. `echo "$@"`
  cannot distinguish one argument with embedded spaces from three separate ones — what tells them
  apart is that a single space-containing argument never invokes the recorder at all, leaving the
  log empty. Keep the existing recorder fixture (`vendor/bin/phpunit` writing `pwd` and `"$@"`);
  remove the `$SHELL_CLI` comparison, assert against the binary's own recorded output directly.

### 4.7 — [x] GREEN: `mysqldump` — completeness against the database itself

- Replace `:179-181`: the fixture inserts three rows instead of one; the dump produced by the binary
  is restored into a second database (`CREATE DATABASE verificacion`, load, `SELECT COUNT(*)`), and
  the count must be 3. This is completeness proven against the database itself, stronger than
  `grep -c 'INSERT INTO'` against another dump — extended inserts can make both `grep -c` counts 1
  regardless of what actually happened.
- Remove the `$SHELL_CLI` comparison case.

### 4.8 — [x] GREEN: `version --json` and its text rendering — a git oracle independent of both CLIs

- Replace `:190-192` and `:201-203`. Preflight first (a check that skips is a check that passes):
  ```bash
  COMMIT=$(git -C "$COMMAND_BIN_DIR" rev-parse --short HEAD 2>/dev/null)
  DESCRIBED=$(git -C "$COMMAND_BIN_DIR" describe --tags 2>/dev/null)

  test_case "the installation directory is a checkout, or nothing below is an oracle"
  assert_equals "checkout" "$([ -n "$COMMIT" ] && echo checkout || echo "not a checkout")"
  ```
  An empty `COMMIT` **fails** here rather than skipping what follows.
- Field by field against `gitvcs.Git.Installed` (`gitvcs/vcs.go:173-204`):
  - `.data.commit` — `$COMMIT` exactly (`rev-parse --short HEAD`, not a prefix of the long hash)
  - `.data.path` — `$COMMAND_BIN_DIR`
  - `.data.version` — `$DESCRIBED`, or exactly `unknown` when `describe` prints nothing
    (`vcs.go:189-191`) — both branches assert, neither skips
  - `.data.tag` — `git -C "$COMMAND_BIN_DIR" describe --tags --abbrev=0`
  - `.data.dirty` — `true` iff `git -C "$COMMAND_BIN_DIR" status --porcelain --untracked-files=no`
    prints anything
  - `.data.commits_ahead` — the `<n>` of `<tag>-<n>-g<hash>` in `$DESCRIBED`, else `0`
    (`core/installation.go:35-53`)
- `.data.binary` is not null (`:198`) stays as it is — no git oracle, it is this binary's own build
  stamp.
- Text rendering checked against the same git oracle, never against the JSON (a cross-format
  self-comparison cannot catch a value wrong in both renderings): `commit` line is `$COMMIT`,
  `installed` line is `$COMMAND_BIN_DIR`, header carries `$DESCRIBED`. The `branch` line is not
  asserted — on a detached checkout it is not printed at all (`version.go:41-45`).
- Remove the two `$SHELL_CLI`-based comparison cases and every remaining `$SHELL_CLI` reference in
  the file — the eleven cases that already run only the binary (`:75-84`, `:113-130`, `:163-177`,
  `:198-209`) are untouched otherwise.

### 4.9 — [x] Update `MIGRATION.md`

- Record the third step (shell deletion) as executed for these twelve, in Spanish, in the voice of
  the rest of the file. The command table (`§Los comandos`) stays as it is — all twelve already read
  `go` — this is prose only: note near §7's wrapper/version/copy/set-host bullets, or in
  "Cómo se porta un comando", that the shell twin and its bash test are now gone for these twelve, in
  the same deletion-discipline language already stated at lines 62-66.
- Confirm `tests/unit/migration_status_test.sh` still passes unchanged (it already asserts the table
  agrees with the Go tree in both directions; no table row changes here).

### Verify unit 4

- `go build -o bin/hm ./cmd/hm`
- `go test ./...`
- `./tests/run.sh tests/unit` (including `command_safety_test.sh`, `migration_status_test.sh`)
- `./tests/run.sh tests/integration/go_wrappers_test.sh`
- `./tests/run.sh tests/integration/cli_contract_test.sh`
- Confirm every one of the six rewritten `go_wrappers_test.sh` cases still fails when the binary's
  behaviour is broken on purpose (manual sanity check, per `proposal.md`'s Success Criteria).
- `rg -l "console/commands/(set-host|copy-to-container|copy-from-container|version|varnish-off|varnish-on|mysqldump|purge|n98-magerun|test-unit|npm|test-integration)\.sh" .` finds nothing outside `MIGRATION.md` prose and `openspec/`.

### Commit message (unit 4)

```
refactor(console): the last nine twins go, and the parity tests stand on their own
```

---

## Final verification (after all four commits)

- `go test ./...` — expect the pre-existing 289 Go tests across 23 packages, plus the new
  `selfrun` tests (1.1), `TestTheInstallDoorIsTheBinary` (1.3) and
  `TestTheInstallStepsRunInOneOrderThroughOneDoor` (1.5), all green.
- `./tests/run.sh unit` — expect the pre-existing 612 shell unit assertions, adjusted by:
  `command_safety_test.sh`'s relaxed rule (2.10) and new drift case (4.3), the new
  `hm_run_binary` refusal case (2.1), and `tests/lib/router.sh`'s extraction (2.10) — all green.
- Named integration suites, all green:
  - `tests/integration/go_setup_test.sh` (1.6)
  - `tests/integration/cli_contract_test.sh` (2.4, 4.1)
  - `tests/integration/concurrent_safety_test.sh` (2.11)
  - `tests/integration/go_wrappers_test.sh` (4.4-4.8)
- `gofmt -l ./cmd ./internal ./dockergento` empty; `go vet ./...` clean.
- `bin/run --help` byte-identical to the pre-change capture (`/tmp/help.before` from 2.7), diffed
  after the last commit.
- `rg -l "copy-to-container.sh|copy-from-container.sh|set-host.sh" console/ bin/` finds nothing.
- Manual, safe-by-construction: `hm --help` compared old vs. new build (reads nothing); and — only
  if the developer already has a rebuildable project up — `hm composer install` on macOS and
  `hm debug-on`.

## Size exception (recorded, not reopened)

Cumulative diff across the four commits is **881 lines** against the repository's 400-line
per-unit budget, measured with `git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'`. Each
of the four commits is individually under budget (170 / 254 / 117 / 340), which is what makes the
four-commit split legal without a `size:exception` on any single commit — the exception recorded
here is only that the **change as a whole**, summed across its four commits, exceeds what a single
unit could carry, and the delivery decision (four commits, taken before `sdd-apply`) is what
resolves that rather than deferring it, per `design.md`'s closing arithmetic.
