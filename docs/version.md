# version

The version of the tool, and of the container tooling it drives.

```bash
hm version
hm version --json
```

```console
$ hm version
hm 1.5.0-rc.3-4-g32a9490
  branch       release/1.5.0
  commit       32a9490
  installed    /Users/someone/hm

  docker       27.4.0
  compose      2.34.0
```

It does not require a project: it exists to be pasted into a bug report, and the problem being
reported may be that there is no project.

## Why it is not just `hm --version`

`hm --version` answers the first half and stops there. It is the shortest path in the CLI and it
has a [performance budget](performance.md) watched by a test, so it does not call Docker.
`hm version` pays for the two Docker calls because the versions of Docker and Compose are
exactly what a bug report needs.

`hm --version` is unchanged, and its JSON keys are the same as before: scripts that read it keep
working.

## Docker not available

Not a failure. The tool's own version is reported and the others are left empty (`not available`
in readable output), because "Docker is not responding" is itself worth reporting.

## The versions available

`hm switch --list` lists them, most recent first, marking the one installed and labelling
pre-releases:

```console
$ hm switch --list
Versions
  1.5.0-rc.3            pre-release
  1.4.5                 ← installed
  1.4.4
  ...
```

In JSON each entry carries `name`, `pre_release` and `installed`, so a script can pick the
newest final version without parsing version numbers itself:

```bash
hm switch --list --json | jq -r '[.data.versions[] | select(.pre_release | not)][0].name'
```

See [switch](switch.md) for moving between versions, and
[environment labels](environment-labels.md) for the version recorded on each environment.
