---
name: dockergento-environment
description: Work with a Hiberus Dockergento (hm) Magento environment - find out what a project is, start and stop it, reach its URLs, read its logs and diagnose it. Use whenever a repository has a config/docker/properties.json and you need the environment running or need to know its state.
---

# Dockergento: the environment

`hm` runs a Magento 2 project in Docker. Every command below is run from the project directory.

## Find out what you are working with, before anything else

```bash
hm describe --json     # this project: versions, services, URLs, state, deploy mode
hm list --json         # every environment on this machine
hm doctor --json       # what is wrong, with a hint per check
```

`hm describe` answers with the environment stopped, so it is the first thing to run and never the
thing that fails. Do not read `docker-compose.yml`, `app/etc/env.php` or `composer.lock` to work
out versions, ports or URLs — this answers with what the machine actually resolved.

## The output contract

Every command that reports something takes `--json`, and **uses it by default when stdout is not
a terminal**, which is your case. The shape is always the same:

```json
{"ok": true, "schema_version": 1, "command": "describe", "data": { }}
```

Errors go to stderr with `ok: false`, a `type`, a `message` and a `hint`. Exit codes are worth
branching on:

| Code | Means |
|---|---|
| 0 | Fine |
| 2 | The command was called wrong |
| 3 | Docker is not running |
| 4 | This directory is not a Dockergento project |
| 5 | A service is not running |
| 6 | Refused on purpose — read the message before retrying |

**A 6 is not a failure to route around.** It is the tool saying the action would destroy something
or act on the wrong environment.

## Lifecycle

```bash
hm start               # bring it up
hm start -s            # and stop every other environment first
hm stop                # leave it stopped, keep the data
hm stop --snapshot     # take a copy of the database first
hm restart nginx       # one service
hm rebuild             # recreate the containers with the current configuration
```

`hm down` removes the containers. **`hm down -v` also removes the volumes, which is the
database.** Never run it to "clean up" — ask the person first; `hm stop` is what you want.

## Reaching it

```bash
hm launch              # open the storefront
hm launch --admin      # the admin, at its real front name, not /admin
hm launch --json       # the URL, without opening a browser
hm logs -f nginx       # follow one service
hm logs -n 200 phpfpm  # the last 200 lines
hm tunnel search       # a temporary way in to a service with no address of its own
```

Services are named as in the compose file: `phpfpm`, `nginx`, `db`, `search`, `redis`, `varnish`,
`rabbitmq`. There is no `mysql` service and no `elasticsearch` service — they are `db` and
`search`.

## Several projects at once

```bash
hm proxy status        # is the global proxy up
hm proxy up            # bring it up
```

With the proxy, projects are reached by name instead of by port and several can run at the same
time. Without it, only one can hold ports 80 and 443, which is what `hm start -s` is for.

## Running things inside the containers

```bash
hm magento cache:flush         # Magento CLI
hm composer install            # Composer, with the vendor synchronisation macOS needs
hm exec php -v                 # any command, in the php container
hm exec -r chown -R app:app var  # as root
```

`hm bash` opens an interactive shell and takes no command — only `-r`. To run something, use
`hm exec`. And use `hm magento` and `hm composer` rather than running `bin/magento` or `composer`
through a shell: on macOS the code lives in a volume and those two do the copying that keeps the
host and the container in step.

## What not to do

- Do not invent container names. If you need one, `hm describe --json` has them.
- Do not edit `docker-compose.yml` by hand: it is generated. Change
  `config/docker/properties.json` and run `hm setup -f`.
- Do not run `hm down`, `hm purge` or anything that removes data without being asked to.
