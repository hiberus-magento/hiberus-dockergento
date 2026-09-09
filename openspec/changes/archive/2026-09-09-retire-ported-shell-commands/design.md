# Design

## The third step is a deletion, and everything here exists to make it legal

Twelve files go. Nothing replaces them. What follows is the five things that hold them in place —
four callers reaching them by path, a help that lists files, a `bin/run` that would say the command
does not exist, a `setup` whose install door is the shell CLI, and three tests that read the shell
side — and what each becomes so that the deletion changes no behaviour a person can see.

The order is not decorative. Every caller has to stop pointing at a file **before** the file goes,
and both have to be in the same commit, or the repository is broken between two commits that each
pass their own tests.

## 1 · One helper, and why it is not the stub

`console/helpers/binary.sh` is new and defines one function:

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

The refusal is `worktree.sh:20-24` word for word, with `$1` — the command name, which is also the
binary's first argument — where the stub hardcodes its own. So `copy-to-container` refuses in the
same sentence `docker-compose` refuses in, and there is one sentence rather than four.

| Decision | Choice | Rejected | Why |
|---|---|---|---|
| It runs, it does not `exec` | `"$binary" "$@"` | `exec "$binary"` like the stubs | `composer.sh` has work left to do afterwards (`:52-63`); `exec` would end the script at the first copy |
| It runs, it does not answer a path | `hm_run_binary <command> [args]` | `binary=$(hm_binary)` | `hm_fail` ends with `exit`, and inside `$( )` that exits the subshell only: the caller would carry on with an empty path |
| It sources its own dependency | `declare -F hm_fail` guard, then `source "$HELPERS_DIR"/exit_codes.sh` | assume the caller sourced it | `debug-on.sh` and `magento_installation.sh` do not source it today, and the helper's whole value is that the caller writes one line |
| The three existing stubs are left alone | untouched | refactor them onto the helper now | they `exec` and this does not, so sharing needs an exec mode; three files this change has no other reason to open, for no behaviour to prove. When a fourth stub appears, `hm_run_binary` gains an exec sibling then |

Under `set -euo pipefail` a non-zero from `hm_run_binary` still ends the caller, exactly as a
non-zero from `"$COMMANDS_DIR"/copy-to-container.sh` did.

## 2 · The four call sites

Each caller gains `source "$HELPERS_DIR"/binary.sh` beside its other sources, and one line changes:

| Where | Today | After |
|---|---|---|
| `console/commands/composer.sh:39` | `"$COMMANDS_DIR"/copy-to-container.sh "vendor"` | `hm_run_binary copy-to-container "vendor"` |
| `console/commands/composer.sh:51` | `"$COMMANDS_DIR"/copy-to-container.sh $default_files_magento` | `hm_run_binary copy-to-container $default_files_magento` |
| `console/commands/debug-on.sh:22` | `"$COMMANDS_DIR"/copy-from-container.sh generated` | `hm_run_binary copy-from-container generated` |
| `console/tasks/magento_installation.sh:32` | `"$COMMANDS_DIR"/set-host.sh "$DOMAIN" --no-database` | `hm_run_binary set-host "$DOMAIN" --no-database` |

`$default_files_magento` **stays unquoted, deliberately**. It is
`jq -r 'keys \| join(" ")'` over `data/default_files_magento.json` (`composer.sh:49`): one string of
31 names that the shell is meant to split into 31 arguments. Quoting it would hand the binary a
single argument with spaces in it and `copy-to-container` would look for a file by that name — the
same bug class the `test-unit` parity case exists for. Unquoted expansion is also subject to
pathname expansion; none of the 31 names contains `*`, `?` or `[`, so nothing globs, and that was
already true before this change. `"$DOMAIN"` and `"vendor"` stay quoted for the same reason they are
quoted today.

**mac and linux.** Two of the four are macOS-only paths and one is dormant on Linux:
`copy_vendor_to_container` and `sync_all_from_container_to_host` run only under
`"$MACHINE" == "mac"` (`composer.sh:91`), and `debug-on.sh:20` is the same guard. On Linux the
binary answers `composer install` itself — `mirrorsVendor` is `runtime.GOOS == "darwin" &&
writesDependencies(args)` (`internal/cli/php.go:99-101`) — so `composer.sh` is not even reached.
`set-host` is both platforms.

