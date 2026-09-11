# Design: the third step for `list`, `describe` and `logs`

The deletions are three `rm`s (61 + 92 + 70 = **223** lines, counted). Everything below is the
repair that makes them legal, written with the files open.

**Size note, declared**: the phase skill budgets a design at 800 words. The launch prompt asks for
the literal before/after of every assertion, helper source, per-file line tables and a
break-analysis per replacement. Those are incompatible; the prompt wins and this document is long
on purpose.

---

## 0 · What the proposal got wrong, read against the code

The proposal corrected the exploration once (ten sites, seven suites, not eight across five). Reading
the code corrects the proposal four more times. Each of these is verified by reading, none by
running — the repository has no running Docker daemon in this session, so every claim below is a
claim about source, and the apply phase must execute the before/after check the proposal already
mandates.

| # | Claim | What the code says |
|---|---|---|
| A | Seven suites | **Eight.** `tests/integration/worktree_test.sh` (`HM` = `bin/run` at `:12`) drives `describe` at `:66`, `:73`, `:115`, `:151` and `list` at `:119`. Neither document names it. It carries an **eleventh** wrong-reason pass (`:73-74`) and one silent coverage loss (`:151`) |
| B | `introspection_test.sh` is "one line at `:8`" | It is one line **plus a corrected expectation**. `:70-72` asserts `describe --nope --json` exits **4**. It does so because `bin/run` runs `validate_docker_compose.sh` (exit 4, `project_not_configured`) *before* `describe.sh` ever parses `--nope` (`bin/run:289-299`). The binary parses arguments first (`internal/cli/describe.go:15-23`, and `go-entrypoint:594-601` requires it), so it answers **2**. The swap turns that case RED unless the expectation moves |
| C | `project_name_test.sh`'s helper goes to the binary | **It must not.** `bin/run:103` derives `COMPOSE_PROJECT_NAME` from the directory; the binary does not — `dockergento/app/resolve.go:116` reads `properties["COMPOSE_PROJECT_NAME"]` and stops. Three of that suite's fixtures (`make_project` default, `:26`) configure no name, so routing the helper would make `:69`, `:72`, `:91-94`, `:112` and `:115` report `""`. Name derivation is **not ported**; that is a separate cycle |
| D | 8 `logs` sites | **7** `logs` sites (`:163`, `:170`, `:182`, `:189`, `:196`, `:205`, `:212`) plus the `describe` at `:119` = 8 call sites |

Two further divergences that no parity test covers, found the same way:

- **`logs <unknown-option>` after the command name.** `logs` is in `transparent`
  (`internal/cli/globals.go:21`), so `--json` reaches the handler — which refuses every unknown
  `-*` with `invalid_argument`, exit 2 (`orchestrate.go:139-143`). The shell forwards it to Compose,
  which exits 1. `logs_launch_version_test.sh:211-213` asserts **1**. A twelfth site.
- **`describe` from an *unregistered* git worktree.** The shell reports
  `.data.project.worktree` from `$HM_WORKTREE`, which git alone sets
  (`collect_project_info.sh:120`). The binary reports it **only** for a registered worktree
  (`app/describe.go:108-110`; `app/resolve.go:59-63` documents why). `worktree_test.sh:66-67`
  expects `"wt"` for an unregistered one. `go_passthrough`'s whole-document `describe` comparison
  never caught this because its fixture is not a git repository.

None of these is a user-visible regression: `hm describe`, `hm list` and `hm logs` already reach the
binary. What moves is the *subject* of a test, from a dying implementation to the living one.

---

## 1 · The twelve sites, literally

`route` = point the call at `bin/hm`. `tighten` = assert a fact a refusal cannot satisfy.
`retire` = the shell half stops existing, so the comparison goes.

### route + tighten

**#1 `introspection_test.sh:57-59`** — `invalid_argument` and `command_moved_to_binary` are both 2.

```bash
# before
test_case "list rejects unknown options"
run_in "$WORKDIR" list --nope
assert_equals "2" "$STATUS"
# after
test_case "list rejects unknown options"
run_in "$WORKDIR" list --nope
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "invalid_argument"
```

