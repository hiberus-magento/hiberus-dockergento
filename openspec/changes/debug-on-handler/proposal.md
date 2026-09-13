# Proposal: `debug-on` stops going through bash

Backlog ID: none — no `docs/research/backlog.md` ID covers the Go migration itself; it is tracked by
`MIGRATION.md` and ADR-007/ADR-009 bis. Same rationale as `debug-off-in-go` and its predecessors.

Unit 3 of three, the last and largest. Units 1 (`capture-container-output`) and 2
(`debug-on-host-detection`) shipped today; this unit composes them into a handler and routes the
command. Built on `openspec/changes/debug-on-handler/exploration.md`, whose claims were re-verified.

## Intent

`console/commands/debug-on.sh` (38 lines) is the last xdebug command still on the bash side
(`MIGRATION.md:261` says `shell`; `:260` says `debug-off` is `go`). Its two dependencies are already
in Go and have no caller: `(*dockergento.Engine).Capture` (`dockergento.go:469`) and
`machine.Host.XdebugHost` (`adapters/machine/xdebug_host.go:28`). This unit gives both a caller,
which is what makes `internal/cli/engine.go:14-16`'s written invariant — the interface "names what
the handlers call and nothing else" — true again.

## Scope

### In Scope

- **The seam grows 16 → 18.** `Capture` and `XdebugHost` join `commands` (`engine.go:17-34`, exactly
  sixteen methods today, verified). `XdebugHost` needs a new
  `(*dockergento.Engine).XdebugHost() (string, error)` facade first: `internal/cli` imports nothing
  under `dockergento/adapters/` (grep, zero hits), so the adapter cannot be called directly.
- **`outcome` grows a data field**, not `fakeEngine` — see Decision 4.
- **Two pure parsers** with fixtures: the Xdebug version out of `php -v` and its v2/v3
  classification; the `grep -e ... | cut -d= -f1-2` equivalent over `php -i`.
- **The `debugOn` handler**, one `case` in `run.go`'s switch, and its seam tests.
- **`MIGRATION.md`**: `:21` `33 de 65` → `34 de 65` and `:261` `shell` → `go`, in the same commit as
  the `case` (`tests/unit/migration_status_test.sh` checks that direction).

### Out of Scope / Non-goals

- **No shell file is retired.** `debug-on.sh` and `set_xdebug_host_property.sh` both stay; that is
  step three and its own cycle. No `bin/run` change.
- **`toolinfo.Reader.Xdebug`'s hand-rolled label disjunction** (`adapters/toolinfo/tooling.go:84-85`,
  re-verified) — a defect now recorded across five cycles. Neither reused nor fixed here.
- **`Transfer.container`** (`app/transfer.go`), the third copy of the resolution shape. Untouched.
- The `tests/run.sh` silent-fallthrough and the CI `gofmt` gap (CI checks `./cmd ./internal`, not
  `./dockergento`). Both known, both somebody else's cycle.
- **No bash test is ported**: no file under `tests/` mentions `debug-on` (verified). `docs/debug-on.md`
  and the `data/command_descriptions.json` entry already exist and need no change.

## Decisions for the user (three)

Each is laid out with a recommendation and its reasoning. **The call is the user's**; nothing below
is settled, and Slice 3 turns each into RED/GREEN behaviour, so settling them before apply is what
keeps the estimate honest.

### 1. What `hm debug-on` does when Xdebug is not installed

`debug-on.sh:27` is a bare assignment under `set -euo pipefail`, so when `grep "Xdebug v"` matches
nothing the script aborts there — and the `else` at `:36` that prints "Xdebug is not installed or the
version is not recognized" therefore **cannot fire for the absent case**. It remains reachable only
for a version matching `Xdebug v` but neither `v2.` nor `v3.` — a hypothetical v4. In Go the branch
becomes reachable for the first time.