## 3 · The help stops asking the filesystem

`print_help.sh:41-46` keeps the glob and gains a second source. The names carry `.sh` all the way
now, because that is the string the order is defined on:

```bash
    for script in "$command_path"/*.sh; do
        [ -f "$script" ] || continue
        names+="${script##*/}"$'\n'
    done

    if ! $custom; then
        while IFS= read -r declared; do
            [ -z "$declared" ] && continue
            [ -f "$command_path/$declared.sh" ] && continue
            names+="$declared.sh"$'\n'
        done <<< "$(printf '%s' "$file_content" | jq -r 'keys_unsorted[] | select(startswith("_") | not)')"

        names=$(printf '%s' "$names" | sort)
    fi
```

and the table `jq` changes by one token: `map(select(length > 0))` becomes
`map(select(length > 0) | rtrimstr(".sh"))`.

| Decision | Choice | Rejected | Why |
|---|---|---|---|
| Union, not replacement | files ∪ declared-with-no-file | declared only | `help_test.sh:90-99` writes `selftestungrouped.sh` with no entry in the data file and requires it to appear. A project's own commands are not in that file either, which is why the custom pass still only globs |
| No duplicates by construction | a declared name is added **only** when it has no file | `sort -u` | under a collation that ignores punctuation, `-u` drops lines that merely *compare* equal, which would silently lose a command |
| `_groups`, `_examples`, `_global` excluded | `select(startswith("_") \| not)` | `keys[]` | they are keys and they are not commands, which is what the underscore already means here (`run.go:39-44`) |
| Sorted by `sort`, not by `jq` | one `sort`, in the ambient locale | `sort_by(.name + ".sh")` inside the existing `jq` | bash's glob and `sort(1)` both order through `strcoll` in the same locale, so `sort` reproduces the glob's order exactly. `jq` sorts by code point, and the two disagree here: on glibc, `dbeaver.sh` sorts **before** `db.sh` and `mysqldump.sh` before `mysql.sh`, because `.` is ignored at the primary level. That is a visible change to the help on Linux, and the success criterion is a byte-identical help |
| A second `jq`, accepted | 2 per native pass | keep exactly 1 | the 5.7s → 0.4s the comment at `:52-53` guards was 65 `jq` processes, one per command, becoming one. This adds one constant call, not one per command; the table `jq` keeps its shape |

Proof it did not change: `bin/run --help > /tmp/help.before` captured before the first commit and
`diff`ed after the last, on the developer's own platform, plus `help_test.sh` untouched.

## 4 · What `bin/run` says about a command it no longer has

`check_command_dir` (`bin/run:72-82`) gets a third branch before the refusal:

```bash
    local name="${command_name%.sh}"

    if [ "$(jq -r --arg n "$name" 'has($n)' "$DATA_DIR/command_descriptions.json")" == "true" ]; then
        hm_fail "$HM_EXIT_USAGE" "command_moved_to_binary" \
            "'$name' is part of the $COMMAND_BIN_NAME binary now" \
            "$COMMAND_BIN_NAME $name"
    fi
```

Exit code stays `$HM_EXIT_USAGE` (2) and the position stays: before `docker_env_variables`, so
nothing asks Docker anything first. `Command not found: <name>` survives byte-for-byte for a name
that is neither a file nor declared.

| How `bin/run` knows | Cost | Verdict |
|---|---|---|
| Declared in `command_descriptions.json` and no file | one `jq`, on the miss path only | **chosen** |
| Ask the binary for its routes | a process on a path that exists because the binary may be missing | rejected |
| A static list in `bin/run` | free, and stale the next time a command is ported | rejected |

The predicate is only honest if "declared, no file" really does mean the binary owns it — and that
is exactly what §6 makes `command_safety_test.sh` assert. One rule, two readers: the message and the
test that keeps it true.

