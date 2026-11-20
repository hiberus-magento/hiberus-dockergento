# switch

Move the installation to another version or branch, and back.

```bash
hm switch --list          # what is available
hm switch 2.0.0-rc.1      # try a release candidate
hm switch --stable        # back to the stable branch
hm switch release/2.0.0   # follow a release branch
```

## Only between versions of 2.0 and later

From 2.0 hm is a Go program, `cmd/hm`; 1.x is the shell tool, which has no binary to obtain and
no `bin/hm` to run. So `hm switch` refuses a tag or branch whose tree has no `cmd/hm` — read from
the tree, since a branch has no version number — before the checkout moves:

```console
$ hm switch 1.5.1
'1.5.1' is a version of the shell tool (1.x), which hm switch cannot move to: the installation was not changed
  → Install it separately, in a directory of its own: git clone -b 1.5.1 https://github.com/hiberus-magento/hiberus-dockergento.git ~/hm-1.x, then run ~/hm-1.x/bin/run
```

The refusal is `version_before_go`, exit code 2, the same as a reference that does not exist.
`--stable` follows the same rule: while `main` is still 1.x, it is refused too. `--list` still
lists the 1.x tags.

## The compiled command follows

`bin/hm` is not in git, so a checkout alone would leave the binary of the version you came from.
After switching, `hm switch` gets the binary for where you are now: the release's
`hm_<os>_<arch>`, checked against its `checksums.txt`, when you switched to a release tag; a
`go build` otherwise, or when that download fails. A branch is only ever built.

When neither works, the switch stays done, the previous binary is kept, and the command fails
with `binary_not_refreshed` (exit code 1) and the command to build it, as described in
[update](update.md).

## What `--list` shows

Versions most recent first, with the installed one marked and pre-releases labelled, followed
by the branches available:

```console
$ hm switch --list
Versions
  1.5.0-rc.3            pre-release
  1.4.5                 ← installed
  1.4.4
```

A pre-release is there to be tried on purpose. Labelling it is what keeps someone reading the
list from the top from landing on one by accident.

In JSON each version carries `name`, `pre_release` and `installed`.

## Why it exists

Validating a release candidate on real projects, and sharing it with colleagues who can go
back the moment something breaks, needs three things: knowing exactly what you are running,
switching without ceremony, and not being taken off the version you are testing.

## What it protects

**Your local changes.** If the installation directory has uncommitted changes to tracked
files, `switch` refuses and lists them. It never stashes for you: a silent stash is an
elegant way of losing work. Untracked files —your notes, editor config, AI skills— do not
get in the way, because git allows a checkout with them present.

**The version you are validating.** While the installation sits on a tag, the checkout is
detached and `hm update` refuses to touch it, pointing you here instead. Before this
command existed, `hm update` on a tag pulled the remote's default branch and took you off
the candidate without a word.

## Knowing what you run

```bash
$ hm --version
hm 1.5.0-rc.3-7-gabc1234
  branch       release/1.5.0
  commit       abc1234
  installed    /Users/me/.hm-install
```

`hm --version --json` gives the same data as separate fields, including `commits_ahead` and
whether the checkout is `dirty`. Quote that version when reporting a problem: "1.5.0-rc.3"
and "1.5.0-rc.3-7-gabc1234" are not the same thing.

## If anything goes wrong

The installation is a git checkout, nothing more, so plain git is always the escape hatch:

```bash
cd "$(dirname "$(dirname "$(readlink "$(command -v hm)")")")"
git checkout <a 2.x tag or branch>
```

## Branches and tags

| | |
|---|---|
| `main` | Stable. Only reached through a pull request, tagged `X.Y.Z` |
| `feature/*` | Work in progress |
| `release/X.Y.Z` | Freeze and validate |
| `X.Y.Z-rc.N` | Release candidates, which is what gets shared for internal validation |

`--stable` means `main`.