Fails when: `list` stops validating before the engine, or a future refusal reuses code 2 with
another type. Both surface as a `.error.type` that is not `invalid_argument`.

**#1b `introspection_test.sh:70-72`** — new, see §0/B.

```bash
# before
test_case "describe rejects unknown options"
run_in "$WORKDIR" describe --nope --json
assert_equals "4" "$STATUS"
# after
test_case "describe rejects unknown options before it looks for a project"
run_in "$WORKDIR" describe --nope --json
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "invalid_argument"
```

Fails when: `describe` resolves the project before parsing (it would answer 4 /
`project_not_configured`). That is exactly the ordering `go-entrypoint:594-601` requires and
nothing at integration level asserted.

**#2 `introspection_test.sh:115-116`** — `assert_json_field` with an empty expected value:
`jq -r` answers `""` on empty stdin and on a refusal alike, and the helper swallows jq's stderr
(`tests/lib/assert.sh:149`, the `2>/dev/null`).

```bash
# before
test_case "without composer.lock the Magento version is unknown"
assert_json_field "$STDOUT" '.data.magento.version' ""
# after
test_case "without composer.lock the Magento version is unknown"
assert_json_field "$STDOUT" '.data.magento | has("version")' "true"
assert_json_field "$STDOUT" '.data.magento.version' ""
```

Fails when: the key disappears from the document, or the command answers nothing at all. An empty
document cannot satisfy `has("version") == true`.

**#4 `logs_launch_version_test.sh:169-171`** — `missing_value` and `command_moved_to_binary` are both 2.
Byte-for-byte the repair `379d317` made for `version` at `:59-63`.

```bash
# before
test_case "an option that needs a value says so when it has none"
run_in_lab logs --tail
assert_equals "2" "$STATUS"
# after
test_case "an option that needs a value says so when it has none"
run_in_lab_go logs --tail
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "missing_value"
```

Fails when: `--tail` stops being in `takesValue` (`orchestrate.go:96`) — the bare `--tail` would
then be `invalid_argument`, or worse be read as a service name and answer 5.

**#4b `logs_launch_version_test.sh:211-213`** — new, see §0.

```bash
# before
test_case "and a flag after the command name is left for Compose"
run_in_lab logs --json db
assert_equals "1" "$STATUS"
# after
# The binary enumerates the options it forwards rather than passing them through
# (`orchestrate.go:87-89`), so an undeclared one after the command name is its own usage error and
# never reaches Compose. The shell implementation forwarded it and inherited Compose's exit 1.
test_case "and an undeclared flag after the command name is a usage error, not Compose's"
run_in_lab_go logs --json db
assert_equals "2" "$STATUS"
assert_json_field "$STDERR" '.error.type' "invalid_argument"
```

Fails when: the option table stops rejecting unknown `-*`, which is the regression that reads
`--tail 3`'s `3` as a service name.

**#7 `project_name_test.sh:96-98`** — two calls to the same helper compared with each other.
Grouping **revised**: `tighten` only, never `route` (§0/C).

```bash
# before
test_case "but a configured name is inherited by a copy, because it is a decision"
cp -R "$LAB/otro-directorio" "$LAB/otro-directorio-copia"
assert_equals "$(hm_name "$LAB/otro-directorio")" "$(hm_name "$LAB/otro-directorio-copia")"
# after
test_case "but a configured name is inherited by a copy, because it is a decision"
cp -R "$LAB/otro-directorio" "$LAB/otro-directorio-copia"
assert_equals "hmnametest" "$(hm_name "$LAB/otro-directorio-copia")"
```

Fails when: the copy derives from its own directory name (`otro-directorio-copia`) instead of
reading the committed `COMPOSE_PROJECT_NAME`. Equality between two broken oracles could not.

**#8 `project_name_test.sh:114-115`** — same shape, same revision.

```bash
# before
test_case "so a worktree does not invent a second environment"
assert_equals "$(hm_name "$LAB/principal")" "$(hm_name "$LAB/una-rama")"
# after
test_case "so a worktree does not invent a second environment"
assert_equals "principal" "$(hm_name "$LAB/principal")"
```

