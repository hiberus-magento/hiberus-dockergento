# Exploration: wrapper-exit-codes

Change: `wrapper-exit-codes`
Engram topic: `sdd/wrapper-exit-codes/explore` (project `hiberus-dockergento`)
Date: 2026-09-09

`hm varnish-on` and `hm varnish-off` answer 0 when the command they ran inside the container failed.
They are the only two commands in the tool that do this.

## The defect

`internal/cli/wrappers.go:153-167`, inside `varnish()`, twice:

```go
status, err := engine.Exec(...)
if err != nil || status != 0 {
    return report(stderr, jsonOutput, command, err)
}
```

`report()` (`internal/cli/orchestrate.go:224-227`) turns an *error* into an exit code, and answers
`exitOK` when there is none. It has no idea what a container exit status is and was never meant to
receive one. So a container command that exits non-zero without a Go-level error arrives as a nil
error, and the answer is success.

## What the reader actually gets

The early return happens **before** the writing, which makes this worse than a wrong exit code:

| | `hm varnish-on` today, when the inner command fails |
|---|---|
| Exit code | `0` |
| Text output | nothing — the return is above `fmt.Fprint(stdout, good("Varnish cache enabled!\n"))` at `:190` |
| `--json` output | nothing — the return is above `document(...)` at `:186` |

So the tool does not lie in words; it says nothing at all and calls it success. Under `--json` it
emits no document, which is the one thing every command in this tool is supposed to do.

## What it costs beyond the code

`varnish-off` never reaches its own cleanup. The block at `:172-181` exists because turning Varnish
off leaves the pages it cached behind, so it runs `purge` and `magento cache:clean`. A failed
`cache:disable` returns above that block: Varnish is off, the stale pages it cached are still being
served, and the caller was told it worked.

## The convention this breaks

Four places in the same package already do it right — status first, raw, and `report()` only for a
Go-level error:

| Where | Shape |
|---|---|
| `internal/cli/php.go:62-66` `inside()` | `if status != 0 { return status }; return report(..., err)` |
| `internal/cli/orchestrate.go:188-194` `execute()` | the same |
| `internal/cli/docker_compose.go:10-21` | `if err != nil { return report(...) }; return status` |
| `internal/cli/mysql.go:67-99` | the same, three times |

`inside()` is what `npm`, `n98-magerun`, `test-unit`, `test-integration`, `purge`, `magento` and
`composer` all run through, and `tests/integration/go_wrappers_test.sh` proves it: a recorder
exiting 42 makes `hm npm` exit 42. The healthy path is already tested; `varnish()` is the one
function that collapses both questions into one guard and then throws `status` away.

Exit codes are defined once in `dockergento/contract/contract.go` and re-exported in
`internal/cli/output.go`: `ExitOK=0, ExitError=1, ExitUsage=2, ExitDocker=3, ExitProject=4,
ExitService=5, ExitBlocked=6`. The convention reads: *the container ran and failed* answers with the
container's own code; *we could not run it* answers with one of these.

## The shell precedent, from the bytes

`git show 583fb0c^:console/commands/varnish-on.sh` — the twin deleted yesterday — opens with
`set -euo pipefail` and then:

```bash
$DOCKER_COMPOSE exec -uroot varnish sed -i '...' /etc/varnish/default.vcl
$DOCKER_COMPOSE restart varnish
$DOCKER_COMPOSE exec phpfpm bin/magento cache:enable full_page
print_info "Varnish cache enabled!\n"
```

Under `set -e` a non-zero `sed` ended the script there, with the `sed`'s own exit code, without the
restart and without the message. That is the same rule `inside()` follows, and it settles what the
Go side owes: not an invention, a restoration.

It also means `583fb0c`'s claim — that deleting the twelve twins moved no exit code — was true for
eleven of them and wrong for this one. The shell twin was the last thing that behaved correctly
here, and it is gone.

## Options

| Option | What changes | Lines | Verdict |
|---|---|---|---|
| Give `report()` the status too | every one of ~30 call sites across 12 files, most of which have no status to give | 150-250 | No. `Restart`, `CopyInto`, `SetHost` and `Down` would all pass a dummy zero for one caller's benefit |
| Make the two guards look like `inside()` | `wrappers.go` ~8 lines, plus the tests | ~50-90 | **Recommended.** Converges on a pattern already proved four times and tested once |
| A new shared helper for "run and report" | a function, its test, two call sites | ~50-85 | No. One caller does not earn an abstraction, and retrofitting it onto the four healthy sites is churn on code that is not broken |

## Testing

No seam change is needed. `internal/cli/fake_engine_test.go`'s `fakeEngine.Exec` already returns
`(out.status, out.err)` from an `outcome` indexed by call number, so `outcomes: []outcome{{},
{status: 42}}` says "the call was made and the container exited 42" today. The existing
`TestAFailedEditStopsBeforeTheRestart` covers the `err != nil` branch only; nothing covers
`status != 0, err == nil`.

## Risks

1. Nothing depends on the wrong answer. `varnish()` is reached only from `run.go:120-123` and its
   return value is the process's exit code, chained into no further Go logic.
2. The fix changes an observable: a command that answered 0 will start answering the container's
   code. That is the point, and it is what the shell did, but it is a behaviour change and belongs
   in the change's own record.
3. `--json` needs deciding, not just the exit code: today nothing is emitted on this path. Whether
   the failure gets an error document, or the raw status with no document as `inside()` does, is a
   real question the proposal has to answer rather than inherit.

## Size

Around 50-90 lines. Well inside one work unit.

## Noted

This defect was written down and deferred in three consecutive cycles — `2026-09-04-wrappers-go-tests`,
`2026-09-05-remaining-wrappers-go-tests` and `2026-09-05-down-and-set-host-go-tests` — each time for
size discipline rather than difficulty. It has been testable at the seam the whole time.
