# hm copy-to-container

Copy files or folders to container from host.

## Usage

```bash
hm copy-to-container [ file1, ..., fileN | folder, ...f, olderN ]
```

## Description

The `copy-to-container` command is used to copy files or folder to container from host.

## Progress

On a terminal, a line under each `Copying …` line shows how far the copy has got, redrawn in place:

```text
  ████████████░░░░░░░░  61%  412 MB / 675 MB · 38120 files · 52 MB/s · ~6s
```

The total is measured on this machine before the copy starts, so the percentage and the time left are real. The line goes to stderr, and is not drawn under `--json`, in CI (`CI` set), with `HM_NO_PROGRESS` or `HM_NON_INTERACTIVE` set, or when stderr is not a terminal: there the output is exactly what it was. `NO_COLOR` keeps the bar and drops its colour.