Fails when: the main checkout's own resolution drifts. Paired with `:112`
(`assert_equals "principal" "$(hm_name "$LAB/una-rama")"`), both sides are now pinned to a literal
and the "no second environment" property holds without either side being allowed to be empty.

### route only — and the adjacent assertion that makes each safe

**#3 `introspection_test.sh:165-166`** (dormant behind `HM_TEST_PROJECT`).

```bash
# unchanged text; only $HM at :8 moves
test_case "the readable output is not JSON"
assert_not_contains "$STDOUT" '"schema_version"'
```

Safe because `:163`, on the **same** `$STDOUT` produced by the `run_in "$PROJECT_DIR" describe
--no-json` at `:162`, reads:

```bash
assert_contains "$STDOUT" "URLs"
```

Empty output fails loudly there, so the negative is never alone.

**#5 `logs_launch_version_test.sh:192-193`.**

```bash
test_case "to that service only"
assert_not_contains "$STDOUT" "hello-from-phpfpm"
```

Safe because `:190`, on the same `$STDOUT` from `run_in_lab logs db` at `:189`, reads:

```bash
assert_contains "$STDOUT" "hello-from-db"
```

**#6 `logs_launch_version_test.sh:208-209`.**

```bash
test_case "so nothing of the JSON envelope appears in it"
assert_not_contains "$STDOUT" "schema_version"
```

Safe because `:206`, on the same `$STDOUT` from `run_in_lab --json logs db` at `:205`, reads:

```bash
assert_contains "$STDOUT" "hello-from-db"
```

**#11 `worktree_test.sh:73-74`** — new; the empty-expected shape of #2, this time surviving by
adjacency rather than by a guard.

```bash
test_case "the main checkout is not treated as a worktree"
run_in_go "$LAB/main" describe --json
assert_json_field "$STDOUT" '.data.project.worktree' ""
```

Safe because `:77`, on the same `$STDOUT`, reads:

```bash
assert_json_field "$STDOUT" '.data.project.name' "$PROJECT"
```

A refusal or an empty document fails there, so the `""` can no longer hold trivially.

### retire — the two parity comparisons

**#9 `go_passthrough_test.sh:80-83`** and **#10 `go_passthrough_test.sh:99-102`**: identical shape.

```bash
# before
both describe --nonsense
test_case "an option nobody declared is refused the same way"
assert_equals "2" "$GO_STATUS"
assert_equals "$SHELL_STATUS" "$GO_STATUS"
# after
solo describe --nonsense
test_case "an option nobody declared is its own usage error"
assert_equals "2" "$GO_STATUS"
assert_json_field "$(cat "$LAB/go.err")" '.error.type' "invalid_argument"
```

(`list --nonsense` identically, with `list` in place of `describe`.) The dropped line is the one
that would have passed on `command_moved_to_binary` — the `version` regression, byte for byte.

### the sites that fail loudly and still need rewriting

`concurrent_safety_test.sh:61`, `:64`; `mail_catcher_test.sh:203`; `worktree_test.sh:66`, `:115`,
`:119`, `:151`; `go_passthrough_test.sh:63-78`, `:91-97`, `:104-106`;
`go_orchestration_test.sh:165-181`; `project_name_test.sh:80-84`. Expected values are non-empty,
so a refusal fails them — but they still have to be rewritten or eight suites go red.

`worktree_test.sh:65-67` is the one that changes meaning rather than address (§0):

```bash
# before
test_case "a worktree is recognised as such"
run_in "$LAB/wt" describe --json
assert_json_field "$STDOUT" '.data.project.worktree' "wt"
# after
# A worktree with no environment of its own keeps resolving against the main checkout
# (`app/resolve.go:59-63`), and the binary names a worktree only when it has one
# (`app/describe.go:108-110`). That a worktree is recognised at all is proved below, by the eight
# commands refused from it with exit 6 — through `bin/run`, which still owns them.
test_case "a worktree with no environment of its own resolves to the main checkout"
run_in_go "$LAB/wt" describe --json
assert_json_field "$STDOUT" '.data.project.worktree' ""
```

Empty-expected again, and again safe only by adjacency: `:70`, unchanged, asserts
`.data.project.root` is `$LAB/main` on that same stdout.

