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

Enabling the write half is a separate decision, taken once, in the place where such decisions
belong: the client's configuration, by a person. See below.

### `database_query` is a SELECT

The statement is checked before it runs. Comments are stripped first (a comment is where a second
statement hides), and then it must begin with `SELECT`, `SHOW`, `DESCRIBE` or `EXPLAIN`, contain
only one statement, and mention neither `INTO OUTFILE`, `INTO DUMPFILE` nor `LOAD_FILE` — those
read and write files on the host as the database user.

It errs towards refusing. A model that cannot run its query gets a sentence explaining why; a
model that can run `SELECT ... INTO OUTFILE` gets a file.

Results are capped at 200 rows, because a `SELECT * FROM sales_order` would otherwise be answered
with the whole context window.

## The write half, off by default

```bash
hm mcp --write
```

adds four tools, and only these four:

| Tool | Does |
|---|---|
| `cache_flush` | Flushes the whole cache |
| `cache_clean` | Cleans the cache types named |
| `reindex` | Rebuilds every index, or the one named |
| `config_set` | Sets a configuration value at default scope |

Without the flag they **do not exist** — they are absent from the tool list, not refused when
called. A tool that exists and says no is worse than no tool: the model sees it, plans around it,
reads the refusal and reaches for a shell.

### This is a smaller permission, not a bigger one

The instinct is that letting an agent write is a widening. Here it is the opposite. An agent that
has to flush a cache is given `hm magento` today, which runs anything Magento can do —
`setup:upgrade` included. Four typed tools replace that with four things it can do and nothing
else.

Which is why the line is where it is. **`setup:upgrade`, Composer, `di:compile`, database imports
and anything that removes an environment are not offered at all**, with or without the flag. They
are not slow versions of the tools above; they are the operations whose failure costs an
afternoon, and they belong to a person at a terminal.

`config_set` checks that the path looks like `section/group/field` before it passes it on. That is
not a boundary against a hostile model and does not pretend to be one — it stops a plausible
mistake from becoming a command argument.

## Wiring it up

```bash
hm mcp --config           # read-only
hm mcp --config --write   # with the four write tools
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
