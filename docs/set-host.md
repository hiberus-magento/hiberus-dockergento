# hm set-host

Set project host in `/etc/hosts` and Magento `core_config_data`.

## Usage

```bash
hm set-host [<domain>] [--remove] [--no-database]
```

## Description

With a domain, or with the project's `DOMAIN` when none is given:

1. Records it as `DOMAIN` in `config/docker/properties.json`.
2. Points it at this machine in `/etc/hosts`.
3. Writes `https://<domain>/` to `web/secure/base_url` and `web/unsecure/base_url` with
   `bin/magento config:set`.

A project with no domain and none given is refused (`no_domain`, exit 2).

## Options

| Name            | Description                                                         | Example                             |
| --------------- | ------------------------------------------------------------------- | ----------------------------------- |
| `--no-database` | Only touch `/etc/hosts` (and the property), not `core_config_data`. | `hm set-host shop.test --no-database` |
| `--remove`      | Remove the entry this tool added for the domain, and nothing else.  | `hm set-host shop.test --remove`    |

Any other option is a usage error (exit 2).
