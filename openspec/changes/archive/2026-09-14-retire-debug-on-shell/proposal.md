# Proposal: The third step, for `debug-on`

Change: `retire-debug-on-shell`
Engram topic: `sdd/retire-debug-on-shell/proposal` (project `hiberus-dockergento`)
Date: 2026-09-13
Branch: `release/2.0.0`
Exploration: `openspec/changes/retire-debug-on-shell/exploration.md`

## Intent

`MIGRATION.md:62-65` states the rule for this port in three steps — a command ported, its Go test
written, then the shell implementation and its bash test deleted, in that order. `hm debug-on` has
had the first two since earlier today (`openspec/changes/archive/2026-09-13-debug-on-handler/`).
This cycle is the third step, and nothing else.

Two files come out: `console/commands/debug-on.sh` (38 lines) and
`console/tasks/set_xdebug_host_property.sh` (19). `internal/cli/run.go:126` already routes
`debug-on` to the Go handler, so the shell files are unreached by `hm debug-on` today; they are
dead weight that two subsequent cycles have had to read, reason about and carry forward.

**Why now**, beyond tidiness: `architecture/05-state-and-configuration.md` describes an xdebug
host-detection algorithm that neither implementation ever had, and has been carried forward unfixed
by two cycles already. Deleting the file it names by hand without correcting it would leave a
document that is both wrong about the behaviour *and* wrong about the file's existence. The
correction has a natural home exactly once, and this is it.

**Success looks like**: both files gone; `hm debug-on` unchanged in behaviour because the binary
already owns it; `bin/run debug-on` refusing with `command_moved_to_binary` and exit 2; `hm --help`
still listing `debug-on`; the architecture document describing what the Go code actually does; and
the full suite in the same state it was in before the cycle started.

## The evidence, and what it is worth

### The experiment was run, not reasoned

The previous retirement cycle's durable finding
(`openspec/changes/archive/2026-09-11-retire-read-only-shell-commands/archive-report.md:41-59`) is
that three successive textual checks each looked sufficient and each was insufficient, and that only
moving the files aside and running everything found the gaps. The orchestrator ran that experiment
for this cycle before this proposal was written.

Both files were removed, `./tests/run.sh` and `go test ./...` were run, and the result was
**identical to baseline**: 59 of 1003 assertions failing in both, the same seven suites
(`cli_contract`, `concurrent_safety`, `go_engine`, `go_passthrough`, `logs_launch_version`,
`tui_dashboard`, `worktree`), and a line-by-line diff of the two full outputs — paths and timings
filtered — that is **empty**.

**Method, and why it is shaped this way.** The full suite includes integration suites that reach a
Docker daemon, and no test in this cycle may reach the live one: the user has unrelated projects
running. Both runs therefore pinned `DOCKER_HOST` at a nonexistent socket, so the daemon was
unreachable *identically in both* and the daemon noise cancels. The evidence is the difference
between the two runs, not either run's absolute figure. All 59 failures in both are that pinned
daemon (`FAIL docker-daemon Docker daemon is not running`) and a synthetic-project artefact
(`The Magento directory './src' does not exist`).

**Restoration** was by copying backups back over the files — never `git checkout <file>`, which in
this repository has already silently reverted an entire uncommitted unit and left `git status`
clean. Byte-identity confirmed by `md5`.

### The corollary, stated rather than enjoyed

The clean result is a materially different shape from the previous retirement, where moving three
files aside broke twelve assertions across eight suites. That difference is not a compliment to this
cycle. **If removing the entire shell implementation breaks nothing, nothing was testing it.** The
green is the absence of coverage, not the presence of proof. `debug-on`'s shell half went
unexercised by 1003 assertions.

And the caution the previous cycle's verifier wrote about itself applies here unchanged: everything
being green with the files gone proves that nothing we run fails. **It does not prove there is
nothing left to run.**

### What the retirement stops proving: nothing

