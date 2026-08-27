# mcp

Read-only tools for agents, over the Model Context Protocol.

```bash
hm mcp --config     # the entry to add to your MCP client
hm mcp              # the server itself, spoken over stdin and stdout
```

## What it is for

An agent's first minutes on a project go into finding out what it is working on: which PHP
version, which containers are up, what the URL is, why the page is failing. It does that by
running shell commands and reading output meant for people — which is why `hm describe`,
`hm list` and `hm doctor` learned to speak JSON.

A tool is different from a command in one respect that matters: the model is *told* it exists.
It asks for it by name instead of guessing, and it can be given boundaries the shell does not
have.

## The tools

| Tool | Answers |
|---|---|
| `describe_project` | Versions, services, URLs, state, deploy mode |
| `list_environments` | Every environment on the machine and what it belongs to |
| `check_environment` | What `hm doctor` checks, with results |
| `service_logs` | The last lines of one service's log |
| `database_query` | One read-only SQL statement |

Each one wraps a command that already answers in JSON, so there is a single implementation of
"what is this project" — the one with tests behind it.

## Nothing here changes anything

That is the point of doing the read-only half on its own. `hm exec`, `hm bash` and `hm mysql` are
[classified dangerous](permissions.md) precisely because they run whatever they are given. A list
of typed questions is the opposite shape, and it can be handed to an agent without deciding
anything else first.

The write half — cache clean, reindex, config set — is a separate decision, with its own review.

### `database_query` is a SELECT

The statement is checked before it runs. Comments are stripped first (a comment is where a second
statement hides), and then it must begin with `SELECT`, `SHOW`, `DESCRIBE` or `EXPLAIN`, contain
only one statement, and mention neither `INTO OUTFILE`, `INTO DUMPFILE` nor `LOAD_FILE` — those
read and write files on the host as the database user.

It errs towards refusing. A model that cannot run its query gets a sentence explaining why; a
model that can run `SELECT ... INTO OUTFILE` gets a file.

Results are capped at 200 rows, because a `SELECT * FROM sales_order` would otherwise be answered
with the whole context window.

## Wiring it up

```bash
hm mcp --config
```

prints the entry for a client's configuration, with the absolute command and this project's
directory:

```json
{
  "mcpServers": {
    "hm": { "command": "/path/to/hm/bin/run", "args": ["mcp"], "cwd": "/path/to/project" }
  }
}
```

It prints and writes nothing: a client's configuration has other servers in it, and editing
somebody's file to save them a copy and paste is not a good trade.

With Claude Code the same thing is done with `claude mcp add hm -- /path/to/hm/bin/run mcp`.

## If it does not answer

Everything the server prints on stdout is a protocol message, so a wrapped command writing to
stdout would be a parse error in the client and a server that appears to have died. Every command
it calls has its stderr redirected and its stdin closed for that reason.

To see a session by hand:

```bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
              '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | hm mcp
```

A tool that could not answer replies with a result marked as an error rather than a protocol
error: a model can read the sentence and try something else, where a JSON-RPC error is shown by
most clients as a broken server.