`project_name_test.sh:80` changes command, not entry point:
`( cd "$LAB/___" && "$HM" describe --json ... )` becomes `... "$HM" db list --json ...`. The
refusal under test is `bin/run:106`'s `no_project_name`, raised in `docker_env_variables` before
any command file is reached, so any surviving shell command is an equally good window and `db`
is one. `:81` (exit 4) and `:84` (`.error.type`) are untouched and still fail loudly on drift.

---

## 2 · The shape of the repair, per suite

| Suite | Shape | Why |
|---|---|---|
| `introspection_test.sh` | `HM="$COMMAND_BIN_DIR/bin/hm"` at `:8` | the whole file is `list`/`describe`; nothing else in it drives `bin/run` |
| `logs_launch_version_test.sh` | keeps `HM` + `GO_BINARY` (`:8`, `:15`); gains `run_in_lab_go` | `launch` is still shell and keeps `run_in_lab`; `logs` and `describe` move |
| `project_name_test.sh` | **stays on `bin/run`**; `hm_name()` changes command; `GO_BINARY` only for `:157` | the subject is `bin/run`'s own derivation (§0/C) |
| `concurrent_safety_test.sh` | `GO_BINARY` + 2 call sites | `worktree add`, `clean` keep `bin/run` (`:11`) |
| `mail_catcher_test.sh` | `GO_BINARY` + 1 call site | `doctor` is still shell and keeps `bin/run` (`:181`) |
| `worktree_test.sh` | `GO_BINARY` + `run_in_go` + 5 call sites | the guardrail cases (`:81-120`) drive `start`/`stop`/`down`/`setup` through `bin/run` and must stay there |
| `go_passthrough_test.sh` | `solo` replaces `both` for the two blocks | `both` survives for `doctor`, `docker-compose`, `worktree` |
| `go_orchestration_test.sh` | `$SHELL_CLI` drops out of the `logs` block only | `start`, `exec` comparisons keep it |

### The helper, and why there is more than one

`logs_launch_version_test.sh` gains, beside `run_in_lab` (`:26-32`):

```bash
#
# The binary's twin of run_in_lab. `logs` and `describe` are the binary's now; `launch` is still
# the shell implementation's and keeps asking bin/run through run_in_lab above.
#
run_in_lab_go() {
    ( cd "$LAB" && "$GO_BINARY" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}
```

`worktree_test.sh` gains the same four lines with the directory argument its own `run_in`
(`:53-61`) already takes:

```bash
run_in_go() {
    local dir="$1"
    shift
    ( cd "$dir" && "$GO_BINARY" "$@" >"$LAB/out" 2>"$LAB/err" )
    STATUS=$?
    STDOUT=$(cat "$LAB/out")
    STDERR=$(cat "$LAB/err")
    return 0
}
```

| Decision | Choice | Rejected | Why |
|---|---|---|---|
| A twin helper per suite, not a shared one | two local helpers, eight lines each | one helper in `tests/lib/` taking a binary and a directory | exactly two suites have the capture shape, and they disagree about the directory (`$LAB` vs an argument). The other three mixed suites use inline `$( cd … && "$HM" … \| jq … )` command substitutions and need a **variable**, not a helper. A shared helper would have one caller per signature |
| A twin, not a parameter on `run_in_lab` | `run_in_lab_go logs …` | `run_in_lab "$GO_BINARY" logs …` | `run_in_lab` has ~13 surviving `launch` call sites; parameterising rewrites all of them, for a change that is supposed to touch eight |
| The name says which side it is | `_go` suffix | `run_in_bin` | `GO_BINARY` is the variable this repository already uses for the same thing (`logs_launch_version_test.sh:15`, `go_passthrough_test.sh:12`) |

`project_name_test.sh`'s helper changes command rather than binary:

```bash
# `describe` is the binary's now, and the binary reports the configured name rather than the one
# this entry point derives (`bin/run:103` against `app/resolve.go:116`): derivation is not ported.
# What is under test here is bin/run's own resolution, so the window onto it stays inside bin/run.
# `db list` is still the shell implementation's and prints the name it resolved (`db.sh:171-174`).
hm_name() {
    ( cd "$1" && "$HM" db list --json 2>/dev/null ) | jq -r '.data.project // ""'
}
```