Stated as an explicit "none" rather than omitted, following the previous cycle's convention of
writing the empty answer down.

There is no parity oracle between the shell and Go halves of `debug-on`, because none ever existed.
`internal/cli/wrappers_test.go` proves the Go handler against `fakeEngine`, never against the
script's real output, and no integration or e2e test touches the command — xdebug needs a live
container. So no comparison retires here. This is the same fact as the corollary above, seen from
the other side.

## Correction to the exploration

Three findings against source. The first changes the delivery plan.

### 1. Both "convention additions" are deletion-gated — the recommended commit order is wrong

The exploration recommends "the additions first, with both files still present so every suite stays
green throughout." Verified against source, **neither addition can precede the deletion**:

- `tests/unit/command_safety_test.sh:76` is
  `[ -f "$COMMANDS_DIR/$name.sh" ] && drift="$drift ${name}(file)"`, and `:79` is
  `assert_empty "$drift"`. Adding `debug-on` to `RETIRED` while `debug-on.sh` still exists makes that
  assertion report `debug-on(file)` and go **red**. This is not a guess: the previous cycle's
  archive report (`:78-80`) records exactly this, having mutation-checked all three of its own
  deletions by restoring the `.sh` and watching the suite report `list(file) describe(file)
  logs(file)`.
- `tests/integration/cli_contract_test.sh` asserts exit 2 and
  `.error.type == "command_moved_to_binary"`, which `bin/run:72-81`'s `check_command_dir` produces
  **only when the file is absent** and the name is declared in `command_descriptions.json`. With
  `debug-on.sh` present, `run debug-on` would not refuse — it would execute the script, from the
  temp non-project directory the suite runs in, reaching `start_service_if_not_running.sh`. Red, and
  a live-daemon touch this cycle forbids.

The existing precedent says the same thing in its own words. `cli_contract_test.sh:73-76`:
*"`set-host` stood in for the twelve retired twins in unit 2: it was the one whose shell file went in
that same unit, so it was the only one that could turn this RED into GREEN without waiting on a later
unit. `version` is added here, in unit 4, now that `console/commands/version.sh` is actually gone."*
And `MIGRATION.md:186-188` records the `RETIRED` entries for `list`/`describe`/`logs` being added
*"en el mismo commit que borra los ficheros."*

So the split below puts the deletion and its two assertions in one commit. They are one unit, not
two: each is red until the other lands.

### 2. A third citation site the exploration missed: the live spec

Exploration §F enumerates the dangling `debug-on.sh:NN` comments in `internal/cli/wrappers.go` and
`wrappers_test.go`. It omits `openspec/specs/go-entrypoint/spec.md`, which cites the two files at
three places — `:466`, `:570` and `:606`. `:606` is not provenance but a **premise**:

> **GIVEN** nothing in this repository sets `XDEBUG_HOST` except `set_xdebug_host_property.sh`
> itself (verified, unchanged by this port) — a shell-only escape hatch this port drops

After deletion nothing in the repository sets `XDEBUG_HOST` at all, so the GIVEN becomes vacuous and
its parenthetical ("unchanged by this port") refers to a port that is no longer the latest one. The
scenario's `WHEN`, `THEN` and test (`TestDebugOnIgnoresXdebugHostEnvironmentVariable`) are
unaffected — if anything the conclusion holds more strongly. Handled under "Dangling citations"
below.

### 3. Two range and count details

- §G cites `architecture/05-state-and-configuration.md:364-384`. Section 5.8.1 actually runs
  `:364-389`; the range stops four lines short and excludes the `hm debug-off` half of the
  activation snippet at `:386-388`, which is wrong in the same way. The correction must cover
  `:364-389`.
- §H estimates the `cli_contract_test.sh` pair at 15-20 lines. The `version` precedent at `:87-94`
  is 8 lines for a pair. With a comment, 10-14.