| Option | Consequence |
|---|---|
| (a) Replicate the abort | `hm debug-on` fails after its enabling work already succeeded |
| (b) Non-fatal, informative branch | enable what can be enabled, print the message, exit 0 |

**Recommendation: (b).** Three reasons. By line 27 the seds, the uncomment and the restart have all
already succeeded, so (a) reports failure for work that was done. The abort is an accident of `set
-e` on a bare assignment, not a choice — and the author's visible intent, written nine lines below,
is to inform. And the branch is narrower than it looks: where the image ships no
`docker-php-ext-xdebug.ini` at all, the Go handler already fails at the earlier `sed` step naming
it, exactly as `debug-off` does; this branch is reached only when the ini exists and the extension
does not load.

**On the asymmetry with unit 2**, which decided the opposite way: there the *silent* path hid a
misconfiguration the user could not see, so failing added information. Here the shell already fails
and the message is the information. Deciding this the same way would be consistency for its own sake.

### 2. The `--json` envelope's shape

No shell precedent. `debug-off` ships `{"xdebug": false}`; the only other precedent is `describe`'s
`core.Tooling.Xdebug`, a single `"on"`/`"off"`/`"unknown"` string (`core/environment.go:240`) — too
thin to dictate a shape. `debug-on` has a version plus ini values whose **key names differ by
version** (v2 `xdebug.remote_host`/`remote_port`, v3 `xdebug.client_host`/`client_port`, both
`xdebug.idekey`).

| Shape | Payload | Tradeoff |
|---|---|---|
| (a) minimal mirror | `{"xdebug": true}` | cheapest, symmetric with `debug-off`; discards everything the banner shows |
| (b) verbatim keys | `{"xdebug": true, "version": "3.1.5", "host": "172.17.0.1"\|null, "settings": {"xdebug.client_host": "...", ...}}` | reports exactly what was read; a consumer must branch on version |
| (c) normalised | same, but `settings` renamed to `idekey`/`host`/`port` | one shape for both versions; asserts v2 and v3 keys are equivalent |

**Recommendation: (b).** `version` as the verbatim string from `php -v` rather than a structured
field — deriving a major/minor pair invents precision the parser does not need, while the string is
what the banner already shows. `host` reported as the address actually patched, `null` when nothing
was patched, which carries unit 2's not-applicable/unresolved distinction outward instead of
flattening it. (c) is rejected because the rename would assert an equivalence this port never
verified; v3 renamed those settings and changed their defaults, and a JSON consumer that needs the
real key name would have to guess it back.

### 3. Whether the Go path needs `start_service_if_not_running`'s equivalent

`debug-on.sh:8` probes with `compose exec -T` and, **only on status exactly 1**
(`start_service_if_not_running.sh:11`, verified), runs `$COMMAND_BIN_NAME start` — the whole
environment, not `phpfpm`.

**Recommendation: drop it**, as `debug-off` already did (shipped, and pinned by
`go-entrypoint/spec.md:349-358`). Starting an entire nine-service environment as a side effect of
"turn on xdebug" is disproportionate and unannounced; `Start` is **not** on `commands`, so keeping it
would grow the seam to nineteen for this path alone; the probe fires only on status exactly 1, so it
is already unreliable; and keeping it would make the twins diverge in the opposite direction from
the one `debug-off` just chose. With the environment down, `hm debug-on` then fails with the status
`Exec` returns and names the failing step. **This is an acceptance criterion, not a footnote.**

## Settled — recorded, not reopened

4. **The fake's data field goes on `outcome`, not on `fakeEngine`.** Unit 1's proposal sketched
   `f.captured`, matching the house per-field pattern (`project`, `stopped`, `description`). That
   pattern **breaks here**: `debug-on` captures twice — `php -v` then `php -i` — and needs different
   answers for each. `outcomes` is already "indexed by call number"
   (`fake_engine_test.go:48-50`), so `outcome{captured: "..."}` gives per-call answers for free.
   `call` grows nothing: `Dir`, `Service` and `Command` already exist.
