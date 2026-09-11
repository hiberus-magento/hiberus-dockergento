# Exploration: exec-and-masquerade-go-tests

Change: `exec-and-masquerade-go-tests`
Engram topic: `sdd/exec-and-masquerade-go-tests/explore` (project `hiberus-dockergento`)
Date: 2026-09-11

Step two of `MIGRATION.md`'s rule for the three commands deliberately left out of
`read-only-commands-go-tests`: `bash`, `exec` and `masquerade`. They were held back because all
three are declared `"safety": "dangerous"`, because `bash` and `exec` are joined at `execute()` and
cannot move separately, and because `masquerade`'s decline path needs `os.Stdin` redirection.

## The table, verified

| Command | Handler | Engine call | On `commands`? |
|---|---|---|---|
| `bash` | `tools.go:10-18` `shell()` → `execute()` | via `execute()` | — |
| `exec` | `orchestrate.go:171-195` `execute()`, call at `:188-189` | `Exec(here(), phpService, args, terminalOptions(user))` through the concrete `engine(...)` | `Exec` yes, but not reached through `newEngine` |
| `masquerade` | `tools.go:24-55`, call at `:54` | `Anonymise(here())` through the concrete `engine(...)` | **no** |

The interface has 15 methods. No Go test exists for any of the three.

One correction to the orchestrator's count: `debug-on.sh` calls `exec.sh` at **six** sites — `:15`,
`:16`, `:19`, `:27`, `:32`, `:34` — not five.

## A. The seam grows by one method and two lines

**`bash` and `exec` cost one line.** `orchestrate.go:188`, `engine(` → `newEngine(`. `Exec` is
already on the interface and already exercised through `inside()`. That single line puts both
commands on the seam, because `shell()` only ever calls `execute()`. This is the cheapest seam
addition of the four cycles so far.

**`masquerade` costs one method**, `Anonymise(dir string) error`, outcome-only like `Restart` and
`Dump` — it returns only an error, so there is nothing to answer with — and `call` gains **nothing**,
since `Dir` already exists:

```go
func (f *fakeEngine) Anonymise(dir string) error {
    number := len(f.calls)
    f.calls = append(f.calls, call{Method: "Anonymise", Dir: dir})

    return f.outcomeFor(number).err
}
```

15 methods become 16. `var _ commands = (*dockergento.Engine)(nil)` needs no edit.

## B. The decline path, which is the one piece of new machinery

`masquerade()` rejects dashed arguments, calls `projectOr`, then `ask("…", "Y")`, and only on `""`
or `"y"` reaches `Anonymise`. Anything else returns `exitOK` without touching the engine.

`ask()` (`ask.go:23-56`) checks `HM_NON_INTERACTIVE` first and returns the suggestion without
reading stdin at all. `answering(t)` pins that variable to `1` as one of its six RED-safety pins,
**so the decline branch is structurally unreachable under the standard helper** — the switch can
only ever see `""` or `"y"`.

