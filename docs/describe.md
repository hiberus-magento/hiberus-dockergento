# describe

Everything that defines the project in the current directory: URLs, versions, services,
paths and state. It is the answer to "what is this environment?", for a person and for a
machine.

```bash
hm describe                  # readable output
hm describe --json           # machine readable
hm describe --with-secrets   # include database credentials
```

Readable output when stdout is a terminal, JSON when it is piped or redirected. See
[output, exit codes and non-interactive use](../README.md).

## Options

| Option | Description |
|---|---|
| `--with-secrets` | Include credentials. They are omitted by default, because this output ends up in tickets and in AI agent context |
| `--json` / `--no-json` | Force the output format |

## What it reports

| Block | Contents |
|---|---|
| `project` | Name, root directory, worktree, domain, status (`running`, `stopped`, `partial`) and URLs |
| `magento` | Version and deploy mode |
| `services` | Every service with its image, state and published ports |
| `paths` | Magento directory, workdir inside the container, mount strategy and compose files |
| `tooling` | Platform, `hm` version, Docker Compose version and Xdebug state |
| `credentials` | Only with `--with-secrets` |

## It works with the environment stopped

Most of the information comes from files, not from Docker, so `describe` answers even when
nothing is running — which is when it is usually needed. The state is reported as
`stopped`, services as `not created`, and Xdebug as `unknown`.

## Schema stability

The JSON response is versioned with `schema_version`. Within a version, the keys
documented above are stable: they are always present, even when their value is empty or
null. New keys may be added without bumping the version, so consumers should ignore keys
they do not know. Removing or renaming a documented key bumps `schema_version`.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `2` | Unknown option |
| `4` | Not a configured Dockergento project |

## Example

```bash
$ hm describe --json | jq -r '.data.project.urls.base'
https://myproject.local/

$ hm describe --json | jq -r '.data.services[] | select(.state != "running") | .name'
varnish
```

## The admin URL

The address of the admin panel uses the `frontName` from `app/etc/env.php`, which is a random
string on most installs. See [launch](launch.md#the-admin-is-not-always-at-admin).
