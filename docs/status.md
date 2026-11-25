# status

How the environment in the current directory is doing right now, in one call: each container's
state and the last error its log reported, whether the URLs answer, Xdebug on or off, the deploy
mode, and whether the database is anonymised. Where [`describe`](describe.md) answers "what is
this environment?", `status` answers "does it work?" — the question asked after every change.

```bash
hm status          # a compact table
hm status --json   # machine readable
```

Readable output when stdout is a terminal, JSON when it is piped or redirected. It takes no
options. The [MCP server](mcp.md) offers the same answer as the `environment_status` tool.

## What it reports

| Key | Contents |
|---|---|
| `project`, `root`, `worktree` | Which environment |
| `state` | `running`, `partial` or `stopped`, as `describe` reports it |
| `mode` | Magento's deploy mode, as `describe` reports it: `default` when `env.php` sets no `MAGE_MODE` |
| `xdebug` | `on`, `off`, or `unknown` when the php container is not running |
| `anonymised`, `anonymised_at` | `yes` and when, or `unknown`: unknown is never treated as safe, and anything that replaces the database clears it |
| `services` | Each service with Docker's state, `running`, and `last_error`: the last line of its recent log that reports an error |
| `urls` | The base, admin, mail, RabbitMQ and search addresses, each with `answers` and the HTTP `code`, or the `error` when nothing answered |

```
shop  running  mode developer  xdebug off  anonymised 2026-11-25 10:00

  SERVICE         STATE        LAST ERROR
  db              running      -
  nginx           running      2026/11/25 10:01:00 [error] 7#7: upstream timed out
  phpfpm          running      -

  URL             ANSWERS      ADDRESS
  base            200          https://shop.local/
  admin           302          https://shop.local/admin_x1y2
```

## How it finds out

Everything `describe` knows, it reuses: the services and their state, the URLs (with the admin's
real front name), the mode, Xdebug and the anonymisation. On top of that it asks two things:

- **The last error.** The last 200 lines of each *running* service's log are read and the last
  line that says `error`, `fatal`, `critical`, `emergency`, `alert` or `panic` — as a whole word,
  so `error_reporting` in a configuration dump is not one, and a warning is not one either — is
  reported, cut at 300 characters. A stopped container is not asked. Only what the container
  wrote, on its stdout or its stderr, is read; Compose's own messages are not part of it. All
  running services are read at once, each with a five-second deadline: a log that does not
  arrive in time is reported as `(the log could not be read: it did not answer within 5s)`, and
  the rest of the answer does not wait for it.
- **Whether the URLs answer.** A `HEAD` request to each, all at once, with a three-second timeout.
  Any HTTP answer counts, a `500` or a redirect included: the question is whether a server is
  there, and the code says the rest. The certificate is not checked, because a development
  domain's certificate is the tool's own and may not be trusted by `hm` itself, and redirects are
  not followed. A `HEAD` answered `404` or `405` is asked again with a `GET`, with the same
  timeout and the same rules, and the `GET`'s code is the one reported: some servers only route
  `GET` — Mailhog answers `HEAD` with a `404` and its page to a browser. The body is not read.

A container's Docker healthcheck is not reported: the images' compose templates declare none.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, whatever the environment's state: a stopped environment is an answer |
| `2` | An argument was given |
| `3` | Docker could not be reached |
| `4` | Not a configured Dockergento project |
