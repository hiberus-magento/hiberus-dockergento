# logs

The logs of the project's containers, or of the services you name.

```bash
hm logs                       # everything
hm logs phpfpm                # one service
hm logs phpfpm nginx          # several
hm logs -f                    # follow
hm logs --tail 100 phpfpm     # the last hundred lines
hm logs --since 10m           # the last ten minutes
```

## It is a thin wrapper, on purpose

The output is somebody else's and can be endless with `-f`, so nothing gets between you and it.
What the wrapper adds is not having to know that Compose is underneath, and a readable error for
a service that does not exist:

```console
$ hm logs redsi
Error: This project has no service called 'redsi'
Try: hm logs db hitch mailhog nginx phpfpm rabbitmq redis search varnish
```

Exit code `5`, the service code. Compose would have answered with an error about YAML files.
The check only runs when you name a service: `hm logs` on its own has nothing to validate.

## Options

The options of `docker compose logs`, read by `hm` and handed to Compose as a library:

| Option | Meaning |
|---|---|
| `-f`, `--follow` | Follow the output |
| `-n N`, `--tail N` | The last N lines of each container |
| `--since T`, `--until T` | A time window: a timestamp or a duration such as `10m` |
| `-t`, `--timestamps` | Show timestamps |
| `--index N` | One replica of a scaled service |
| `--no-color` | No colour |
| `--no-log-prefix` | No service name in front of each line |

A value goes in the next word (`--tail 100`). Any other option is a usage error (exit 2).

## Flags before and after the command name

`logs` is a *transparent* command, like `mysql` and `composer`: its output is data. A flag
**before** the command name is the CLI's, a flag **after** it belongs to Compose.

```bash
hm --json logs db     # the CLI's flag — and the logs still come out raw, never wrapped
hm logs --json db     # logs' flag, and it has none: a usage error
```

The logs are never wrapped in the JSON envelope in either case. See
[the output contract](output.md).