Everything else in the exploration held. Spot-checked and confirmed: `debug-on.sh` is 38 lines and
`set_xdebug_host_property.sh` 19, both read in full; `set_xdebug_host_property.sh` is sourced at
`debug-on.sh:11` and nowhere else in the repository; `RETIRED` at `command_safety_test.sh:69-71` is
exactly 15 names; `debug-on` is declared at `command_descriptions.json:244-247` and routed at
`run.go:126`; `print_help.sh:54-59` unions declared names lacking a file; `tui.sh` never mentions
`debug-on`; `skills/dockergento-debugging/SKILL.md:24` has `hm debug-on` and
`skills_test.sh:75-86` checks file-**or**-route, so it passes unmodified; `MIGRATION.md:261` tables
`debug-on` as `go`; and §G's three divergences are each exactly as described.

## Scope

### In Scope

1. **Delete `console/commands/debug-on.sh`** (38 lines). No retirement mechanism to add:
   `bin/run:72-81`'s `check_command_dir` answers `command_moved_to_binary` with `$HM_EXIT_USAGE`
   for any name declared in `command_descriptions.json` that has no file. The absence *is* the
   mechanism.
2. **Delete `console/tasks/set_xdebug_host_property.sh`** (19 lines). Tasks are never dispatched
   and this one is sourced from exactly one place, the file above. It needs no mechanism at all; it
   disappears as a side effect.
3. **`tests/unit/command_safety_test.sh:69-71`** — add `debug-on` to the hand-written `RETIRED`
   array, 15 names to 16. No assertion requires this: the array is self-checked, not derived. It is
   the convention, established twice. While editing those exact lines, the stale "over the twelve
   retired by name" in the comment at `:64` becomes "the sixteen" — it was already wrong at 15.
4. **`tests/integration/cli_contract_test.sh`** — one "and so does `debug-on`, once its own shell
   twin is gone" pair, following the `set-host`/`version`/`describe` precedent at `:78-103`: exit 2
   with `.error.type == "command_moved_to_binary"`, then `--no-json` naming `hm debug-on`.
5. **Correct `architecture/05-state-and-configuration.md`** — the table row at `:207` and section
   5.8.1 at `:364-389`, rewritten to match `dockergento/adapters/machine/xdebug_host.go` and
   `internal/cli/wrappers.go`'s `debugOn`, and saying the shell script is gone. Detailed below.
6. **`MIGRATION.md`** — one third-step note beside the existing ones at `:157-188`. Not required by
   `migration_status_test.sh`, which is driven generically; taken because that list is where this
   repository records which commands have been retired and it is the one canonical place a reader
   hitting a dangling `debug-on.sh` citation can be sent.

### Out of Scope

Named so a reviewer does not go hunting.

- **`data/command_descriptions.json` is not touched.** `debug-on` must stay declared:
  `print_help.sh:54-59` unions declared names that have no file into `hm --help`, which is precisely
  how `list`, `describe` and `logs` stayed listed after their own retirement. Removing the entry
  would remove the command from help *and* turn `bin/run debug-on` from
  `command_moved_to_binary` into `command_not_found`.
- **`console/commands/debug-off.sh` and `exec.sh` are not retired.** `debug-off` has a Go handler
  but its own retirement cycle; `exec.sh` still has live callers and, per
  `openspec/changes/archive/2026-09-11-retire-read-only-shell-commands/archive-report.md:105-106`,
  no Go test at all.
- **`toolinfo/tooling.go:84-85`** — the empty-project-name container match, carried forward from the
  previous cycle with its own backlog entry.
- **`transfer.go`'s third resolution copy**, the **`tests/run.sh` silent fallthrough**, the **CI
  `gofmt` gap**, and **CI's missing macOS coverage**. All pre-existing, none touched by this cycle,
  none made worse by it.
- **No source code is written or modified.** `internal/cli/` and `dockergento/` are unchanged except
  for the comment decision below, which this proposal declines. The Go implementation was proved in
  its own cycle; this one only removes what it superseded.

