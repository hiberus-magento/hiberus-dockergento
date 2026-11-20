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
3. Regenerates the shell completion.

## When the binary cannot be obtained

With no release binary to download and no Go to build one, the update still happens and the
previous binary stays in place, but the command fails (exit code 1, error `binary_not_refreshed`)
and says why each way failed. The old binary may not match the updated files, so install
[Go](https://go.dev/dl/) and run `hm update` again, or build it yourself:

```bash
cd ~/hm && go build -o bin/hm ./cmd/hm
```