This path is reached only by someone running the shell entry point directly. `hm <command>` is the
binary, which routes all twelve before the bridge. `bin/run --version` and `hm --version` are
untouched: they are `process_hm_options.sh:74`, not `console/commands/version.sh`.

## 5 · The setup door becomes the binary

`app/setup.go:516-530` is not edited. The list stays byte-for-byte, the comment at `:507-509` stays
true, and the only thing that changes is what `s.Legacy` is:

```
hm setup ──→ app.Setup.install ──→ ports.Legacy ──→ selfrun.Runner ──→ hm start
                                                                    ──→ hm composer install ──→ bin/run ──→ composer.sh ──→ hm copy-to-container   (mac)
                                                                    ──→ hm install ─────────→ bin/run ──→ install.sh
                                                                    ──→ hm set-host <domain> --no-database
```

New adapter `dockergento/adapters/selfrun`, a `Runner` implementing `ports.Legacy` by running this
executable again with the step's arguments and answering with its exit code. Stdio is inherited
exactly as `legacy/runner.go:48-50` does it, which is what keeps `set-host`'s `sudo` prompt on the
caller's terminal.

| | |
|---|---|
| Where it lives | its own adapter package, beside `legacy` and `composecli`, not a mode flag on `legacy.Runner` — that type's identity is "runs `bin/run`", and its six callers should not have to read a flag to know what it does |
| Wired | `dockergento.go:601` only. `:534` (`hosts`), `:1182` (`Shell`) and `:1380` keep `legacy.Runner`: they mean the shell tree |
| Which executable | `Executable` field → `HM_SELF` → **`$HM_LEGACY_ROOT/bin/run` when that variable is set** → `os.Executable()`. A path that is not executable is an error, not an exec |
| Still shell | `install` and `ssl` have no case in `run.go` at all, so the child binary falls through to `bin/run` one hop later — its normal behaviour, and what `Shell()` is for. `composer` **does** have a case (`run.go:67-72`) and bridges only when `mirrorsVendor` is true, so the third hop exists on macOS and not on Linux |
| The environment | untouched. `legacy.Runner` strips `HM_REGISTERED*` because the shell half cannot read the registry; the binary reads it directly, and the one hop that still hands it over (`dockergento.go:1182`) strips and re-adds on the way |
| No recursion | `setup` is not one of its own steps, and every hop is a different command. Deepest chain is three processes, on macOS: `hm setup` → `hm composer install` → `bin/run composer` → `hm copy-to-container`, which is routed in Go and stops |
| mac and linux | identical call. The depth differs only because of `mirrorsVendor` (§2): on Linux `composer install` is answered in Go and there is no third hop |

**Why `$HM_LEGACY_ROOT/bin/run` is in that order, and why it is third.** The order has to fail
**safe**, and `Executable → HM_SELF → os.Executable()` fails open: a suite that fakes its whole
world and forgets one hook runs a real `hm start`, a real `composer install` and a real `mysql -i`
against a project made of two placeholder files. `go_setup_test.sh` is exactly that suite — it fakes
the shell tree with `HM_LEGACY_ROOT="$LAB/fake"` and a `bin/run` that appends `"$*"` to a log
(`:39-46`), and asserts the step order from that log (`:132-153`). Consulting `HM_LEGACY_ROOT`
means *whoever has already said "the tree is not the real one" has already said it*: the suite gets
the recorder, `"$*"` is identical, every existing assertion keeps passing, and it needs **zero** new
lines rather than one it can forget. The same holds for Go tests — the pin at
`internal/cli/fake_engine_test.go:205-208` points `HM_LEGACY_ROOT` at a fresh temporary directory
precisely so that `<root>/bin/run` fails with ENOENT; that same empty directory now also makes
`Setup.install` fail to exec instead of running the test binary again. One pin, both hazards.

