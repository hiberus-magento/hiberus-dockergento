# list

Every Dockergento environment on this machine, running or not. Unlike the rest of the
commands it does not need to be run inside a project: the inventory comes from container
labels, so it answers from anywhere.

```bash
hm list
hm list --json
```

## What it reports

| Field | Contents |
|---|---|
| `name` | Compose project name |
| `root` | Directory the environment was started from |
| `worktree` | Worktree identifier, empty for a main checkout |
| `branch` | Current branch, derived from `root` at read time |
| `magento` | Magento version recorded when the environment was created |
| `status` | `running`, `stopped` or `partial` |
| `containers` | How many containers are running out of the total |
| `has_metadata` | Whether the environment carries `hm.*` labels |
| `orphan` | Whether its directory no longer exists |

## Environments it cannot see

An environment whose containers have never been created does not appear: there is nothing
to read labels from. Environments created before the `hm.*` labels existed do appear,
recognised by their `phpfpm` service and flagged as having no metadata; `hm setup -f`
followed by `hm rebuild` adds the labels.

See [environment labels](environment-labels.md).

## Schema stability

The JSON response is versioned with `schema_version`. Within a version, the keys
documented above are stable: they are always present, even when their value is empty or
null. New keys may be added without bumping the version, so consumers should ignore keys
they do not know. Removing or renaming a documented key bumps `schema_version`.

## Example

```bash
# Environments that are currently up
hm list --json | jq -r '.data.environments[] | select(.status == "running") | .name'

# Leftovers whose directory is gone
hm list --json | jq -r '.data.environments[] | select(.orphan) | "\(.name)\t\(.root)"'
```
