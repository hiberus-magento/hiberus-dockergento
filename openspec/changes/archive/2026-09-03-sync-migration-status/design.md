# Design

## The router is the source, and only that file

The check reads `internal/cli/run.go` and nothing else. The switch in `Run()` is 33 `case` labels
(verified: `case "_registry"` at line 51 through `case "restart"` at line 134); the rest of
`internal/cli/` holds another 42, and every one of them is a subcommand or a flag — `db.go` alone
has nine. A check pointed at the package would count `freeze` and `--json` as commands. Pointed at
the one file that dispatches, it counts commands.

```sh
ROUTER="$COMMAND_BIN_DIR/internal/cli/run.go"

routed() {
    sed -n 's/^[[:space:]]*case "\([a-z0-9_-]*\)":.*/\1/p' "$ROUTER" | grep -v '^_' | sort
}
```

A basic expression and `[[:space:]]` instead of `-E` and a literal tab, for the reason already
written at lines 27-28 of the suite: BSD and busybox read the extended ones differently and the
difference only appears on Alpine. `sed -n 's/…/…/p'` is the form `tests/run.sh:98` already uses.

The underscore exclusion is not a list of one name. It is the same rule
`data/command_descriptions.json` uses and the same rule line 16 of this suite already applies —
a leading underscore means "not a command" — so the next piece of scaffolding is excluded the day
it is written, without touching this file.

Rejected: an `awk` range bounded by the `switch` block. It buys protection against a second switch
inside `run.go` at the price of a tab-anchored terminator, and a second dispatcher in `Run()` would
be the thing to fix, not to parse around.

## The two that are only half in

`db` and `proxy` are routed for part of their subcommands only (`templateSubcommands` at line 85,
`proxySubcommands` at line 129). They are named in the check, with the condition for each one's
disappearance written beside them, the way `MIGRATION.md` already writes it for `_registry`:

```sh
PARTIAL="db proxy"
is_partial() { case " $PARTIAL " in *" $1 "*) return 0 ;; esac; return 1; }
```

Rejected: a third owner value in the table. It would widen the vocabulary of line 47 for two rows,
put a word in the document that means "read the source to find out how much", and cost an edit in
both the test and the table for a distinction that lives in `run.go` anyway.

The exception is pinned from both sides. One assertion excuses `db` and `proxy` from being marked
`go`; a second asserts each is still in the router **and** still marked `shell`. The day either is
finished and its row turns `go`, the exception is stale and the suite says so. It cannot linger.

## Thirty of sixty-five

Thirty-two routed commands minus the two partial ones is thirty, and sixteen rows marked `go` plus
the fourteen to flip is thirty. The two sets coincide exactly, which is what makes the existing
counter assertion at lines 62-65 sufficient: `| Comandos en Go | 30 de 65 |`.

The fourteen: `down`, `set-host`, `copy-to-container`, `copy-from-container`, `version`, `purge`,
`npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `varnish-on`, `varnish-off`,
`setup`.

The reverse check is written first and fails today with those fourteen names in its message.

## Two more places the document contradicts itself

Found while reading it, both inside `MIGRATION.md` and both two lines: line 282 describes the
suite as one-directional, which it stops being; and line 190 explains that `composer` stays in
shell on macOS because `copy-to-container` "no está portado", while line 148 of the same document
records porting it. The real reason is `mirrorsVendor` (`internal/cli/php.go:98`): on darwin the
dependency-writing invocations are a copy-in, run, copy-back sequence, not one call.

The same stale sentence is in the comment above that function, at `internal/cli/php.go:96`. It is
left alone on purpose: this change touches no runtime file, and a comment edit there belongs with
the slice that gives `internal/cli/` its own unit tests.

Outside it, `CLAUDE.md:11` and `architecture/02-cli-architecture.md:5` each lose one sentence and
gain one: the binary from `cmd/hm` is the entry point, `internal/cli/run.go` routes what is
ported, everything else falls through to `bin/run` with its arguments untouched, and `MIGRATION.md`
says which is which. The flow diagram below line 5 stays — it is `bin/run`'s, and that is still
what an unported command goes through.

## The commits

| # | Message | Files | Changed lines |
|---|---|---|---|
| 1 | `test(migration): every command the router answers, checked against the table` | `tests/unit/migration_status_test.sh` (+42), `MIGRATION.md` (~38) | ~80 |
| 2 | `docs: the implementation is Go and Bash, not only Bash` | `CLAUDE.md`, `architecture/02-cli-architecture.md`, `openspec/config.yaml` (already patched) | ~24 |

Commit 1 is one cycle: reverting the table alone leaves a failing suite, so the guarantee and the
data it guarantees move together. Commit 2 is prose no test covers. One direct delivery to
`release/2.0.0`; if the session's one-commit rule is literal, collapse them — the content is
identical and only the rollback separation is lost. The change folder under `openspec/changes/`
rides separately, so the reviewed diff stays the ~104 lines above rather than ~450.

Nothing else travels with them. The `.gitignore` entry for `.codegraph/` sitting in the working
tree is the orchestrator's tooling residue, not this reconciliation, and it is delivered on its
own: no runtime file and no unrelated file changes here.

Threat matrix: not applicable. No routing, subprocess, VCS or PR automation, or executable-file
classification changes — a test script reads two files in the repository it lives in.