It is third, after `HM_SELF`, because redirecting the tree and redirecting the binary are still two
questions, and someone who wants only the first says so with `HM_SELF`. What it costs is visible and
loud: with `HM_LEGACY_ROOT` pointed at a **real** other checkout, `setup`'s `set-host` step reaches
that checkout's `bin/run`, which after §4 refuses with `command_moved_to_binary` and exit 2. That is
the correct answer to "run a retired command through a redirected tree", and it is a message, not a
silent wrong environment. A required `Executable` field with no fallback was rejected for the
opposite reason: production wiring would always resolve `os.Executable()`, so the faking suite would
still get the real binary — it makes the *constructor* strict and leaves the *hazard* untouched.

### What RED means here

| Layer | RED | GREEN | Test |
|---|---|---|---|
| Adapter | the package does not exist, so the suite does not compile | `Runner` runs what it was given, with the arguments it was given, and answers with its code | `dockergento/adapters/selfrun/runner_test.go`: `TestItRunsTheExecutableItWasGiven` (a recorder script in `t.TempDir()` that writes its argv and exits 7), `TestAnExecutableThatIsNotThereIsAnError` |
| Wiring | `setup().Legacy` is a `legacy.Runner` today, so the assertion fails | one line at `:601` | `TestTheInstallDoorIsTheBinary` in `dockergento/dockergento_test.go`, on `motor(t)` |
| Order | *not* RED, and said plainly: the list already exists. It is a characterisation pin, written in the same commit, because CI runs `go test ./...` and `tests/run.sh tests/unit` and neither includes `go_setup_test.sh` — a silent reorder would land green | — | `TestTheInstallStepsRunInOneOrderThroughOneDoor` in `dockergento/app/setup_test.go` (new), a recording `ports.Legacy`, subtests with and without a dump |
| Bridge | nothing anywhere exercises the fall-through: once the door is the binary, the recorder in `go_setup_test.sh` *replaces* the child, so no assertion ever watches the real binary hand an unported step to `bin/run` | the router misses, `Shell()` runs `$HM_LEGACY_ROOT/bin/run` | new case **"a step the binary does not route reaches the bridge"** in `tests/integration/go_setup_test.sh`: `: > "$HM_FAKE_LOG"; "$GO_BINARY" --no-json install` and the log is exactly `install`. `install` and not `composer install` — `composer` has a case in `run.go` and bridges only on macOS, so it would prove nothing on Linux |

## 6 · The tests that block deletion

**`tests/unit/command_safety_test.sh:52-57`** — "and every entry has a file" becomes "and every
entry has a file **or** a route". It reads the routes with the extractor
`migration_status_test.sh:78-80` already uses, moved to `tests/lib/router.sh` as
`hm_routed_commands` and sourced by both. That file's own header says a copy nobody checks is a copy
that drifts; this is the second reader, so it is shared rather than pasted.

It does not become brittle, and that is already proven from the other side: if the router's shape
ever stops matching the `sed`, `hm_routed_commands` returns an empty list, and
`migration_status_test.sh:97-105` ("the partial exception is still routed") fails loudly, because
`db` and `proxy` would no longer be found. An empty extractor cannot pass quietly.

The same file gains the **reverse** direction, which nothing asserts today — `:44-50` says
file→entry and `:52-57` said entry→file, and neither says routed→no-file. New case **"and what the
binary owns outright has no file to drift from"**, over the twelve by name:

```bash
RETIRED="copy-to-container copy-from-container varnish-on varnish-off purge npm
         n98-magerun test-unit test-integration mysqldump version set-host"

drift=""
routed="$(hm_routed_commands)"
for name in $RETIRED; do
    [ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"
    grep -qx "$name" <<< "$routed" || drift="$drift ${name}(not-routed)"
done
```

Both directions, so the list cannot rot in either: a resurrected file fails, and a lost route fails.
By name and not "every routed command", because twenty routed commands legitimately still have a
file — `magento`, `start`, `mysql`, the partials, the three delegating stubs — and a blanket rule
would be false today and unfixable. It lands in the **last** commit, because eleven of the twelve
still have files until then.

**`tests/unit/exit_codes_test.sh`** gains the helper's refusal, which is a new user-visible message
and has no test anywhere: `source "$HELPERS_DIR/binary.sh"`, then **"hm_run_binary refuses when the
binary is not installed"** — `COMMAND_BIN_DIR` pointed at an empty `mktemp -d`, exit `1`
(`$HM_EXIT_ERROR`), and in JSON mode `.error.type` is `binary_missing` and the message names the
command that needed it. That file already sources `exit_codes.sh` and owns every other `hm_fail`
shape (`:9-39`), so this is the second reader of the same sentence, not a new suite.