5. **`Restart` introduces a refusal that does not exist today.** `restart.sh` is `stop.sh` then
   `start.sh`, so `commands.Restart` is the faithful port of `:26` — but `Operator.Restart` calls
   `refuseFromAnUnregisteredWorktree` unconditionally (`app/orchestrate.go:238`, verified), and
   `debug-on` is absent from `hm_alters_environment` (`console/helpers/worktree.sh:85-94`, verified),
   which tests only the top-level invoked name. So `hm debug-on` gains **exit 6 from an unregistered
   worktree**. Disclosed criterion, not an inherited side effect.
6. **The macOS copy branch costs nothing.** `copy-from-container` is routed (`run.go:104`), its
   handler is `copyFrom()` (`wrappers.go:303`) and `CopyFrom` is on `commands` (`engine.go:24`) — all
   verified. Calling a sibling handler mid-body is established precedent
   (`wrappers_test.go`'s varnish `off` case), and brings extra `Resolve` calls into the log. The
   shell also prints `Copying generated code into host` in yellow (`:22`); `copyFrom` prints nothing
   on success, so the handler prints that line itself via `warning()` (`palette.go:71`).
7. **Inert escapes are dropped, with a comment**, exactly as `debug-off`'s shipped Decision 3 did for
   `\z`. `\=` in the two host patterns (`:15-16`) and `\;` in `s/^\;zend_extension/` (`:19`) are
   undefined escapes that sed treats as the literal character. `\.` is meaningful and stays.
8. **The undocumented `XDEBUG_HOST` environment override is dropped.** The shell skips detection when
   `$XDEBUG_HOST` is already set (`:10`); nothing in the repo sets it except
   `set_xdebug_host_property.sh` (grep, verified), so it is a user-environment escape hatch only.
   Unit 2's `XdebugHost()` has no override. Disclosed.
9. **Unit 2's decision stands**: on Linux an unresolvable `docker0` is a failure. `XdebugHost` already
   answers `("", nil)` on macOS and `("", ErrNoBridgeAddress)` on an unresolved Linux
   (`xdebug_host.go:11-52`), so the handler renders each differently.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `go-entrypoint`: "A ported command's engine interaction is provable without Docker"
  (`spec.md:253-263`) gains `debug-on` — the nineteenth, and the first needing **two** new seam
  methods; "The state of the migration is written down and true" (`:106`) covers the row and counter.
  **Not** touched: "A command proved in Go has no shell twin" (`:1002`) — `debug-on.sh` stays.
- `magento-commands`: "A wrapper runs what it says it runs" gains the `debug-on` failure shapes
  beside `debug-off`'s xdebug-ini scenario (`spec.md:104-109`).
- **No spec documents `hm debug-on` today** (grep over `openspec/specs/`, verified), so Decisions 1-3
  create spec-level behaviour rather than changing it.

## Approach

| Layer | Change |
|---|---|
| `dockergento/dockergento.go` | `Engine.XdebugHost()` — thin facade over `machine.Host{}` |
| `internal/cli/engine.go` | two lines on `commands`; doc comment stays true |
| `internal/cli/fake_engine_test.go` | `outcome.captured`; `Capture` and `XdebugHost` answers |
| `internal/cli/wrappers.go` | the parsers and `debugOn`, beside `debugOff` |
| `internal/cli/run.go` | one `case` |
| `MIGRATION.md` | row + counter |
| `console/`, `bin/run`, `dockergento/ports` | **nothing** |

Strict TDD, RED first, with `answering(t)`'s pins (`HM_NON_INTERACTIVE`,
`DOCKER_HOST=unix:///nonexistent`, `HM_LEGACY_ROOT`) so an unrouted RED reaches neither a daemon nor
the bash bridge.

### Fixtures must discriminate — a verified lesson, not advice

