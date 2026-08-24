# Mail

Everything a local environment sends is captured, never delivered. Two catchers are supported and
a project chooses one:

| | Mailhog | Mailpit |
|---|---|---|
| Status | **unmaintained** | actively maintained |
| SMTP | 1025 | 1025 |
| Web interface | 8025 | 8025 |
| Default | yes | no |

They listen on the same ports, so Magento's configuration, the published ports and the address
you have bookmarked are the same either way.

## Opening the mailbox

```bash
hm launch --mail       # whichever the project uses
```

`--mailhog` and `--mailpit` do the same thing. `hm describe --json` reports the address under
`urls.mail`, and under `urls.mailhog` as well for anything that was already reading that key.

## Choosing it for a new project

```bash
hm setup --mail=mailpit
```

Interactively `hm setup` asks, with Mailhog as the default answer. Accepting the default records
nothing, so the project stays exactly as projects were built before this choice existed.

## Switching an existing project

Nothing switches on its own. When you want to:

```bash
# in config/docker/properties.json
"MAIL_SERVICE": "mailpit"
```

```bash
hm setup -f     # regenerate the compose files
hm rebuild      # recreate the containers
```

**Mail captured until then is lost.** Neither catcher persists anything: the messages live in the
container.

## Why Magento does not need reconfiguring

A Magento that was installed against this environment has `mailhog` recorded as its SMTP server.
Both services answer to that name on the Docker network — Mailpit through a network alias — so
mail keeps being delivered after a switch without touching Magento's configuration.

What does change is the service's name in Compose, which is deliberate: `hm logs mailpit` is what
you type when Mailpit is what is running.

## The image is published by hand

`hiberusmagento/mailpit` is pushed to the registry manually, outside the tool's release. Until
that has happened, choosing Mailpit points the project at an image Docker cannot pull, and
`hm doctor` says so rather than letting `up` fail halfway:

```console
$ hm doctor
✗ The mail catcher image cannot be obtained: hiberusmagento/mailpit:1
  That image is published manually; ask whoever maintains the registry, or set
  MAIL_SERVICE back to mailhog
```