**`tests/integration/concurrent_safety_test.sh:105-109`** — both cases go, replaced by a comment:

```bash
# The marker on the entries this tool writes, and being able to remove its own, are
# `dockergento/core/hosts.go:19-24` now, pinned by `TestOnlyWhatThisToolAddedIsRemoved` and
# `TestTheEntryIsItsOwnLine` in `core/hosts_test.go`. There is no set-host.sh to read.
```

`:111-115`, which asserts `hm clean` does not touch `/etc/hosts`, stays: it tests the tool, not a
file's contents.

**`tests/integration/cli_contract_test.sh`** gains §4's message beside the `command_not_found` cases
that are already there (`:24-45`): `run version` exits 2 with `.error.type` =
`command_moved_to_binary`, and `--no-json version` names `hm version` in the hint.

## 7 · The six parity cases, one by one

Each keeps the fact it pinned, asserted against the binary and something that is not the binary.
None becomes `assert_equals "$X" "$X"`.

| Case | What the comparison pinned | The binary-only assertion |
|---|---|---|
| `:95-96` npm | it runs in the php container, and the failure is the container's | a recorder `npm` at `/usr/local/bin/npm` in the container writes `pwd` and its argv in the unambiguous form below, and exits 42. Assert: the recorded directory is `/var/www/html`, the argv is exactly `1` / `[--version]`, and `hm npm --version` exits **42** — the container's status is the tool's status |
| `:98-101` n98-magerun | the same, for a wrapper that composes **one** `bash -c` string (`wrappers.go:55`) | see below — the recorder has to make the joining visible, or the case cannot fail on the regression it exists for |
| `:137-144` test-unit | that phpunit was reached **at all** with the whole command line, not one argument with spaces in it | exact equality of the entire recorded invocation: `/var/www/html` and `--config ./dev/tests/unit/phpunit.xml.dist`, nothing appended. `echo "$@"` cannot tell one argument from three — what tells them apart is that a single argument with spaces never invokes the recorder, and the log stays empty |
| `:179-181` mysqldump | the dump carries every row | the fixture inserts three rows instead of one; the dump is restored into a second database (`CREATE DATABASE verificacion`, load, `SELECT COUNT(*)`) and the count is 3. Completeness against the database itself, which is stronger than a `grep -c` against another dump — extended inserts make both counts 1 whatever happened |
| `:190-192` version `--json` | the document describes *this* installation — and the shell half was the only **independent** oracle for the values in it | git in the installation directory becomes that oracle. See below |
| `:201-203` the text | the words a person reads say what the document says | the same git oracle, per rendering. A cross-format comparison runs one binary against itself and cannot catch a value that is wrong in both renderings, so it is not the check |

The eleven cases that already run only the binary (`:75-84`, `:113-130`, `:163-177`, `:198-209`) do
not change. `$SHELL_CLI` disappears from the file.

### The two cases the table defers

**`n98-magerun`.** `wrappers.go:55` builds `[]string{"bash", "-c", "n98-magerun " + strings.Join(args, " ")}`
— three elements, the third one already joined. A recorder that logs `"$@"` or `"$*"` renders that
identically to three separate arguments, so it would pass either way; this is the very collapse the
`test-unit` row above relies on going the other direction. Alpine has no `bash`, which is why both
halves failed identically before and why the name is free to take. A recorder at
`/usr/local/bin/bash`:

```sh
#!/bin/sh
printf '%s\n' "$#"        >  /tmp/argv
printf '[%s]\n' "$@"      >> /tmp/argv
exit 43
```

Assert `/tmp/argv` is exactly

```
2
[-c]
[n98-magerun --version]
```