Unit 2 proved a realistic fixture can hide the bug it claims to cover: its `ip addr` fixture listed
`inet` before `inet6`, so a parser wrongly matching `inet6` returned the right address by accident of
ordering and **stayed green under the mutation**. Only a fixture containing *only* the excluded thing
carried the criterion. The three equivalents here are acceptance criteria:

- a `php -v` fixture with **no Xdebug line at all**;
- a `php -i` fixture carrying **both** `xdebug.remote_host` and `xdebug.client_host`, so a v3 parse
  that wrongly accepts a v2 key cannot pass by that key being absent;
- a version string matching `Xdebug v` but neither `v2.` nor `v3.`.

### What proves what

| Claim | Proved by |
|---|---|
| What the handler asks of the engine and in what order: `XdebugHost`, the two host `Exec`s, the uncomment `Exec`, the `CopyFrom` (with `copyFrom`'s extra `Resolve`), the `Restart`, both `Capture`s | `answering(t)` + `fakeEngine`'s ordered log, `cmp.Diff` |
| No `Start` call is recorded (Decision 3) | absence in the same log |
| v2/v3/absent/v4 branching, and the selected ini keys | package fixture tests on the two parsers, no `t.Skip` |
| Exit 6 from an unregistered worktree (Decision 5) | the choice of `Restart`, as `debug-off`'s spec pins the inverse |
| **Not provable here at all** | that the seds land in the real ini, that PHP-FPM picks them up after restart, and what a real PHP image prints. Fixtures come from documented output shapes, not this project's images — stated rather than papered over |

`Capture` attaches without a TTY (`adapters/dockerd/exec.go:81-85`, verified), so the parsers see
plain LF and no CR-stripping is owed.

## Size and slicing

**Independent estimate: ~570-720 authored lines, centre ~640**, spec deltas excluded. Narrower than
the exploration's 550-850 and far above the ~350-500 earlier documents carried.

| Slice | What | Lines |
|---|---|---|
| 1 · parsers | both parsers, discriminating fixtures, tests | ~155 |
| 2 · seam growth | `Engine.XdebugHost` facade, two `commands` lines, `outcome.captured`, two fake answers | ~45 |
| 3 · handler | `debugOn`, `run.go` case, `MIGRATION.md`, seam tests | ~370-440 |

**Which pattern I expect: the overshoot, not the recent good run — and I will not assume otherwise
just because three units in a row landed at or under estimate.** Those three held because decisions
were settled before apply, no spec scenarios arrived late, and seam growth was foreclosed. Here the
seam grows by two plus a new field, and three decisions are open. Settling Decisions 1-3 in this
proposal converts one condition; the third — late spec scenarios — is the one genuinely *not* under
control, because unlike unit 2 this change does have delta specs, and the spec phase will enumerate
a branch matrix (v2 / v3 / absent × mac / linux × text / json) where **every enumerated scenario
becomes a test**. That is exactly where the mass lands: `debug-off`'s five tests are already ~220
lines for a three-call handler. Budget to **~800** and forecast **High**.

**Slicing: I agree with the three boundaries, with two changes.** Order the parsers **first** — they
land alone with full RED/GREEN and need no seam. Slice 2 is then an assertion-only commit whose proof
is that `var _ commands = (*dockergento.Engine)(nil)` (`engine.go:36`) and
`var _ commands = (*fakeEngine)(nil)` (`fake_engine_test.go:90`) still compile; that is unit 1's
precedent and unit 2's proposal already named it a poor thing to ask a reviewer to accept alone, so
it is disclosed rather than hidden. And **Slice 3 needs a contingency split**, because ~440 is over
the 400-line budget on its own: 3a = enabling, up to and including the `Restart`; 3b = the readback
banner, the JSON envelope, the `run.go` case and `MIGRATION.md`. Routing lands in 3b on purpose, so
the command is never routed without the banner it exists to print.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `dockergento/dockergento.go` | New | `Engine.XdebugHost()` facade |
| `internal/cli/engine.go:17-34` | Modified | two seam methods, 16 → 18 |
| `internal/cli/wrappers.go` | New | two parsers + `debugOn` |
| `internal/cli/run.go` | Modified | one `case` |
| `internal/cli/fake_engine_test.go` | Modified | `outcome.captured`, two answers |
| `internal/cli/wrappers_test.go` | New | the seam tests |
| `MIGRATION.md:21`, `:261` | Modified | counter and row |
| `console/commands/debug-on.sh`, `console/tasks/set_xdebug_host_property.sh` | Unchanged | retired in a later cycle |
| `adapters/toolinfo/tooling.go`, `app/transfer.go`, `dockergento/ports` | Unchanged | stated, so the overlap does not read as oversight |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Spec-phase scenario growth pushes Slice 3 past 400 lines | High | 3a/3b split named above; `sdd-tasks` forecasts High |
| A parser passes on a fixture that cannot discriminate | Med | The three adversarial fixtures are success criteria, failing-first |
| Exit 6 from a worktree surprises someone | Med | Disclosed criterion (Decision 5), not discovered after the fact |
| Fixtures come from documented shapes, not this project's images | Med | Stated plainly; manual verification against a real project is the oracle |
| Decisions 1-3 drift into apply unsettled | Med | They are the proposal's question to the user; apply must not start with them open |
| Dropping the implicit `start` surprises a user on a stopped environment | Med | Same disclosure `debug-off` shipped with; the failure names the step |
| A reviewer reads the dropped `\=`/`\;` escapes as transcription slips | Low | Comment in the code plus Decision 7, as `debug-off` did for `\z` |

## Rollback Plan

Three commits on `release/2.0.0` (or four under the 3a/3b split). `git revert` restores the `shell`
row, the counter and the bridge; `console/commands/debug-on.sh` is never touched, so `hm debug-on`
falls straight back through `bin/run` to it. No state is written, no files are created in any project.

## Dependencies

Units 1 and 2, both merged. **No impact on existing projects and no migration**: the arguments, the
container-facing effects and the printed banner are preserved, except for the four disclosed
behaviour changes (Decisions 1, 3, 5 and 8).

## Success Criteria

- [ ] `HM_LEGACY_ROOT=/nonexistent hm debug-on` works — proof it does not reach `bin/run`.
- [ ] The seam test asserts the exact ordered call log, including both `Capture` calls, the
      `CopyFrom` and its extra `Resolve`, and the `sed` argv with **no** `\=` and no `\;`.
- [ ] **No `Start` call is recorded** (Decision 3), and with the environment down `hm debug-on` fails
      with `Exec`'s own status, naming the step.
- [ ] A `php -v` fixture with **no Xdebug line at all** drives the Decision 1 branch, failing first.
- [ ] A `php -i` fixture carrying **both** `xdebug.remote_host` and `xdebug.client_host` proves the v3
      parse selects only the v3 keys, and the v2 parse only the v2 keys.
- [ ] A version string matching `Xdebug v` but neither `v2.` nor `v3.` reaches the same branch.
- [ ] `commands` has **eighteen** methods and its doc comment (`engine.go:14-16`) is unchanged and
      still true; both compile assertions still hold.
- [ ] From an unregistered worktree, `hm debug-on` refuses with code 6 — **new**, disclosed.
- [ ] `MIGRATION.md` says `34 de 65` and the `debug-on` row says `go`;
      `tests/unit/migration_status_test.sh` and `tests/unit/command_safety_test.sh` pass.
- [ ] Against a real project, `hm debug-on` prints the yellow banner with the ini values and
      `hm describe` then reports xdebug on; run twice, the second is a no-op.
- [ ] `go test ./... -short`, `tests/run.sh unit`, `gofmt -l ./cmd ./internal` **and**
      `gofmt -l ./dockergento` empty, `go vet ./...` clean.
