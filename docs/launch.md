# launch

Open the project in the browser.

```bash
hm launch              # the storefront
hm launch --admin      # the admin panel
hm launch --mailhog    # the mail catcher
hm launch --rabbitmq   # the message queue
hm launch --search     # the search engine
```

The addresses are the ones [`describe`](describe.md) reports, so there is one definition of what
this project's URL is.

## Where there is nowhere to open

The address itself is the useful answer in a script, over SSH, or on a machine with no desktop:

| Situation | What happens |
|---|---|
| A terminal, with `open` or `xdg-open` | Opens the browser and says what it opened |
| `--json`, or output redirected | Writes the address, opens nothing |
| No opener on the machine | Writes the address and explains |

```bash
hm launch --json | jq -r .data.url
```

## One destination at a time

Two destinations in one invocation is a usage error (exit code `2`), not a request for two tabs:
`hm launch --admin --search` is far more likely to be a typo.

## What it does not do

It does not start the environment. Opening a browser against a stopped environment is not what
was asked for, and starting it by surprise is worse. A project with no domain configured says so
instead of opening something invented:

```console
$ hm launch
Error: This project has no domain configured, so it has no address to open
Try: hm setup --domain=project.local
```
