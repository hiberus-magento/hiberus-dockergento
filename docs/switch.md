# switch

Move the installation to another version or branch, and back.

```bash
hm switch --list          # what is available
hm switch 1.5.0-rc.3      # try a release candidate
hm switch --stable        # back to the stable branch
hm switch release/1.6.0   # follow a release branch
```

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

## One limitation worth knowing

`hm switch` can only take you **away** from a version that has it. Versions before 1.5.0 do
not include the command, so once you switch to one of those, coming back is plain git:

```bash
cd "$(dirname "$(dirname "$(readlink "$(command -v hm)")")")"
git checkout main
```

Same escape hatch if anything ever goes wrong: the installation is a git checkout, nothing
more.

## Branches and tags

| | |
|---|---|
| `main` | Stable. Only reached through a pull request, tagged `X.Y.Z` |
| `feature/*` | Work in progress |
| `release/X.Y.Z` | Freeze and validate |
| `X.Y.Z-rc.N` | Release candidates, which is what gets shared for internal validation |

`--stable` means `main`.
