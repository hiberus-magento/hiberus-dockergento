# share

Gives this project a temporary public address.

```bash
hm share          # open it; Ctrl-C closes it
hm share --stop   # close one left open
```

## What it is for

**Showing progress without deploying.** A client or somebody in QA wants to see how something is
going, and the alternative is deploying to a shared environment — waiting, treading on somebody
else's work, or standing up an environment for a ten-minute demo.

**Receiving real webhooks.** Payment gateways, ERPs and marketplaces post to a public URL. They
cannot reach a local environment, so it gets tested blind or on a shared environment that is not
the one being worked on.

## It puts your environment on the internet

That is the point, and it is worth saying plainly. While it is open, anyone with the address
reaches the project — the admin panel and whatever data is in it. So it asks first:

```console
$ hm share

This puts 'my-project' on the public internet.
Anyone with the address reaches it — the admin panel and the data in it.

Share it? [y/N]:
```

It runs in the foreground: the tunnel lasts as long as the command, and Ctrl-C closes it. If the
window is closed instead, `hm share --stop` clears what was left, and the next `hm share` does too.

## Two things to expect

**The address changes every time.** These are Cloudflare quick tunnels: anonymous, free, and
without an account or credentials to hand around. A stable address would need a named tunnel over a
domain of the company's, which is not what this does.

**Magento's links still point at your local domain.** Magento builds absolute URLs from `base_url`,
so pages render but their links lead back to `project.local`. For receiving webhooks that does not
matter at all — which is the case where this helps most. For a demo somebody will click through,
change the base URL first:

```bash
hm magento config:set web/unsecure/base_url https://the-address.trycloudflare.com/
hm magento config:set web/secure/base_url   https://the-address.trycloudflare.com/
hm magento cache:flush
```

Remember to put it back afterwards. That is not done for you on purpose: writing to somebody's
database for a demo is worse than explaining the command.

## Why Cloudflared and not ngrok

ngrok has been blocked at the company before. Cloudflare quick tunnels need no account and were
verified to work from the company network before this was built.

## It does not need the proxy

It works the same whether the project publishes its ports or goes through
[the proxy](proxy.md) and publishes none: the tunnel joins the project's network and reaches the
web service by name.

Only HTTP is shared. Exposing a database to the internet is not a feature.