---

## 3 · What replaces the two lost parity comparisons, and what is no longer proved

A parity test's value was having a second implementation to disagree with. Nothing replaces that.
What replaces each comparison is an assertion against a **fixed oracle that is not the binary** —
the fixture's own configuration, or the container's own output — which is weaker in one specific
way, stated per block.

### `go_passthrough_test.sh:57-106`

| Was | Becomes | No longer proved |
|---|---|---|
| `:63-65` whole `describe` document, shell vs Go | field assertions against the fixture: `.data.project.name == "$PROJECT"`, `.domain == "go.test"`, `.paths.magento_dir == "./src"` (already at `:256-263`, now the only proof), plus `.data \| has("project") and has("magento") and has("services") and has("paths") and has("tooling")` — **not** the multi-argument `has("project","magento",…)`, which in jq is a generator and prints one boolean per line, so `assert_json_field`'s single-string comparison could never match even against a correct document (verified: `echo '{"a":1,"b":2}' \| jq '. \| has("a","b")'` prints `true` twice) | **a field nobody names.** The comparison covered every key of the document including ones no assertion mentions. The replacement covers the keys somebody wrote down |
| `:67-69` with `--with-secrets` | `.data.credentials.database \| has("password") == "true"` | that the credential *values* match the shell's |
| `:71-73` credentials never volunteered | unchanged — already Go-only | — |
| `:75-78` the table, character for character | `NO_COLOR=1 "$GO_BINARY" --no-json describe` contains the section headings `Environment`, `URLs`, `Magento`, `Services`, `Paths`. **Not the project name** — this row originally asked for it, and running the binary against the fixture shows the rendered table never prints it (status, domain, worktree, root, URLs, Magento, Services and Paths only), so that assertion would have failed always | **the rendering's exact bytes.** Column widths, ordering and spacing had a second opinion; now they have none |
| `:80-83` `--nonsense` | #9 above | nothing — the shell half was the wrong-reason half |
| `:91-93`, `:95-97`, `:104-106` `list` | `.schema_version == 1`, `.command == "list"`, `.data.count \| type == "number"`, and every environment carrying the nine documented keys — the assertions `introspection_test.sh:37-52` already makes, which is why they are **not duplicated here**: the `list` block collapses to the envelope shape and `:124-125`'s piped-output default | the table rendering, as above |

### `go_orchestration_test.sh:165-181`

| Was | Becomes | No longer proved |
|---|---|---|
| `:165-168` all logs, both, sorted | `"$GO_BINARY" --no-json logs --no-color` contains `listo phpfpm` **and** `listo nginx` — the fixture's own `command:` strings (`:65`, `:70`), which is an oracle written before either implementation ran | **the prefix format.** The comparison covered the `service-name  \|` prefix Compose prints; the replacement only requires the text to be present |
| `:171-174` one service, character for character | `logs nginx` contains `listo nginx` and **not** `listo phpfpm` — a paired positive/negative on one stdout, so neither half can hold on an empty capture | the exact bytes, as above |
| `:176-181` unknown service | exit **5**, `.error.type == "unknown_service"`, and the refusal names `phpfpm` and `nginx` — the services the fixture defines | **that the two refusals are the same JSON.** `:181` compared the whole envelope; the hint's exact wording now has one author |