and that `hm n98-magerun --version` exits **43**. (`$#` is 2, not 3: `$0` is the recorder's own
name.) Had the arguments been passed separately the file would read `3` / `[-c]` / `[n98-magerun]` /
`[--version]`, so the case now fails on the regression it exists for. The `npm` recorder uses the
same two `printf` lines, and it is one shape written once.

**`version`.** The field is `commits_ahead`, not `commits` (`version.go:28`, `core/installation.go:16`).
A preflight first, because a check that skips is a check that passes:

```bash
COMMIT=$(git -C "$COMMAND_BIN_DIR" rev-parse --short HEAD 2>/dev/null)
DESCRIBED=$(git -C "$COMMAND_BIN_DIR" describe --tags 2>/dev/null)

test_case "the installation directory is a checkout, or nothing below is an oracle"
assert_equals "checkout" "$([ -n "$COMMIT" ] && echo checkout || echo "not a checkout")"
```

An empty `COMMIT` **fails** here rather than skipping the assertions under it: a version report
nobody can check is the defect, not the reason to stop looking. Then, field by field against what
`gitvcs.Git.Installed` actually reads (`gitvcs/vcs.go:173-204`):

| Field | Oracle |
|---|---|
| `.data.commit` | `$COMMIT` exactly — `rev-parse --short HEAD`, not a prefix of the long one |
| `.data.path` | `$COMMAND_BIN_DIR` |
| `.data.version` | `$DESCRIBED`, or exactly `unknown` when `describe` prints nothing (`vcs.go:189-191`) — both branches assert, neither skips |
| `.data.tag` | `git -C "$COMMAND_BIN_DIR" describe --tags --abbrev=0` |
| `.data.dirty` | `true` iff `git -C "$COMMAND_BIN_DIR" status --porcelain --untracked-files=no` prints anything |
| `.data.commits_ahead` | the `<n>` of `<tag>-<n>-g<hash>` in `$DESCRIBED`, else `0` (`core/installation.go:35-53`) |

A build reporting a stale or `unknown` version now fails, which is the thing the retired comparison
was there for. `:198` (`.data.binary` is not null) stays as it is — it has no git oracle, because it
is this binary's own build stamp.

The text rendering is checked against the same oracle and not against the JSON: its `commit` line is
`$COMMIT`, its `installed` line is `$COMMAND_BIN_DIR`, and its header carries `$DESCRIBED`. The
`branch` line is not asserted — on a detached checkout it is not printed at all (`version.go:41-45`).

## Behaviour changes declared

| # | Change | Why it is not a regression |
|---|---|---|
| a | `bin/run <one of the twelve>` refuses with `command_moved_to_binary` and exit 2 | it ran a duplicate implementation before. `hm <command>` and every documented path are unaffected |
| b | `hm setup`'s `start`, `mysql`, `magento`, `restart` and `set-host` steps are answered by the binary | that is what "ported" means, and it is already what a person typing those commands gets. Their behaviour is covered by the Go tests of each |
| c | `hm --help` takes its names from the declared commands ∪ the files | the output is byte-identical, captured and diffed |

Nothing else moves: no argument, prompt, `--json` document or exit code of any of the twelve.

## File changes

| File | Action |
|---|---|
| `console/commands/set-host.sh` | Delete (109 lines) |
| `console/commands/{copy-to-container,copy-from-container}.sh` | Delete (110 lines) |
| `console/commands/{version,varnish-off,varnish-on,mysqldump,purge,n98-magerun,test-unit,npm,test-integration}.sh` | Delete (176 lines) |
| `console/helpers/binary.sh` | Create — `hm_run_binary` |
| `console/commands/composer.sh`, `console/commands/debug-on.sh`, `console/tasks/magento_installation.sh` | Modify — one source line and one call each (`composer.sh`: two calls) |
| `console/helpers/print_help.sh` | Modify — names from both sources, sorted as `<name>.sh` |
| `bin/run` | Modify — `check_command_dir` |
| `dockergento/adapters/selfrun/runner.go`, `runner_test.go` | Create |
| `dockergento/dockergento.go:601`, `dockergento_test.go` | Modify — the door, and the test that it is |
| `dockergento/app/setup_test.go` | Create — the step order |
| `tests/lib/router.sh` | Create — `hm_routed_commands` |
| `tests/unit/command_safety_test.sh`, `tests/unit/migration_status_test.sh` | Modify — "a file **or** a route", and the routed→no-file direction |
| `tests/unit/exit_codes_test.sh` | Modify — `hm_run_binary`'s `binary_missing` refusal |
| `tests/integration/{concurrent_safety,cli_contract,go_setup,go_wrappers}_test.sh` | Modify |
| `MIGRATION.md` | Modify — the third step recorded as executed, in Spanish, as the rest of that file |

