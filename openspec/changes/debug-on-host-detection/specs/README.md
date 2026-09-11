# Spec check: `debug-on-host-detection`

No delta specs are written for this change. This file records what was checked and why nothing
qualified, so a future reader can tell "checked, nothing applies" from "nobody looked" — the same
discipline `capture-container-output` (unit 1) applied to `Capture`.

## What was checked

The proposal's own Capabilities section says "New: None. Modified: None," and asks the spec phase
to confirm rather than assume it.

### 1. Does any capability spec describe `hm debug-on`, xdebug, or host detection?

Searched every full spec under `openspec/specs/` for `debug-on`, `xdebug`, `docker0`, `WSL`, and
`host.docker.internal` (case-insensitive). Three files match:

- `openspec/specs/magento-commands/spec.md:64-112` — "Scenario: The xdebug ini edit fails inside
  the php container." This is **`debug-off`**, the sibling command that turns xdebug off, not
  `debug-on`. It describes the `sed`/failure shape of clearing `zend_extension`, nothing about
  finding a host address.
- `openspec/specs/go-entrypoint/spec.md:322-413` — "Turning xdebug off" / "Turning xdebug off stops
  at a failing edit" / "Disabling xdebug in JSON." Same command, same scope: three engine calls
  (`Resolve`, `Exec` the `sed`, `Compose restart`), the failure shape, and the `--json` key. Nothing
  here concerns how a host address is found, because `debug-off` never looks for one.
- `openspec/specs/environment-introspection/spec.md:10-16` — `hm describe` reports "el estado de
  Xdebug" (Xdebug's status). This is a boolean read of whether xdebug is on, surfaced via
  `Tooling.Xdebug` in `internal/cli/describe.go`, not a host-address lookup.

No spec anywhere mentions `docker0`, WSL, `/proc/version`, `ip addr`, `ifconfig`, or a fixed xdebug
host address. **No spec documents `hm debug-on`'s current host-detection behaviour, ported or
otherwise** — confirming the premise the proposal asked this phase to check rather than assume.

Also checked `internal/cli` directly for the same terms: `wrappers.go` and `describe.go` match on
`xdebug`, and both are the same two surfaces above (`debug-off`'s handler, `describe`'s status
read). Neither reaches host detection or `debug-on`.

### 2. Does `go-entrypoint`'s seam requirement reach an adapter method with no handler?

"Requirement: A ported command's engine interaction is provable without Docker"
(`go-entrypoint/spec.md:253-263`) is scoped explicitly: **"For a command listed in
`internal/cli/run.go`'s `Run()` switch."** Verified against `internal/cli/run.go` directly: no
`case "debug-on":` exists there. Verified against `MIGRATION.md:261`: `debug-on` is still listed
`shell`, not `go`.

Unit 1's conclusion (`Capture` has no seam entry because it has no command) holds here for the same
reason, one layer further removed: this unit does not even add a caller inside `dockergento` for
its new method, let alone a `case` in `run.go`. `Host.XdebugHost` is called by nothing after this
change lands (proposal Non-goals, `internal/cli`: nothing). The requirement's own scope —
commands in the switch — cannot reach a method with no command, no handler, and no switch entry at
all. Confirmed, not merely inherited from unit 1's precedent.

### 3. Do Decision 2 or Decision 4 belong in a spec now?

Both are real behaviour decisions, settled by the user, and both are recorded permanently in
`proposal.md` (Decisions 2 and 4). The question is whether either also needs a spec-level MODIFIED
requirement in **this** unit.

