# `magento` and `composer` in Go, and a release that builds for both platforms

## Why

`hm magento` is the command run most of all — a cache flush, a reindex, a setup:upgrade, dozens of
times a day — and underneath it is one thing: something run in the php container. The shell
implementation spent 210 ms deciding that before the Magento CLI had started. It now spends 75 ms.

`composer` is the same thing with two sentences said before it starts, and one path that is not
the same thing at all.

The release also needed finishing. Linking the Compose engine brought in a dependency that reaches
macOS's FSEvents through cgo, so a darwin binary cannot be cross-compiled with cgo off — it does
not build. The tool is used on macOS and on Linux, and a release configured to produce one of them
broken would be found by whoever installed it.

## What Changes

- **`hm magento` is Go**: the same command in the same container, with the Magento CLI's own exit
  code, and 135 ms less of getting there.
- **`hm composer` is Go**, including the two refusals: `create-project`, which would leave the
  project inside the container, and a worktree that reads the main checkout's dependencies, where
  Composer cannot write to them. Both now go through the error contract, so a `--json` caller can
  read them — the shell implementation printed a paragraph to stdout.
- **One Composer path stays in shell**: on macOS, `install`, `update`, `require` and `remove` copy
  the dependencies into the container, run Composer, and copy the whole tree back out over the
  host's — deleting the host's vendor directory on the way. It depends on `copy-to-container`,
  which is not ported, and it is not a thing to port half of.
- **The release builds on macOS**, which is where all four binaries can be built: darwin natively
  and for the other architecture through Apple's toolchain, linux with cgo off. Every one of the
  four was built and run before this was written.
- **Both proxy cases are pinned down by tests**: a project routed through the global proxy and one
  that is not, because the tool has to serve both and the difference is a file that may or may not
  be there.
