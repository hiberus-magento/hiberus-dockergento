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
   it. An interrupted update never leaves half a binary. When `/usr/local/bin/hm` is a link to
   `~/.hm/bin/hm`, which is what the installer leaves, `~/.hm/bin/hm` is replaced and no `sudo`
   is needed.

## What a terminal shows while it works

On a terminal, each step is shown on stderr as it happens: a spinner while it runs, a green ✓
once it is done, and the download as a progress line (bar, percentage, bytes of the total, rate
and time left; a byte counter when the server announces no size):

```
✓ Latest pre-release: 2.0.0-rc.5 (you have 2.0.0-rc.4)
  ████████████████████ 100%  26 MB / 26 MB · 8.7 MB/s · 3s
✓ Checksum verified
✓ Installed at /home/you/.hm/bin/hm
Hiberus Dockergento updated from 2.0.0-rc.4 to 2.0.0-rc.5
  installed    /home/you/.hm/bin/hm
  what changed https://github.com/hiberus-magento/hiberus-dockergento/releases/tag/2.0.0-rc.5
```

Up to date, the spinner of the look-up goes and only the answer is left. A step that fails is
shown with a red ✗ before the error. `NO_COLOR` keeps the marks and the bar, without colour.

The last three lines are stdout, and they are the same everywhere. The steps are drawn only on a
terminal that can draw: redirected, with `--json`, `--progress=jsonl`, `CI`, `HM_NO_PROGRESS`,
`HM_NON_INTERACTIVE` or `TERM=dumb`, nothing is drawn and stderr is what it always was. The rules
are those of the copy progress line.

## A binary root installed

2.0.0-rc.1 to rc.3 put the binary itself in `/usr/local/bin`, which only root can write. On a
terminal, `hm update` (and `hm switch`) moves it once:

```console
$ hm update --pre
/usr/local/bin/hm is in /usr/local/bin, which only root can write: sudo is needed this once, to move hm to /home/you/.hm/bin and leave a link to it in its place
Password:
hm now lives in /home/you/.hm/bin/hm and /usr/local/bin/hm links to it, so later updates need no sudo
Hiberus Dockergento updated from 2.0.0-rc.2 to 2.0.0-rc.3
  installed    /home/you/.hm/bin/hm
```

The verified binary is written to `~/.hm/bin/hm` first, then `sudo ln -sfn ~/.hm/bin/hm
/usr/local/bin/hm` asks for your password. If `sudo` is refused or fails, the old binary still
runs as before, and `~/.hm/bin/hm` is a copy nothing uses yet, or, when an earlier hm was already
there, that hm put back as it was. Something at `~/.hm/bin/hm` that is not hm 2 (another program,
hm 1.x, a link) is never overwritten: the update is refused before anything is downloaded, naming
the file. Later updates replace `~/.hm/bin/hm` through the link, without `sudo`.

The link in `/usr/local/bin` then points at a binary you own. On a machine several people use,
anyone who can act as your user can change what every other account runs as
`/usr/local/bin/hm`. On a shared host, install per user instead (`HM_INSTALL_DIR=$HOME/.local/bin`),
or keep a binary only root can change by passing the system directory to the installer
explicitly, `--dir /usr/local/bin`, and updating through the installer that way, since `hm update`
at a terminal offers to move it (see [Installation](installation.md)).

Without a terminal — CI, a pipe, `--json`, `HM_NON_INTERACTIVE` or `--yes` — or without `sudo`,
nothing is asked and the update is refused (`install_dir_not_writable`). Re-run the installer,
which moves it with `sudo` once:

```bash
curl -fsSL https://raw.githubusercontent.com/hiberus-magento/hiberus-dockergento/main/install.sh | bash
```

or install where you can write with `HM_INSTALL_DIR=$HOME/.local/bin` (the variable goes on
`bash`).

## When it refuses

| Error | Exit | When |
|---|---|---|
| `built_from_source` | 6 | The binary was built from a checkout and has no release version. Update it where it was built: `git pull && go build -o bin/hm ./cmd/hm` |
| `releases_unavailable` | 1 | GitHub could not be read: no connection, an error, or the rate limit (set `GITHUB_TOKEN`) |
| `asset_missing` | 1 | The release publishes no binary for your platform |
| `download_failed` | 1 | The binary or its checksums could not be downloaded, or they do not match |
| `install_dir_not_writable` | 1 | The directory of the executable is not yours to write, `/usr/local/bin` say, and there is no terminal to move it to `~/.hm/bin` with `sudo`, or `sudo` was refused, or `~/.hm/bin/hm` is something other than hm 2. Re-run the installer, or install where you can write with `HM_INSTALL_DIR=$HOME/.local/bin` (see [A binary root installed](#a-binary-root-installed)) |

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
