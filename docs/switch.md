# switch

Install another release in place of the `hm` you are running, and back.

```bash
hm switch --list          # the releases available
hm switch 2.1.0-rc.1      # try a release candidate
hm switch 2.0.0           # go back to a previous release
hm switch --stable        # the newest final release
```

## How it works

`hm` is one self-contained binary. Switching reads the releases from GitHub, downloads the
release's `hm_<os>_<arch>` and `checksums.txt`, checks the sha256, and renames the new binary
over the running executable — the same download, check and replacement as
[update](update.md), with the same refusals when something fails. Unlike `update`, `switch`
goes to the version you name, older or newer, pre-release or not.

Switching to the version already running changes nothing and says so.

## Only releases of 2.0 and later

From 2.0 hm is a Go program with a binary for each platform; 1.x is the shell tool, which has
none. So `hm switch` refuses a 1.x before asking GitHub anything:

```console
$ hm switch 1.5.1
'1.5.1' is a version of the shell tool (1.x), which hm switch cannot install: nothing was changed
  → Install it separately, in a directory of its own: git clone -b 1.5.1 https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm-1.x, then run ~/hm-1.x/bin/run
```

The refusal is `version_before_go`, exit code 2.

## Branches are not switched to

A branch has no release binary. `hm switch release/2.1.0` is refused (`not_a_version`, exit
code 2) with how to build it from source instead:

```bash
git clone -b release/2.1.0 https://github.com/hiberus-magento/hiberus-dockergento.git
cd hiberus-dockergento && go build -o hm ./cmd/hm
```

A version that is not published is `unknown_version`, exit code 2. `--stable` with no final
release of 2.0 or later published yet is `no_release`, exit code 2.

A binary built from a checkout is not replaced (`built_from_source`, exit code 6), as in
[update](update.md). `--list` still works from one.

## What `--list` shows

Releases of 2.0 and later, newest first, with the running one marked and pre-releases
labelled:

```console
$ hm switch --list
Versions
  2.1.0-rc.1            pre-release
  2.0.0                 ← installed
  2.0.0-rc.1            pre-release

Running 2.0.0
```

A pre-release is there to be tried on purpose. Labelling it is what keeps someone reading the
list from the top from landing on one by accident.

In JSON, `current` is the running version (`devel` for a binary built from a checkout) and each
version carries `name`, `pre_release` and `installed`.

## Why it exists

Validating a release candidate on real projects, and sharing it with colleagues who can go
back the moment something breaks, needs three things: knowing exactly what you are running,
switching without ceremony, and not being taken off the version you are testing. `hm update`
never moves a release candidate back to an older final release.

## Knowing what you run

```bash
$ hm --version
```

Quote that version when reporting a problem: "2.0.0-rc.3" and "2.0.0" are not the same thing.

## If anything goes wrong

The running executable is only ever replaced by a verified binary, in one rename. If it is
damaged anyway, re-run the installer: it installs the latest release again.

## Tags

| | |
|---|---|
| `X.Y.Z` | Final releases, the ones `--stable` and `hm update` go to |
| `X.Y.Z-rc.N` | Release candidates, which is what gets shared for internal validation |