`openspec/specs/go-entrypoint/spec.md:586-588` predicted this inheritance in writing. The delta
must say the guarantee now lives in `logs_launch_version_test.sh` (`:162-167` after renumbering —
`:583-585`'s claim that it "runs the Go binary" becomes true only in this change) and in the
Go-only block above, and that no second implementation checks it any more.

---

## 4 · Commits, ordering, and the arithmetic

**Ordering constraint, verified.** `tests/unit/command_safety_test.sh:69-70` holds `RETIRED`;
`:74-77` walks it, and `:75` is literally

```bash
    [ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"
```

A listed name that still has a file **is** drift. Adding `list describe logs` while the files exist
makes the suite red, so that edit belongs in the deletion commit and nowhere else. Confirmed at
those exact lines. Its sibling case at `:53-61` ("every entry has a file **or** a route") stays
green throughout, because all three are routed (`internal/cli/run.go:53`, `:55`, `:63`).

Everything else runs the other way: repairs first, with the files present, so a corrected assertion
can be watched passing on the live shell command and then on the binary. Deleting first makes eight
suites red at once and a repair that works indistinguishable from one that merely stops the
complaint.

### Commit 1 — repairs (files still present)

| File | Action | +/− | Lines |
|---|---|---|---|
| `tests/integration/introspection_test.sh` | `HM` at `:8`; #1 tighten; `:70-72` expectation + type; #2 `has()` guard | +7/−3 | 10 |
| `tests/integration/logs_launch_version_test.sh` | `run_in_lab_go`; 8 call sites; #4 tighten; `:211-213` rewritten | +21/−10 | 31 |
| `tests/integration/project_name_test.sh` | `GO_BINARY`; `hm_name` command + comment; `:80`; #7; #8; `:157` | +10/−5 | 15 |
| `tests/integration/concurrent_safety_test.sh` | `GO_BINARY`; `:61`, `:64` | +3/−2 | 5 |
| `tests/integration/mail_catcher_test.sh` | `GO_BINARY`; `:203` | +2/−1 | 3 |
| `tests/integration/worktree_test.sh` | `GO_BINARY`; `run_in_go`; 5 call sites; `:65-67` rewritten | +16/−7 | 23 |
| **Subtotal 1a — the six repaired suites** | | | **87** |
| `tests/integration/go_passthrough_test.sh` | `:57-106` parity retired, Go-only kept | +30/−25 | 55 |
| `tests/integration/go_orchestration_test.sh` | `:165-181` parity retired, Go-only kept | +14/−17 | 31 |
| **Subtotal 1b — the two parity retirements** | | | **86** |
| **Commit 1 total** | | | **173** |

### Commit 2 — deletions

| File | Action | Lines |
|---|---|---|
| `console/commands/list.sh` | Delete | 61 |
| `console/commands/describe.sh` | Delete | 92 |
| `console/commands/logs.sh` | Delete | 70 |
| `tests/unit/command_safety_test.sh` | `RETIRED` gains `list describe logs` | 3 |
| `tests/integration/cli_contract_test.sh` | one `command_moved_to_binary` case for `describe`, after `:94` | 10 |
| `MIGRATION.md` | sibling block at `:157-176` | 12 |
| **Commit 2 total** | | **248** |

**The halves add up**: 87 + 86 = 173, and 173 + 248 = **421**. Both commits are inside the
400-line budget on their own; cumulative 421 is over it, and at this repository's recorded 1.3–2x
overshoot (170→273, 50–90→184, 120–180→302, 841→1364) plausibly 550–840. The delivery decision
belongs **before apply**.

**Contingency, already sized**: subtotals 1a and 1b are a clean seam — repairs of assertions versus
removals of comparisons. Three commits are **87 / 86 / 248 = 421**, the same total.

Measured against this change's own parent, never a literal hash:
`BASE=$(\git rev-parse HEAD)` before the first commit, then
`\git diff --shortstat "$BASE"..HEAD -- . ':(exclude)openspec'`.

---

## 5 · The check that actually works before deleting

`rtk proxy rg -l '<command>' tests/` is necessary and insufficient — proven twice in this cycle:
the two parity blocks hide behind `$SHELL_CLI`, and `worktree_test.sh` was missed even though the
name is in it four times. The working check has three parts and all three are required.

**Part 1 — every file that mentions a name, with the entry points it defines.** The second command
is the part a name-only grep does not do.

```bash
rtk proxy rg -l --glob 'tests/**/*.sh' '\b(list|describe|logs)\b' tests/ \
  | xargs rtk proxy rg -n '^[[:space:]]*(HM|GO_BINARY|SHELL_CLI|CLI|BIN)=' 
```

Any file whose assignment resolves to `bin/run` is **in scope until it is explicitly cleared**.
Clearing means writing down, per hit, one of four labels: `binary` (safe), `shell entry point`
(repair), `prose` (a comment, a string, a docs path), or `another command` — `db list`,
`worktree list`, `docker-compose logs` contain the word and are not these commands. A hit with no
label is the miss; that is the rule the exploration broke.

**Part 2 — resolve the variable, not the name.** For each hit, walk outward: the invocation is
`"$VAR" … <name>` or `helper … <name>`. Resolve `helper` to its definition **in the same file** and
read the `"$VAR"` inside it (`run_in`, `run_in_lab`, `both`, `hm_name` all hide one), then resolve
`$VAR` to its assignment. Four of this cycle's twelve sites are behind a wrapper and two behind
`$SHELL_CLI`.

**Part 3 — the execution half, because reading finds the loud ones and misses the quiet ones.**

```bash
tests/run.sh > /tmp/before.txt
for n in list describe logs; do mv "console/commands/$n.sh" "/tmp/$n.sh.away"; done
tests/run.sh > /tmp/after.txt
for n in list describe logs; do mv "/tmp/$n.sh.away" "console/commands/$n.sh"; done
diff /tmp/before.txt /tmp/after.txt
```

Every line the diff shows is a site part 1 had to have found; a site in the diff that is not on the
list means the list is wrong. **The cases that do not move are the interesting ones**: a case that
passes in *both* runs while driving `bin/run` for one of the three names is a wrong-reason
candidate and must be opened by hand. That is the step the `version` cycle skipped — it never ran
the suite at all — and the step that finds passes like #2, #3, #5, #6, #9, #10 and #11, which no
diff of results will ever show.

---

## 6 · What each replacement must not become

The defect this repository has now found eight times in three days is an assertion that cannot
fail. Six of the twelve sites are being repaired precisely because they were one. Each replacement
below is stated with the break it catches and how that break surfaces; nothing is specified that
cannot fail.

| # | Replacement | Plausible break | How it fails |
|---|---|---|---|
| 1 | `list --nope`: 2 **and** `.error.type == "invalid_argument"` | `list` validates after the engine, or reuses code 2 for another type | `.error.type` reads something else; on the shell entry point it would read `command_moved_to_binary` |
| 1b | `describe --nope`: 2 **and** `invalid_argument` | `describe` resolves the project before parsing | exit 4, `.error.type` `project_not_configured` |
| 2 | `has("version") == "true"` before `== ""` | the key is dropped, or the command answers nothing | `has()` reads `false` on a document without it and `null`→mismatch on empty stdin |
| 3 | route only | `describe --no-json` prints nothing, or prints JSON | `:163` fails on the missing `URLs`; the negative fails if the envelope appears |
| 4 | `logs --tail`: 2 **and** `missing_value` | `--tail` leaves `takesValue` | `.error.type` becomes `invalid_argument`, or the bare flag is read as a service and answers 5 |
| 4b | `logs --json db`: 2 **and** `invalid_argument` | the unknown-`-*` branch is removed and options are forwarded again | exit 1 from Compose, no envelope to read |
| 5 | route only | the service filter stops narrowing | `hello-from-phpfpm` appears; `:190` guards the empty case |
| 6 | route only | the JSON envelope wraps a data stream | `schema_version` appears; `:206` guards the empty case |
| 7 | copy equals literal `hmnametest` | derivation wins over the committed name | reads `otro-directorio-copia` |
| 8 | main checkout equals literal `principal` | the main checkout's own resolution drifts | reads the derived or empty value; `:112` pins the worktree side independently |
| 9 | `describe --nonsense`: 2 **and** `invalid_argument` | as #1 | `.error.type` |
| 10 | `list --nonsense`: 2 **and** `invalid_argument` | as #1 | `.error.type` |
| 11 | main checkout `.project.worktree == ""` | the binary starts naming a worktree it has no environment for | reads a name; `:77` on the same stdout guards the empty capture |

**One candidate is not specified, deliberately.** `worktree_test.sh:151`'s
`run_in "$LAB/wt" describe --json` exists only to run *an allowed command* before asserting
`mounts_pointing_at "$LAB/wt" == 0`. Once `describe` is the binary's, that call proves nothing the
`doctor` call on the next line does not already prove, and after deletion it would have proved
nothing at all while still reading green. It is **routed and kept as a second allowed command**,
not dressed up with an assertion: there is no assertion about it that could fail, and inventing one
would add the ninth instance of the defect this section exists to prevent.

Likewise **no replacement is specified for the `describe` table's exact bytes** (§3). A
cross-format check (`--no-json` against `--json`) runs one binary against itself and cannot catch a
value wrong in both; a byte-for-byte golden would have to be regenerated by the thing it checks.
The honest disposition is the section-heading assertion plus the declaration that column widths and
spacing lost their second opinion — recorded in the spec delta, not papered over with a tautology.

---

## Interfaces / contracts

No production interface changes. `bin/run`'s `check_command_dir` (`:72-90`) already answers
`command_moved_to_binary` with `HM_EXIT_USAGE=2` for a declared command with no file; these three
inherit it with no edit. `data/command_descriptions.json` keeps all three entries, which is what
keeps them in `--help`, and `console/helpers/print_help.sh:54-67`'s declared ∪ glob union already
generalises.

## Testing strategy

| Layer | What | How |
|---|---|---|
| Unit (shell) | `RETIRED` covers the three, in both directions | `command_safety_test.sh:68-78`, commit 2 |
| Integration | the twelve corrected assertions | run at commit 1 against the live shell, then against the binary; each broken on purpose once |
| Integration | the shell entry point's refusal | `cli_contract_test.sh`, new `describe` case, commit 2 |
| Integration | `hm --help` unchanged | `bin/run --help` captured before commit 1, diffed after commit 2 |
| Go | unchanged | `go test ./... -short`, `gofmt -l ./cmd ./internal`, `go vet ./...` |

## Threat matrix

| Boundary | Applicability | Response | RED test |
|---|---|---|---|
| Router selection | **Applicable** | `bin/run` gains no branch; the existing `command_moved_to_binary` path starts answering for three more names. `internal/cli/run.go` is not touched | `cli_contract_test.sh`'s new `describe` case; `command_safety_test.sh`'s drift case in both directions |
| Shell command / subprocess argument composition | **Applicable** | `logs` forwards an enumerated option set to Compose (`orchestrate.go:96`) rather than passing arguments through; this change makes the binary's enumeration the only implementation | #4 (`missing_value`) and #4b (`invalid_argument`) |
| Executable-file classification | **Applicable** | three files stop existing; `print_help.sh` keeps `[ -f ]` and adds a declared name only when no file exists, unchanged | `hm --help` byte-diff; `command_safety_test.sh:45-61` |
| Process integration / self-invocation | N/A | no caller reaches these three by path — verified: no `"$COMMANDS_DIR"/{list,describe,logs}.sh` anywhere | — |
| VCS / PR automation | N/A | none added | — |
| Documentation-like paths | N/A | no file is classified or executed by name | — |

## Migration / rollout

No data migration. Two commits (or three, on the contingency seam) on `release/2.0.0`;
`\git revert` in reverse order restores the files and every suite edit. Reverting the deletion
commit alone leaves the repairs consistent and green — they drive the binary, which does not care
whether the shell twins exist.

## Open questions

- [ ] **Not blocking, but it must be run**: every §0 finding is read from source, not executed.
      `mail_catcher_test.sh`'s fixture configures no `COMPOSE_PROJECT_NAME`, so the binary resolves
      `project.Name == ""` there; the mail URLs come from the compose configuration rather than from
      running containers, so `:203` should still answer. If it does not, add
      `"COMPOSE_PROJECT_NAME": "hmmailreported"` to `make_project`'s properties — a fixture detail,
      not the property under test.
- [ ] **For the spec phase, not for this change**: project-name derivation is not ported
      (`bin/run:103` against `app/resolve.go:116`), so `hm start` in a project with no configured
      name resolves to `project_not_configured` while `bin/run start` works. That is why
      `project_name_test.sh` stays on `bin/run`. It deserves a `MIGRATION.md` line and a backlog
      entry; it is out of scope here.
- [ ] `go-entrypoint`'s retired list must read fifteen, not twelve (`spec.md:749-756`, `:765-767`),
      and `:576-591` must record where the unknown-service guarantee now lives **and** correct its
      own stale `:148-153` reference to `:162-167`.
