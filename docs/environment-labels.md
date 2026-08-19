# Environment labels

Every container of a Dockergento environment carries a set of `hm.*` labels. They are what
lets any tool answer "which environments exist on this machine, and what are they?" without
guessing from container names.

## The labels

| Label | Contents |
|---|---|
| `hm.project` | Compose project name |
| `hm.root` | Absolute host path of the checkout the environment was started from |
| `hm.worktree` | Worktree identifier; empty for the main checkout |
| `hm.profile` | Environment profile (`full` by default) |
| `hm.magento` | Magento version taken from `composer.lock` |
| `hm.version` | Version of `hm` that created the environment |
| `hm.agent` | Optional owner, set through the `HM_AGENT` environment variable |

## Stable identity only

A container lives for days or weeks; the current branch changes every few hours. A
`hm.branch` label would therefore be wrong most of the time, so it does not exist.

Anything volatile is derived at read time from `hm.root`:

```bash
git -C "$(docker ps --filter "label=hm.project=myproject" \
    --format '{{.Label "hm.root"}}' | head -1)" rev-parse --abbrev-ref HEAD
```

That same path gives orphan detection for free: an environment whose `hm.root` no longer
exists on disk is a leftover that can be cleaned up.

## Useful queries

```bash
# Every Dockergento environment on the machine
docker ps -a --filter "label=hm.project" --format '{{.Label "hm.project"}}' | sort -u

# Containers of one environment
docker ps -a --filter "label=hm.project=myproject"

# A single service, without matching container names by substring
docker ps --filter "label=hm.project=myproject" --filter "label=com.docker.compose.service=db"
```

The helper functions in `console/helpers/docker.sh` (`hm_environments`,
`hm_environment_label`, `hm_environment_containers`, `hm_environment_branch`,
`hm_environment_is_orphan`, `hm_service_container`) wrap all of this.

## Existing projects

The labels are interpolated by Compose at `up` time, so they are not written into the
`docker-compose.yml` that lives in the project repository. A project created before this
feature picks them up by regenerating its configuration and recreating the containers:

```bash
hm setup -f
hm rebuild
```

Until then the environment is still discovered through the standard Compose labels and
reported as having no metadata.