`varnish-off.sh:18` runs `"$COMMANDS_DIR"/purge.sh` — a fifth by-path caller, and the only one
needing no fix, because both files are among the twelve. It does constrain the split: `purge` and
`varnish-off` must die in the same commit. Both are in the last one, whichever split is taken.

`data/command_descriptions.json` and `docs/` are untouched: all twelve keep their entry, their
description and their documentation. They are still commands.

## Threat matrix

| Boundary | Applicability | Response | RED test |
|---|---|---|---|
| Router selection | Applicable | `bin/run` gains one branch on the miss path, before any Docker work; `run.go` is not touched at all. The declared-with-no-file predicate is kept true by `command_safety_test.sh`, in both directions | the `cli_contract_test.sh` cases in §6; `command_safety_test.sh`'s "a file or a route" **and** "what the binary owns outright has no file to drift from"; `migration_status_test.sh` unchanged |
| Subprocess argument composition | Applicable | `selfrun` builds an argv from the step slice — never a shell string. `hm_run_binary` passes `"$@"` through, and the one deliberate word split (`composer.sh:51`) is documented above with its glob analysis | `TestItRunsTheExecutableItWasGiven` asserts the exact argv; the `test-unit` and `n98-magerun` cases in §7 assert the argv that reaches the container, the latter with a recorder that counts it |
| Process integration / self-invocation | Applicable | `Executable` → `HM_SELF` → `$HM_LEGACY_ROOT/bin/run` → `os.Executable()`, which fails safe: a suite that has already redirected the tree cannot accidentally reach the real binary. `setup` is not one of its own steps; every hop is a different command; depth is bounded at three | `go_setup_test.sh:132-153` through the existing `HM_LEGACY_ROOT` recorder, no new hook; `TestTheInstallStepsRunInOneOrderThroughOneDoor`; `TestAnExecutableThatIsNotThereIsAnError` |
| Executable-file classification | Applicable | `hm_run_binary` refuses on `! -x`, with the stubs' sentence. `selfrun` refuses a resolved path that is not executable rather than exec'ing it. `print_help.sh` keeps `[ -f "$script" ]` and adds a declared name only when no file exists | "hm_run_binary refuses when the binary is not installed" in `tests/unit/exit_codes_test.sh`, with `COMMAND_BIN_DIR` pointed at a tree with no `bin/hm`; `TestAnExecutableThatIsNotThereIsAnError` |
| Documentation-like paths | N/A | no file is classified or executed by name; the help reads names, never runs them | — |
| Git repository selection | N/A | no VCS command is added to the tool. The `version` case *reads* `rev-parse`, `describe` and `status --porcelain` in a test, always with an explicit `-C "$COMMAND_BIN_DIR"` and never from the ambient directory, and mutates nothing | — |
| Commit / push state, PR commands | N/A | no VCS or PR automation in this change | — |

## Size, and an honest forecast

Measured with `git diff --shortstat <base>..HEAD -- . ':(exclude)openspec'`, additions plus
deletions, at this repository's density (production ~1.2x an estimate, tests ~1.8x):

Every row below is a component, and the totals are the sum of the rows. They are meant to be added
up by a reader who does not trust them.