**No, for both, and for the same reason question 1 already establishes**: there is no existing spec
text describing `hm debug-on`'s host-detection behaviour to modify, and no new user-facing surface
in this unit to describe as a MODIFIED or ADDED requirement. Every scenario in every spec under
`openspec/specs/` is phrased as an observable command interaction — a `GIVEN`/`WHEN` naming a
command or a fake-engine call, a `THEN` naming stdout, stderr, an exit code, or a `--json` document.
`Host.XdebugHost` has none of those yet: no command runs it, so there is nothing for a `WHEN` to
name and nothing for a `THEN` to observe. Writing a spec scenario now would have to describe the
adapter method directly ("WHEN `XdebugHost` is called on Linux with no resolvable `docker0`, THEN
it returns `ErrNoBridgeAddress`"), which is not the shape this repository's spec layer uses anywhere
— it is exactly the internal-method-with-no-caller case unit 1's report already declined for
`Capture`, one layer further removed since this unit doesn't even add a caller.

**Decision 2 (unresolvable `docker0` is a failure) is not observable until unit 3's handler exits
non-zero.** Nothing a user can run today would exercise it either way; it is proved in this unit
only by `Host.XdebugHost`'s own (untested, by design — matching `Listening()`) I/O path and by the
sentinel `ErrNoBridgeAddress`'s existence, neither of which is a spec-level fact.

**Decision 4 (WSL2 pattern tested first) deserves the closer look the task asked for, because it is
a deliberate divergence from the shell, not a transcription** — and divergences are usually exactly
what a spec is for. Weighed against that: divergence from the *shell* is not divergence from any
*spec*, because no spec states the shell's check order to begin with (question 1). A spec exists to
pin an observable contract so it does not drift by accident; recording the parser's internal
ordering now would pin an implementation detail of a function nothing calls, before unit 3 gives it
a `WHEN`. The right place to record it as a **behavioural** fact — one a user could notice, i.e.
"a WSL2 machine gets `host.docker.internal`, including one whose kernel string names `Microsoft`
somewhere" — is unit 3's spec, once `debug-on` actually runs and that outcome is something a
`THEN` can assert. Recording it only in `proposal.md` for now does not lose it: Decision 4 is
already permanent there, with its own rationale, and `tasks.md` and unit 3's exploration are what
carry it forward, the same way Decision 2 does.

**Conclusion for both**: defer to unit 3's spec, where each becomes a `THEN` a real command
produces. Flagged explicitly here so unit 3's spec phase does not have to rediscover either
decision from `git log`.

## Conclusion

No new capability, no modified capability. No spec anywhere describes `hm debug-on`'s
host-detection behaviour — current shell behaviour or otherwise — so there is no MODIFIED
requirement text this change would alter, restate, or contradict. `go-entrypoint`'s seam
requirement is scoped to commands in `internal/cli/run.go`'s switch and `debug-on` has no entry
there (verified against `run.go` and `MIGRATION.md:261`), so it does not reach `Host.XdebugHost`
either. Decisions 2 and 4 are real and permanently recorded in `proposal.md`, but neither has a
`WHEN` for a spec to hang a `THEN` on until unit 3 gives `debug-on` a handler. `Host.XdebugHost`
gets a spec, and Decisions 2 and 4 get their scenarios, when unit 3 gives it a caller and a command
to describe.

## Claims checked against source during this phase

- **magento-commands and go-entrypoint's xdebug scenarios are `debug-off`, not `debug-on`** —
  confirmed by reading both cited ranges; neither mentions a host address, `docker0`, or WSL.
- **`environment-introspection`'s Xdebug mention is `describe`'s status read, not host
  detection** — confirmed by reading `environment-introspection/spec.md:10-16` and cross-checking
  against `internal/cli/describe.go`, which calls `Tooling.Xdebug`, unrelated to
  `machine.Host.XdebugHost`.
- **No `case "debug-on":` in `internal/cli/run.go`** — confirmed directly; grep for `debug-on` in
  that file returns nothing.
- **`MIGRATION.md:261` still lists `debug-on` as `shell`** — confirmed by reading the line.
- **`ports/ports.go:150` does declare `ResolvesLocally(domain string) bool`** — confirmed by
  reading the interface; this supports the proposal's own correction of the exploration's
  overstatement (the interface entry exists; the precedent is narrower than "no interface entry"
  implied).
- **`dockergento.go:860` calls `(machine.Host{}).ResolvesLocally(...)` directly, bypassing the
  interface** — confirmed by reading the line; this is the precedent Decision 1 actually rests on.
- **`Platform()` is at `dockergento.go:1376`, not `:1361`** — confirmed by reading the line; the
  proposal's own citation is current as of this phase, though it is expected to drift again before
  unit 3 (Risks table).
- No spec anywhere mentions `docker0`, WSL, `/proc/version`, `host.docker.internal`, or `ip
  addr`/`ifconfig` — confirmed by a case-insensitive search across all of `openspec/specs/`.
