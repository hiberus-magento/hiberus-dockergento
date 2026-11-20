# hm update

Update the `hiberus dockergento` installation and its compiled command.

## Usage

```bash
hm update
```

## What it does

1. Pulls the branch the installation is on (`git pull origin <branch>`). An installation pinned
   to a tag refuses instead: see [switch](switch.md).
2. Gets the `hm` binary for what was pulled. `bin/hm` is not in git, so the pull does not move
   it:
   - when the checkout is exactly at a release tag, the release's `hm_<os>_<arch>` is
     downloaded and checked against that release's `checksums.txt`;
   - otherwise, or when that fails, it is built with Go: `go build -o bin/hm ./cmd/hm`.

   The new binary replaces `bin/hm` in one rename, so an interrupted update never leaves half
   a binary.

   When the running `hm` is `bin/hm` built from exactly the commit that is checked out (an
   update that pulled nothing, say), there is nothing to obtain and no toolchain is needed.
3. Regenerates the shell completion.

## When the binary cannot be obtained

`bin/hm` is the program itself, so it only ever comes from the checkout's own commit: a release
binary is downloaded only for a checkout exactly at that release's tag, never the latest release
for a branch. On a branch (`release/2.0.0`, `main`) with no Go toolchain there is therefore no
binary to get.

Then the pull is kept, the previous `bin/hm` stays in place, and the command fails (exit code 1,
error `binary_not_refreshed`). The message says first that the installation moved, then why each
way of getting a binary failed. It does not report success, because the program you run has not
been updated. To finish the update, either:

- install [Go](https://go.dev/dl/) and build it:

  ```bash
  cd ~/hm && go build -o bin/hm ./cmd/hm
  ```

- or pin a release, whose binary is downloaded: `hm switch --list`, then `hm switch <version>`.

If the shell completion also fails, both failures are reported in the same error.

The installer follows the same rule: on a checkout that is not at a release tag it only builds,
and without Go it stops and says how to build or which release to pin.