| # | Component | Lines |
|---|---|---|
| c1 | `set-host.sh` deleted | 109 |
| c2 | `copy-to-container.sh` + `copy-from-container.sh` deleted | 110 |
| c3 | the nine remaining twins deleted | 176 |
| c4 | `console/helpers/binary.sh` | 24 |
| c5 | `magento_installation.sh` — one source, one call | 3 |
| c6 | `composer.sh` (+2 calls) and `debug-on.sh` — sources and calls | 7 |
| c7 | `print_help.sh` (+18/−5) | 23 |
| c8 | `bin/run` (+21/−10) | 31 |
| c9 | `tests/lib/router.sh` | 13 |
| c10 | `command_safety_test.sh` "a file **or** a route", `migration_status_test.sh`, `concurrent_safety_test.sh` (+22/−15) | 37 |
| c11 | `command_safety_test.sh` "no file to drift from" | 9 |
| c12 | `exit_codes_test.sh` — `binary_missing` | 14 |
| c13 | `selfrun` and its test | 90 |
| c14 | the wiring and the order tests | 75 |
| c15 | `go_setup_test.sh` — the bridge case | 5 |
| c16 | `go_wrappers_test.sh` (+100/−45) | 145 |
| c17 | `MIGRATION.md` (+8/−2) | 10 |
| | **Total** | **881** |

c11, c12, c15 and 25 of c16 are new since the first draft of this design: the `binary_missing`
refusal, the routed→no-file direction, the bridge fall-through and the two parity cases that were
not actually pinning what they claimed. They are the cost of §7's two oracles and §6's two named
tests, and they are not optional — each one exists because something above has no other proof.

**The two-unit plan does not fit, and the proposal says otherwise.** As stated, commit 1 is
c1+c2+c4…c10+c12…c15 = 541 and commit 2 is c3+c11+c16+c17 = 340. `proposal.md:103` claims "Each unit
is inside the budget on its own"; at 541 the first unit is not, and it was not at the first draft's
figures either — that draft's own three-way split double-counted the three `composer.sh`/`debug-on.sh`
call-site lines in both halves (1a 415 + 1b 118 = 533 against a stated total of 526; 1a is really
**≈ 408**, and 408 is still over 400). The claim has been wrong in every version of the arithmetic.

**Four commits do fit**, splitting commit 1 at two seams rather than one:

| | Content | Components | Lines |
|---|---|---|---|
| 1 | the setup door becomes the binary; nothing is deleted yet | c13 + c14 + c15 | **170** |
| 2 | the groundwork, and `set-host` | c4+c5+c7+c8+c9+c10+c12+c1 | **254** |
| 3 | `copy-to-container` and `copy-from-container` | c6 + c2 | **117** |
| 4 | the nine, the parity rewrite, and the drift test | c3+c11+c16+c17 | **340** |

170 + 254 + 117 + 340 = 881. Each is under 400 with room, and each is self-consistent:

- **1** changes the door before any file goes. `set-host.sh` still exists and is still unreachable
  through it, because `hm set-host` is routed in Go; `install` and `ssl` still bridge, which is what
  c15 now proves.
- **2** deletes `set-host.sh` in the same commit as its only caller (`magento_installation.sh:32`),
  `bin/run`'s message, the help's new name source and `command_safety_test.sh`'s "a file **or** a
  route" — which is the moment that assertion must relax, not before.
- **3** deletes the two copy commands with the three call sites that point at them.
- **4** carries c11 rather than any earlier commit, because "the twelve have no file" is false while
  eleven of them still do. `varnish-off.sh:18` runs `purge.sh`, so those two must die together; both
  are here.

`go test ./... -short` and `tests/run.sh tests/unit` pass at each of the four. Cumulatively 881 is
well over the budget, so the delivery decision the proposal defers is still required before apply —
what changes is that there is now a slicing that satisfies the per-unit rule, so it does not need a
`size:exception`.

Commit messages, in the repository's voice:

```
refactor(setup): the install door is the binary, and one test watches the order
refactor(console): set-host has one implementation, and bin/run says where it went
refactor(console): composer and debug-on copy through the binary
refactor(console): the last nine twins go, and the parity tests stand on their own
```

## Open questions

None blocking. One decision is deferred by design: whether the three delegation stubs
(`worktree.sh`, `docker-compose.sh`, `docker-stop-all.sh`) fold onto `console/helpers/binary.sh` —
they need an `exec` sibling, and the fourth stub is when that is worth writing.