## The architecture document correction

`architecture/05-state-and-configuration.md` describes an algorithm **neither implementation ever
had**. Three divergences, each verified against both:

| The doc says | The shell did (`set_xdebug_host_property.sh`) | Go does (`xdebug_host.go`) |
|---|---|---|
| `:374` reads the **default gateway**: `/sbin/ip route \| awk '/default/ { print $3 }'` | `:12` reads **`docker0`'s own inet address**: `ip addr show docker0 \| grep "inet\b" \| awk '{print $2}' \| cut -d/ -f1`, falling back to `ifconfig docker0` at `:14` | `hostFromIPAddr` / `hostFromIfconfig`, same two sources, same `inet`-not-`inet6` discrimination |
| `:370-371` gives macOS `XDEBUG_HOST="host.docker.internal"` | `:4-5` leaves it **empty** — the whole block is `if [ "$MACHINE" == "linux" ]` | `XdebugHost()` returns `("", nil)` outside Linux: not applicable, nothing to patch |
| `:383`/`:387` activates by **renaming** `docker-php-ext-xdebug.ini.disabled` | `debug-on.sh:19` runs `sed -i -e 's/^\;zend_extension/zend_extension/g'` — a comment/uncomment, no rename | `debugOn` runs the same `sed`, minus the shell's stray inert `\;` |

The doc's snippet also has **no WSL branches at all**, where both implementations have two
(`/proc/version` matching `Microsoft` → `10.0.75.1`, `microsoft-standard` → `host.docker.internal`),
and the table row at `:207` gives `XDEBUG_HOST`'s example value as `"host.docker.internal"`, which
is the WSL2 answer presented as the general one.

The correction rewrites `:207` and `:364-389` against the Go implementation, notes the two
behaviours Go has that the shell did not — `ErrNoBridgeAddress` for a `docker0` that exists but
cannot be resolved, and the deliberate `microsoft-standard`-before-`Microsoft` ordering
(`xdebug_host.go:57-60`) — and states that the shell script is gone.

**Ordering matters here.** The previous cycle's verification flagged, as its one non-blocking
warning, a comment added two commits before the change that made it true — "the same *documentation
claims a fact before the commit that makes it true* shape this cycle was repairing elsewhere." This
correction says the script is gone, so it lands after the commit that removes it, not before.

## Dangling citations: the decision, taken deliberately

Nine comment sites cite `debug-on.sh:NN` as the provenance of a ported behaviour —
`internal/cli/wrappers.go:376, :381, :468, :472, :474, :496, :519` and
`internal/cli/wrappers_test.go:1476, :1484` — plus three in `openspec/specs/go-entrypoint/spec.md`
(`:466, :570, :606`). All twelve verified at those exact lines. Nothing reads them; they break no
test. After deletion they cite a path that no longer resolves, though the content is still in
history.

**Recommendation: leave all twelve, and record the fact once in `MIGRATION.md` rather than twelve
times in the source.** The reasoning, including the part that cuts the other way:

- The line numbers are the *point*. `wrappers.go:376` explains forty-eight dashes by saying the
  shell had forty-eight, at `debug-on.sh:30` and `:38`. A reader who doubts it can run
  `git show <sha>:console/commands/debug-on.sh` and land on the line. Stripping the numbers destroys
  the only thing that makes the claim checkable; rewriting each to "in git history, see …" adds
  twelve edits of noise to buy back what one line elsewhere buys.
- The comments remain **true**. They assert what the shell did, which deletion does not falsify.
  This is the same "true when written" principle the previous cycle applied to its own stale comment
  and declined to amend, and the same reason it did not edit the analogous `list`/`describe`/`logs`
  comments.
