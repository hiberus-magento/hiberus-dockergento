# hm update

Replace the `hm` you are running with the newest release, when there is a newer one.

## Usage

```bash
hm update          # newest final release
hm update --pre    # newest release, pre-releases included
```

## What it does

1. Reads the releases of Hiberus Dockergento from GitHub
   (`https://api.github.com/repos/hiberus-magento/hiberus-dockergento/releases`). It works
   unauthenticated; when `GITHUB_TOKEN` is set it is sent to GitHub, which raises the rate
   limit.
2. Picks the newest release of 2.0 or later: final releases only, unless you pass `--pre`.
3. Compares it with the version of the running binary. When the running version is the same or
   newer (a release candidate ahead of the last final release, say), it says it is up to date
   and exits 0. An update never moves back: use [switch](switch.md) for that.
4. Downloads `hm_<os>_<arch>` and `checksums.txt` of that release, and checks the binary's
   sha256 against it. No checksums, no line for the binary, or a different digest: nothing is
   installed.
5. Replaces the running executable (symlinks resolved, so the real file is replaced and the link
   kept) in one rename: the new binary is written beside it, made executable and renamed over
   it. An interrupted update never leaves half a binary.

## When it refuses

| Error | Exit | When |
|---|---|---|
| `built_from_source` | 6 | The binary was built from a checkout and has no release version. Update it where it was built: `git pull && go build -o bin/hm ./cmd/hm` |
| `releases_unavailable` | 1 | GitHub could not be read: no connection, an error, or the rate limit (set `GITHUB_TOKEN`) |
| `asset_missing` | 1 | The release publishes no binary for your platform |
| `download_failed` | 1 | The binary or its checksums could not be downloaded, or they do not match |
| `install_dir_not_writable` | 1 | The directory of the executable is not yours to write, `/usr/local/bin` say. Re-run the installer with `sudo`, or install where you can write with `HM_INSTALL_DIR=$HOME/.local/bin` |

In every case the running executable is left exactly as it was.

## Which version is running

A release binary carries its version, stamped when the release is built: it is what
`hm --version` and `hm version` report, and what `hm update` compares. A binary you built
yourself with `go build` has none unless you stamp it:

```bash
go build -ldflags "-X github.com/hiberus-magento/hiberus-dockergento/internal/cli.Version=2.1.0" -o hm ./cmd/hm
```

Stamp it with a release version. A stamp in the shape `git describe --tags` prints
(`2.0.0-5-gdeadbee`, `-dirty`) or a GoReleaser snapshot's (`2.0.1-SNAPSHOT-deadbee`) names a build
past or beside that release, so `hm update` and `hm switch` treat it as built from source
(`built_from_source`) rather than replace it with an older release.

## For tests

`HM_RELEASES_URL` replaces the address the releases are read from. Pointed at a `file://` JSON
in the GitHub API's format, whose assets are `file://` URLs too, `hm update` and `hm switch` run
without the network: `tests/integration/version_switching_test.sh` does exactly that.
