# Supported versions

What `hm setup` gives each Magento version, the images behind it, and how to move between
releases of `hm` itself.

## The stack `hm setup` picks

`hm setup` reads the Magento version from the project and writes the services below. The table is
`data/requirements.json`, which is the authoritative source; every patch release maps to one of
these rows through `data/equivalent_versions.json` (for example `2.4.7-p10` uses the `2.4.7` row).

| Magento | PHP | MariaDB | Search | Redis / Valkey | Varnish | Composer | Nginx | RabbitMQ |
|---|---|---|---|---|---|---|---|---|
| 2.3.0 – 2.3.2-p2 | 7.2 | 10.2 | Elasticsearch 5.6 | Redis 5 | 6 | 1 | 1.18 | 3.9 |
| 2.3.3 – 2.3.4-p2 | 7.3 | 10.2 | Elasticsearch 6.5 | Redis 5 | 6 | 1 | 1.18 | 3.9 |
| 2.3.5 – 2.3.6-p1 | 7.3 | 10.2 | Elasticsearch 7.17 | Redis 5 | 6 | 1 | 1.18 | 3.9 |
| 2.3.7 – 2.3.7-p4 | 7.4 | 10.3 | Elasticsearch 7.17 | Redis 6.2 | 6 | 1 | 1.18 | 3.9 |
| 2.4.0 – 2.4.1-p1 | 7.4 | 10.4 | Elasticsearch 7.17 | Redis 5 | 6 | 1 | 1.18 | 3.9 |
| 2.4.2 – 2.4.3-p3 | 7.4 | 10.4 | Elasticsearch 7.17 | Redis 6.2 | 6 | 2.1 | 1.18 | 3.9 |
| 2.4.4 – 2.4.4-p18 | 8.1 | 10.4 | OpenSearch 1.2 | Redis 6.2 | 7.1 | 2.1 | 1.18 | 3.9 |
| 2.4.5 – 2.4.5-p17 | 8.1 | 10.4 | OpenSearch 1.2 | Redis 6.2 | 7.1 | 2.2 | 1.18 | 3.9 |
| 2.4.6 – 2.4.6-p15 | 8.2 | 10.6 | OpenSearch 2.5 | Redis 7 | 7.1 | 2.2 | 1.18 | 3.9 |
| 2.4.7 – 2.4.7-p10 | 8.3 | 10.6 | OpenSearch 2.5 | Redis 7 | 7.1 | 2.7 | 1.18 | 3.9 |
| 2.4.8 – 2.4.8-p5 | 8.4 | 11.4 | OpenSearch 2.5 | Redis 7 | 7.1 | 2.9 | 1.18 | 3.9 |
| 2.4.9 | 8.5 | 12.3 | OpenSearch 3 | Valkey 9 | 7.1 | 2.9 | 1.30 | 4.3 |

Every row also gets Hitch 1.7 (TLS) and a mail catcher: Mailhog 1.0, or Mailpit 1.0 with
`hm setup --mail=mailpit`. Run `hm compatibility` to see the full list of versions and patches.

## Docker images

All images come from [hiberusmagento](https://hub.docker.com/u/hiberusmagento) on Docker Hub,
except the two noted below.

- **Nginx**: `1.18`, `1.28`, `1.30`
- **PHP**: `7.2`, `7.3`, `7.4`, `8.1`, `8.2`, `8.3`, `8.4`, `8.5`
- **MariaDB**: `10.2`, `10.3`, `10.4`, `10.6`, `11.4`, `12.3`
- **RabbitMQ**: `3.9`, `4.2`, `4.3`
- **Redis**: `5.0`, `6.2`, `7.0`
- **Valkey**: `9`
- **ElasticSearch**: `5.6`, `6.5`, `7.17`
- **OpenSearch**: `1.2`, `2.5`, `3`
- **Mailhog**: `1.0`
- **Mailpit**: `1.0` (`hm setup --mail=mailpit`)
- **Varnish**: `6.0`, `7.1`
- **Hitch**: `1.7`

> MariaDB `11.4` / `12.3` use the official `mariadb:11.4` / `mariadb:12.3` images directly (no
> Hiberus wrapper).
>
> Valkey `9` uses the official `valkey/valkey:9-alpine` image directly (no Hiberus wrapper). It is
> a drop-in replacement for Redis and is wired through the same `redis` service slot, so no Magento
> configuration changes are required.

## Magento compatible versions

- **Magento 2.4**: `2.4.0`, `2.4.1`, `2.4.2`, `2.4.3`, `2.4.4`, `2.4.5`, `2.4.6`, `2.4.7`,
  `2.4.8`, `2.4.9`.
- **Magento 2.3**: `2.3.0`, `2.3.1`, `2.3.2`, `2.3.3`, `2.3.4`, `2.3.5`, `2.3.6`, `2.3.7`.

_(All patched versions are also compatible — run `hm compatibility` to see the full list)_

## Versions of hm and release candidates

`hm` is one binary downloaded from a GitHub release, so moving between versions is a command:

```bash
hm --version              # exactly what you are running
hm switch --list          # the releases available, pre-releases labelled
hm switch 2.0.0-rc.9      # try a release candidate
hm switch --stable        # back to the newest final release
hm update --pre           # the newest release, candidates included
```

`hm update` never moves back: on a release candidate newer than the last final release it says
you are up to date. `hm switch` only installs releases of 2.0 or later; a 1.x and a branch have no
binary. See [switch](switch.md) and [update](update.md), and the branch and tag model in
[CONTRIBUTING.md](../CONTRIBUTING.md#branches-and-releases).