- **The cost, stated rather than hidden**: a reader's first instinct on seeing `debug-on.sh:27` is
  to open the file, and that now fails. Twelve small dead ends is a real price. It is paid once,
  cheaply, by `MIGRATION.md`'s third-step note naming both deleted files, which is where this
  repository already records retirements and where a puzzled reader will end up.
- **The spec's `:606` is the weakest case for leaving**, because it is a premise rather than
  provenance, and after deletion it is vacuous. It stays anyway: `openspec/specs/` is generated from
  archived deltas, so touching it directly is a spec change requiring its own delta — disproportionate
  for a GIVEN whose conclusion the deletion strengthens rather than weakens.

The point of writing this down is that it be **accepted** here rather than discovered by whoever next
follows one of those citations into nothing.

## Affected Areas

| Path | Change | Note |
|---|---|---|
| `console/commands/debug-on.sh` | **Deleted** | 38 lines; `bin/run` refuses by absence |
| `console/tasks/set_xdebug_host_property.sh` | **Deleted** | 19 lines; sourced only from the above |
| `tests/unit/command_safety_test.sh` | Modified | `RETIRED` 15 → 16; the `:64` comment's count |
| `tests/integration/cli_contract_test.sh` | Modified | one `debug-on` refusal pair |
| `architecture/05-state-and-configuration.md` | Modified | `:207` and §5.8.1 `:364-389` |
| `MIGRATION.md` | Modified | third-step note; the canonical home for the dangling citations |
| `data/command_descriptions.json` | **Unchanged** | stated, because the entry must survive |
| `internal/cli/wrappers.go`, `wrappers_test.go` | **Unchanged** | dangling citations kept, deliberately |
| `openspec/specs/go-entrypoint/spec.md` | **Unchanged** | `:606`'s GIVEN goes vacuous, deliberately |
| `console/commands/debug-off.sh`, `exec.sh` | **Unchanged** | their own cycles |
| `tests/unit/skills_test.sh`, `tui_actions_test.sh`, `migration_status_test.sh` | **Unchanged** | each checked, each already file-or-route |

## Size and delivery

### Size

**Estimate: 115-145 changed lines**, counting additions and deletions the way this repository's
archive reports do.

| Unit | Lines |
|---|---|
| The two deletions | 57 |
| `RETIRED` entry + the `:64` comment count (rewrap) | 4-6 |
| `cli_contract_test.sh` pair, with comment | 10-14 |
| `architecture/05` §5.8.1 rewrite (≈24 removed + ≈28 added) + table row | ≈54 |
| `MIGRATION.md` third-step note | 10-14 |

This is **higher than the exploration's 75-110**, and the difference is arithmetic rather than
pessimism: §H priced the doc fix at "15-25 lines" while the block being replaced is itself 24 lines,
counting the rewrite's additions and not its deletions. Everything else in §H holds.

**Which historical pattern applies, and why.** This repository's overshoot is 1.3-2x, most recently
`debug-on-handler` at 1246 against 850-950. That overshoot has a cause, not a mood: that cycle grew a
new seam method, carried open decisions into implementation, and had to write spec scenarios for
behaviour that did not exist yet. **None of those apply here.** This cycle has no open decisions —
the doc-fix question §G left open is settled as in-scope above, and the dangling-citation question is
settled as declined. It grows no seam, adds no method to the `commands` interface, and writes no new
Go. Most decisively, the experiment already established that **the repairs required are none**: the
1.3-2x overshoots on this repository have consistently come from unplanned repair work discovered
during implementation, and the move-aside run is precisely the instrument that converts that unknown
into a measurement. I expect this one to land **at or under 145**, with the three cycles that landed
at or under estimate today as the applicable precedent.

The residual risk is the doc rewrite, the one unit with no mechanical bound — a correction can always
grow into an essay. Bounded at 60 lines for §5.8.1; past that, trim rather than exceed.

### Commit split

**Two commits.** The boundary is not the exploration's, for the reason given in Correction 1.