A decline test needs three things: `answering(t)` as usual (its pins are still wanted, and
`projectOr`'s `Resolve` must succeed), a nested `t.Setenv("HM_NON_INTERACTIVE", "")` — which stacks
and unwinds LIFO back through the helper's own pin — and `os.Stdin` swapped for a pipe carrying
`n\n`. `ask()` reads the real `os.Stdin` directly, so nothing short of swapping the global works.

`internal/cli/ask_test.go`'s `askingWith` is the precedent and transfers almost unchanged. Strictly
only `os.Stdin` must move, since `ask()` writes its question to the real `os.Stderr` rather than the
writer `masquerade()` was handed — but matching `askingWith`'s three-stream swap keeps the two files
consistent and costs nothing.

The confirm path needs none of this and goes through the ordinary pin.

## C. `Dir` is `here()`, not the project root

Both `execute()` and `masquerade()` call the engine with `here()`. `inside()` does the opposite and
`inside_test.go` asserts `Dir: root` accordingly — so copying that neighbouring precedent produces a
green test asserting the wrong value. This is the fourth handler where the divergence bites;
`logs` and `CopyInto` were the earlier two.

The working template is already in the repo: `wrappers_test.go`'s
`TestWhatCopyingIntoTheContainerAsks` sets `fake.project = core.Project{Name: "shop", Root:
"/code/shop"}`, captures `cwd := here()`, and asserts `Dir: cwd`. The root must differ from any
`t.TempDir()` so the assertion discriminates instead of passing by coincidence.

Expected call logs once routed:

| Command | Log |
|---|---|
| `exec` | `Resolve{cwd}`, `Exec{cwd, phpfpm, …}` |
| `bash` | the same, with `Command: ["bash"]` and `terminalOptions("root")` when `-r` was passed |
| `masquerade`, confirmed | `Resolve{cwd}`, `Anonymise{cwd}` |
| `masquerade`, declined | `Resolve{cwd}` only |

## D. The RED window, traced rather than assumed

Both were followed into the adapters, because the worst case here is not reading a container list —
it is writing to a database.

**`execute()` unrouted** reaches `Engine.Exec` → `Resolve` (which succeeds with a nameless project,
per the correction established two cycles ago) → `orchestrator.Exec` → `load()`, which finds no
compose file under the temp root and refuses before `service.Exec` runs. **`t.Chdir(t.TempDir())` is
what bounds this**, not `DOCKER_HOST`; no daemon dial happens at all.

**`masquerade` unrouted** reaches `Engine.Anonymise` → `Resolve` → `app.Importer.Anonymise` →
`Database.Ready` → `container(project)` → `Engine.Containers()`, a machine-wide `ContainerList`.
**`DOCKER_HOST=unix:///nonexistent` is what bounds this**, exactly as for `list` and `describe`.

**A second, independent layer that `describe`'s RED bug did not have.** `container()` matches on
`candidate.Key() != project.Name`, and `Key()` (`core/environment.go:112-118`) falls back to
`ComposeProject` when the tool's own `hm.project` label is empty. Compose stamps
`com.docker.compose.project` on everything it creates, so a nameless project matches essentially
nothing. Contrast `toolinfo/tooling.go:84-85`, which compares the raw label instead of calling
`Key()` — which is precisely why `describe`'s unrouted RED could have selected an unrelated
container. **That asymmetry is a defect in `toolinfo`, not a virtue here, and belongs in the
backlog.** It does not make `DOCKER_HOST` optional.

With `answering(t)` used as designed, no unrouted RED for any of the three can reach a database or
mutate data. A RED that answers `exitOK`, or logs a call where the log should be empty, is the alarm
signature that a pin did not hold.

## E. The shell twins

All three read, plus the shared `console/components/masquerade.sh`.

- **No `varnish-on`-shaped divergence.** Each script ends its meaningful work in a command whose
  status becomes the script's own; no command substitution assigned to a variable hides a failure.
- **`masquerade`: Go is stricter, and undocumented.** The shell twin never reads `$@`; the Go handler
  rejects any dashed argument. `data/command_descriptions.json:446-451` declares no `opts`. This is
  intentional — `--yes` is a global consumed by `globals()` before the command sees its arguments —
  but a test should pin what ships rather than quietly restore the shell's laxity.
- **Two files are named `masquerade.sh`.** `console/commands/masquerade.sh` is the retireable twin;
  `console/components/masquerade.sh` is a helper sourced by `mysql.sh:34` and `transfer-db.sh:6`, and
  by the twin itself. A later deletion must touch only the first.
- **Below the seam and out of reach here**: `app/anonymise.go:65` passes `tty: false` unconditionally,
  while the shell helper passes `-t -i` when both streams are terminals. Noted, not actionable at
  this layer.

## F. Size

| Unit | Floor | Honest band |
|---|---|---|
| `bash` + `exec` — one routing line, no interface growth | ~180-210 | ~230-420 |
| `masquerade` — one method, one routing line, the stdin machinery | ~220-260 | ~285-500 |
| **Cumulative** | **~400-470** | **~515-920** |

**It needs a split.** Each unit fits 400 alone; the cumulative does not, at either the floor or the
band. Two units, `bash`+`exec` first — it is cheaper and establishes the `t.Chdir` pin discipline —
then `masquerade`, which carries the interface growth and the stdin precedent.

## Deletion is not in this cycle, and only two of the three will ever qualify

`exec.sh` has live callers: `debug-on.sh` (six), `debug-off.sh`, `create-project.sh` — all still
genuinely shell. `magento.sh:4` also calls it, but `magento` is fully routed, so that is dead shell
calling dead shell. `bash.sh` has no runtime caller, only a comment at `tests/unit/skills_test.sh:126`.
`masquerade.sh` has none. So a later retirement can take `bash.sh` and `masquerade.sh`; `exec.sh`
waits for `exec-callers-in-go`.
