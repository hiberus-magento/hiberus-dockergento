# tui

The dashboard: every Dockergento environment on this machine on one screen, with the
actions that matter one key away.

```bash
hm tui
```

Like [`list`](list.md) and [`doctor`](doctor.md) it does not need to run inside a project.
It reads the fleet from container labels, so it answers from anywhere.

## What it shows

**Fleet** — one row per environment: name, status, containers running out of the total,
branch and directory. A worktree is flagged with its identifier, an environment whose
directory no longer exists with `!`. Warnings from `doctor` sit above the table, so a
stopped Docker or a full disk is visible before wondering why nothing runs.

**Detail** — `enter` on a row opens what `describe` knows about that project: Magento and
PHP versions, domain, URLs and the state of each service.

## Keys

| Key | Action |
|---|---|
| `↑` `↓` / `k` `j` | Move through the list |
| `enter` | Open the selected environment |
| `esc` / `h` | Back to the fleet |
| `s` | Start the environment |
| `x` | Stop it |
| `r` | Restart it |
| `l` | Follow its logs |
| `o` | Open its storefront in the browser |
| `g` | Refresh the data |
| `?` | The list of keys |
| `q` / `ctrl-c` | Quit |

The footer shows the keys available in the current view. On a narrow terminal it shows the
short form; the full list is always one `?` away.

## It presents, the CLI decides

There is no logic of its own here. The data comes from `hm list --json`,
`hm describe --json` and `hm doctor --json`, and every action runs the real command —
`hm start`, `hm stop`, `hm restart`, `hm logs -f` — inside that environment's directory.

Two consequences worth knowing:

- **The protections apply.** An action refused by the CLI is refused here too, with the
  same message. The [worktree guardrails](worktree.md) do not have a bypass through the
  dashboard.
- **Failures are readable.** During an action the dashboard hands the terminal back, so the
  command's own output is shown in full instead of cropped inside a box. It waits for a key
  before returning, and refreshes the fleet afterwards.

## Requirements

It needs a terminal. Piped or redirected, it refuses instead of writing control codes into
a file:

```console
$ hm tui > out.txt
Error: The dashboard needs a terminal, and this output is not one
Try: hm list --json
```

Exit code `2`, and in JSON mode the error type is `requires_terminal`.

It runs on the Bash that ships with macOS (3.2) with no dependency beyond what the CLI
already needs. Terminal handling — the alternate screen, key reading, resizing — lives in
[terminal components](terminal-components.md), which also documents what happens on a
terminal too small or without colour.

## When something goes wrong

The dashboard restores the terminal on the way out, including on `ctrl-c` and if it dies
mid-draw: the cursor comes back and the screen returns to what was there before. If a
terminal is ever left in a strange state, `reset` fixes it.

Data older than a few seconds is normal: the header says when it was read, and `g` reads
it again.
