# hm copy-from-container

Copy files or folders to host from container.

## Usage

```bash
hm copy-from-container <path>...    # files or folders, relative to the workdir
```

## Description

The `copy-from-container` command is used to copy files or folder to host from container.

The container's `.git` is never written onto the host: a `.git` directory or file at the top of a copied folder (`hm copy-from-container .` included) is skipped, so a stale repository left in the container cannot overwrite the host's. A `.git` deeper in the tree, such as a package Composer installed from source under `vendor`, is copied like any other file.

A read-only file already on the host is replaced, as `docker cp` replaces it, as long as its directory can be written; when it cannot, the error names the file and the reason.

## Progress

On a terminal, a line under each `Copying …` line shows how far the copy has got, redrawn in place:

```text
  ████████████░░░░░░░░  61%  412 MB / 675 MB · 38120 files · 52 MB/s · ~6s
```

While the php container runs, the total is what `du -sb` reports inside it, which gives a percentage and a time left; when it is stopped, or `du` fails, the line counts bytes, files and rate without a percentage. The line goes to stderr, and is not drawn under `--json`, in CI (`CI` set), with `HM_NO_PROGRESS` or `HM_NON_INTERACTIVE` set, or when stderr is not a terminal: there the output is exactly what it was. `NO_COLOR` keeps the bar and drops its colour.