**Commit 1 — the deletion and the two assertions that only pass once it lands.**
`console/commands/debug-on.sh` and `console/tasks/set_xdebug_host_property.sh` deleted; `debug-on`
added to `RETIRED`; the `cli_contract_test.sh` pair added. ≈75 lines. These four edits are one unit:
each addition is red while the files exist and green the moment they are gone, which is also the
mutation check — reverting either deletion turns both assertions red, exactly as the previous cycle
verified for its own three.

**Commit 2 — the documentation that commit 1 made true.** `architecture/05-state-and-configuration.md`
and `MIGRATION.md`. ≈65 lines. Strictly after commit 1, because it asserts the script is gone, and
because claiming a fact before the commit that makes it true is the exact shape the previous cycle
flagged as its own warning.

Both commits leave the suite in its baseline state. Commit 1 is the only one that touches anything
executable.

## Risks

- **§A's corollary is the finding, not the deletion.** Nothing about removing these two files is
  risky. What the clean result reveals — that an entire shell command went unexercised by 1003
  assertions for as long as it existed — is the part worth a moment's thought, and it is not
  something this cycle can fix.
- **Green with the files gone proves nothing we run fails. It does not prove there is nothing left
  to run.** No macOS coverage exists in CI, and `debug-on.sh:21-24`'s macOS branch was therefore
  never exercised by any run, baseline or treatment. The Go handler's equivalent
  (`wrappers.go:334-342`) has its own unit coverage, so the deletion does not reduce what is proved;
  it is named because the experiment's reach stops there.
- **The doc correction is unverifiable by test.** No assertion enforces documentation, which is how
  `:364-389` survived being wrong through at least three cycles. Correcting it depends entirely on
  reading `xdebug_host.go` and `debugOn` carefully, and nothing will catch a new error in it.
- **Twelve citations will dangle by choice.** Mitigated to one lookup by the `MIGRATION.md` note, not
  eliminated. If a reviewer prefers them rewritten, that is a scope change to take now rather than
  a surprise later — which is why it is written down.
- **`openspec/specs/go-entrypoint/spec.md:606` keeps a vacuous GIVEN.** Harmless today; it becomes a
  small trap if someone later reads it as evidence that a shell escape hatch still exists.

## Rollback Plan

Both commits are pure deletions and documentation. `git revert` of commit 1 restores both files
byte-for-byte and returns `RETIRED` to 15 and `cli_contract_test.sh` to its current pairs; the suite
returns to baseline. Commit 2 reverts independently of commit 1. Nothing in `internal/cli/` or
`dockergento/` changes, so no Go behaviour can regress in either direction.

## Dependencies

- `openspec/changes/archive/2026-09-13-debug-on-handler/` — the port and its proof. Complete.
- No Docker daemon. Every assertion in this cycle refuses before reaching one: exit 2 from
  `check_command_dir` precedes any project or Compose work, which is why the existing
  `set-host`/`version`/`describe` pairs pass from a temp non-project directory.

## Success Criteria

1. `console/commands/debug-on.sh` and `console/tasks/set_xdebug_host_property.sh` do not exist.
2. `bin/run debug-on` exits 2 with `.error.type == "command_moved_to_binary"`, and `--no-json`
   names `hm debug-on`; asserted by the new `cli_contract_test.sh` pair.
3. `hm debug-on` behaves exactly as it did before this cycle — the binary already owned it.
4. `hm --help` still lists `debug-on`.
5. `command_safety_test.sh` reports no drift with 16 names in `RETIRED`, and reverting either
   deletion makes it report `debug-on(file)`.
6. `./tests/run.sh` and `go test ./...` are in the same state as the recorded baseline, by diff and
   not by impression.
7. `architecture/05-state-and-configuration.md` §5.8.1 describes `xdebug_host.go` and `debugOn`, and
   says the shell script is gone.
8. `gofmt` and `go vet` clean — unchanged, since no Go is written.
