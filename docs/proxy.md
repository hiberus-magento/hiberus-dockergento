# proxy

One proxy per machine, so more than one project can be up at a time.

```bash
hm proxy status
hm proxy up
hm proxy down
```

## The problem it solves

Every project publishes ports 80, 443, 3306, 9200, 8025, 5672 and 15672 on the machine. There is
only one port 80, so **two Dockergento projects cannot be up at once**: you stop one to look at the
other. Comparing branches, reviewing somebody's work, running anything in parallel — all of it
becomes a sequence of several-minute starts and stops.

With the proxy, projects stop publishing anything and are told apart by their domain.

## Turning it on for a project

```bash
# in config/docker/properties.json
"USE_PROXY": "true"
```

```bash
hm setup -f     # regenerate
hm rebuild
```

It is **off by default**, and a project that does not turn it on behaves exactly as before.
Turning it off again is the same two commands with `false`.

The proxy starts on its own when you `hm start` a project that uses it. It is not stopped when you
stop that project: other projects may be relying on it.

## What changes for that project

| Before | After |
|---|---|
| `https://project.local` on the published 443 | `https://project.local` through the proxy |
| Mailpit on `localhost:8025` | `https://mail.project.local` |
| RabbitMQ on `localhost:15672` | `https://queue.project.local` |
| OpenSearch on `localhost:9200` | `https://search.project.local` |
| MySQL on `localhost:3306` | [`hm tunnel db`](tunnel.md) |
| Hitch terminating TLS | the proxy does it |

Hitch is dropped from the stack. It was there for one reason — giving Varnish the HTTPS it does not
have — and the proxy terminates TLS now. The chain goes from `hitch → varnish → nginx` to
`traefik → varnish → nginx`, which is closer to production, where TLS ends at the balancer.

## MySQL and AMQP are not routed, and why

They are not HTTP: there is no `Host` header, and the name you type when connecting is only used to
resolve an address and is gone by the time the connection arrives. Traefik refuses outright:

```
invalid rule: "HostSNI(`project.local`)", has HostSNI matcher, but no TLS on router
```

Without TLS it can only accept `HostSNI(*)`, which means one listening port per service — the very
problem the proxy exists to solve. Forcing it would mean TLS on MariaDB and desktop clients
configured by hand. So they are reached with [`hm tunnel`](tunnel.md) instead, which costs one
command and nothing else.

## It needs ports 80 and 443

A project that does **not** use the proxy publishes those itself, so the two cannot be up at the
same time. The proxy says who is in the way rather than letting Docker say it:

```console
$ hm proxy up
Error: 'other-project-varnish-1' is already using port 80 or 443, which the proxy needs
Try: Stop that environment first: it does not go through the proxy
```

## Requirements

- **Docker Compose 2.24 or newer.** The overlay that removes a project's published ports uses
  `!reset`, which arrived in that version. It is checked before generating anything.
- `mkcert`, already required by `hm ssl`, for the wildcard certificate.

## Certificates

Each project gets a wildcard certificate for `*.<domain>`, because a certificate per domain does
not cover the subdomains the auxiliary interfaces live on. They are stored with the proxy in
`~/.hm/proxy/certs/` and loaded by Traefik automatically.

## Where it lives

`~/.hm/proxy/`, as an ordinary Compose project called `hm-proxy`. It shows up in `docker ps` under
a name you recognise and can be stopped by hand without anything magic happening. Its dashboard is
bound to loopback on port 8080.

The directory has to be under your home: Colima only mounts `$HOME` and `/tmp/colima`, and a bind
mount outside those appears **empty** inside the container, with no error.
