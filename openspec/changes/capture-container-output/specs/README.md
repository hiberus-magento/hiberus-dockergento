# Spec check: `capture-container-output`

No delta specs are written for this change. This file records what was checked and why nothing
qualified, so a future reader can tell "checked, nothing applies" from "nobody looked."

## What was checked

The proposal's own Capabilities section says "New: None. Modified: None," and asks the spec phase
to confirm rather than assume it. Checked against three questions.

### 1. Does any spec document service→container resolution as `db`-specific?

Read every capability spec whose text mentions a container, a service, or a refusal tied to one
not running: `database-access`, `database-clients`, `container-inventory`, `environment-access`,
`environment-orchestration`, `go-entrypoint`, `magento-commands`.

None of them describes the resolution mechanism at all — none says "the container is found by
hardcoding `db`" or anything an implementation-level reader could point to as contradicted by a
generalised resolver. Every one states only the **observable refusal**, at the capability's own
scope:

- `database-access` — "the database container is not running" → "refused with the service exit
  code, saying how to start it" (Requirement: Asking the database).
- `database-clients` — "the database service is not running" → "the command says so with the
  service exit code."
- `environment-orchestration` (Reading the logs) — "a service this project does not have" → same
  shape.
- `environment-access` (`logs`) — same shape, for an unnamed service.

Each is scoped to its own capability (the database, a client launcher, logs) and stays true
whether the resolution behind it is `Database`'s own inline code or a shared helper `Database`
delegates to — the proposal's Decision 1 is exactly that the observable refusal is unchanged.
Nothing here becomes false. No delta is warranted under this question.

### 2. Does any spec pin `Database.container`'s refusal (code 5 + hint) so it must be restated?

`database-access`'s "The database is not running" scenario is that pin: refusal with the service
exit code and a hint on how to start it. It already states the behaviour this change is required
to preserve byte-for-byte (proposal Decision 1, proven by `dockergento/app/database_test.go`'s
five tests passing unedited). Because the change does not alter that text's truth — the refusal,
the code, and the hint stay exactly what they are — there is nothing to restate as a MODIFIED
requirement. A MODIFIED block exists to carry forward text that would otherwise be overwritten or
lost; this text is neither.

The literal number 5 and the exact hint wording are not pinned for *this* refusal anywhere in the
spec layer: the capability specs that describe it — `database-access`, `database-clients` — both use
the abstract phrase "the service exit code." So for `Database.container`'s refusal, the numeric
code and the exact hint text live only in `dockergento/app/database_test.go` and the code itself.

**Corrected before archiving.** This paragraph first claimed something wider and false — that *no*
spec in the repo ever names a literal exit code. `openspec/specs/go-entrypoint/spec.md:678` and
`:689` both name `exit 5` outright, in service-refusal scenarios (`logs nonexistent`, the
`unknown_service` branch). That does not change the conclusion: those scenarios describe `logs`'s
branch and its test routing, neither of which this change touches. It is recorded because the false
sentence was the reinforcing kind — it sounded right precisely because the narrower claim around it
is right, which is exactly why it would never have been re-read.

### 3. Does `Capture` deserve a spec at this layer, in this unit?

No. Every full spec under `openspec/specs/` describes a user-facing capability: a command, its
text/`--json` output, its exit codes, its refusals. `go-entrypoint`'s own seam requirement — "A
ported command's engine interaction is provable without Docker" — is scoped explicitly to "a
command listed in `internal/cli/run.go`'s `Run()` switch." `Capture` is not: per the proposal's
Decision 3, it joins `commands` and gets a handler only in unit 3. Until then it has no command,
no output, no exit-code mapping, and nothing in `internal/cli` calls it — there is no user-visible
surface for a spec to describe, and no existing spec's scope reaches it either.

This mirrors the proposal's own reasoning (Decision 3, Non-goals) and the "What proves what" table:
everything this unit ships is proved by `dockergento/app` tests against its own fakes, not by a
capability-level scenario. Writing a spec now would describe an internal method nothing calls,
which the repo's spec layer does not do for any other engine method without a caller (`Diagnose`,
`Console`, `Templates`, `Freeze`, `RegistryState` all have callers; `Capture` does not, yet).

## Conclusion

No new capability, no modified capability. `openspec/specs/database-access/spec.md` already states
the observable refusal this change must preserve, and preserves it unedited; the exact code/hint
text for that refusal is recorded only in `dockergento/app/database_test.go`, not at the spec level
— a pre-existing property of how these capability specs are written, not a gap this change opens. `Capture` gets a spec
when unit 3 gives it a caller and a command to describe.

## Mismatch check against the proposal

The proposal's `magento-commands` citation — "the command exits with that same status, not a
mapped code" — was verified against `openspec/specs/magento-commands/spec.md`; the phrase appears
verbatim in the varnish and `debug-off` non-zero-status scenarios. No mismatch found between the
proposal and the main specs it cites or relies on.
