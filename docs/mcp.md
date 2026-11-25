# mcp

Tools for agents, over the Model Context Protocol: read-only by default, a few that change the
environment with `--write`.

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
| `environment_status` | What [`hm status`](status.md) says: containers and their last error, whether the URLs answer, xdebug, mode, anonymisation |
| `service_logs` | The last lines of one service's log |
| `database_query` | One read-only SQL statement |
| `xdebug` | Whether xdebug is on |
| `after_change` | Given the files changed, the caches to clean and whether `di:compile` or `setup:upgrade` is needed |
| `magento_exceptions` | The tail of `var/log/exception.log`, only when the database is anonymised |

Each one wraps a command that already answers in JSON, so there is a single implementation of
"what is this project" — the one with tests behind it.

`describe_project`, `list_environments`, `check_environment` and `environment_status` answer with that JSON twice: as
text, for clients that read only text, and as `structuredContent`, described by the tool's
`outputSchema`, for clients that validate it and read fields without parsing. The schemas name the
top-level fields and leave the rest open, because the commands grow a field now and then and a
schema that pinned every one would turn that into an answer the client rejects.

### The Magento loop

Four tools cover what an agent does between one edit and the next. `run_tests` is one of the
[write tools](#the-write-half-off-by-default); the other three answer without `--write`.

**`run_tests`** (with `--write` only) runs `hm test-unit` or `hm test-integration` (`suite`, default `unit`) with PHPUnit
writing its JUnit report into the container's `/tmp`, reads the report back through `hm exec` —
which works whether the code is mounted or in a volume — and removes it. The answer has `passed`,
`exit_code`, the counts (`tests`, `failures`, `errors`, `skipped`) and each failing test with its
message, up to twenty. When there is no report, a bootstrap that died before the first test for
example, the answer is the exit code and the last forty lines of output, with `source: "exit
code"` and no counts invented. `path` is a test file or directory; it goes into a command line
inside the container, so only a plain relative path of letters, digits, `_`, `-`, `.` and `/` is
accepted, never one with `..`. Running tests takes `--write`: it runs the project's own code, and
the integration suite installs into and empties a database of its own, so its annotations say it
is destructive and not idempotent.

**`xdebug`** says whether xdebug is `on`, `off` or `unknown`, as `hm describe` reads it. With
`--write` it also takes `state: "on" | "off"` and runs `hm debug-on` or `hm debug-off`, which
restart the php container. A command that fails is a tool error (`isError`) with its exit status
and what it printed.

**`after_change`** takes the changed files and says what Magento needs before it shows them. It
only says; with `--write` and `apply: true` it also does it. The rule is by kind of file, the
first match deciding:

| Changed | Needs |
|---|---|
| `etc/db_schema.xml`, `etc/db_schema_whitelist.json` | `setup:upgrade` |
| a PHP file under `Setup/` (patches, install and upgrade scripts) | `setup:upgrade` |
| `etc/module.xml`, `registration.php`, `theme.xml`, `app/etc/config.php` | `setup:upgrade` |
| `etc/**/di.xml`, `etc/extension_attributes.xml` | `di:compile` and the `config` cache |
| any other `etc/**.xml`, `etc/schema.graphqls` | the `config` cache |
| `*.phtml` | `block_html`, `full_page` |
| `layout/*.xml`, `page_layout/*.xml` | `layout`, `block_html`, `full_page` |
| `i18n/*.csv` | `translate`, `full_page` |
| anything under `web/`, `requirejs-config.js` | static files, and `full_page` |
| anything else — a PHP class among them | nothing |

A class needs nothing because developer mode reads it on every request and generates what it
needs; a changed constructor or plugin is the exception a path cannot show. "Static files" means
`pub/static` and `var/view_preprocessed` hold an older copy, which no cache type covers: the answer
says so (`static: true`) and nothing removes them.

The answer lists the union (`cache_types`, `di_compile`, `setup_upgrade`, `static`), each path with
what it needs, and the commands that would apply it, in the order they would run: `setup:upgrade`,
then `setup:di:compile`, then `cache:clean` with the types. Applying runs them in that order and
stops at the first that fails, which makes the answer an error. **`setup:upgrade` is never run
unless `setup_upgrade: true` is passed as well**, because it changes the database schema and
data; when it is needed and not asked for, it is listed under `skipped`. `setup_upgrade: true`
without `apply` runs nothing, and the answer's `note` says it was ignored. Because it can run
`setup:upgrade`, the `--write` variant's annotations say it is destructive and not idempotent.

**`magento_exceptions`** is the tail of `var/log/exception.log` (`lines`, default 50, at most 200),
read inside the container under the Magento directory. It is refused unless `hm describe` reports
the database anonymised — the same state the agent context and [`hm status`](status.md) report,
cleared by anything that replaces the database. And because `data/ai-exclusions.json` marks
`var/log` as holding real customers' data, only the entries logged after the anonymisation are
shown (from the minute after it, since the record has minutes); the ones before are counted and
left out, as are lines that cannot be dated. The record is this machine's local time, and each
entry's time (UTC, with its offset when Magento wrote one) is moved into that zone before the two
are compared. A log that is not there is answered as empty; one that cannot be read — the
container is not running, for example — is a tool error with the exit status.

### One server, several branch environments

Every tool except `list_environments` takes an optional `environment`: the name of a
[branch environment](worktree.md) of this project, as `hm worktree list` shows it. The tool then
runs its command in that environment's directory, so one server configured for the main checkout
answers about each worktree too. Left out, the tool answers about the directory the server was
started in, as before.

The name is looked up through `hm worktree list`, and only matched: the directory comes from the
registry, never from the model. A name that is not there is a tool error listing the ones that
are; one whose directory was deleted says so and points at `hm clean`.

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

A backslash inside a quoted string is refused too. Whether it escapes the quote after it depends
on the server's SQL mode (`NO_BACKSLASH_ESCAPES`, and `ANSI_QUOTES` for `"..."`), which the check
cannot see; read the other way, a comment or a second statement can sit where the check sees text.
So no reading is trusted, and the refusal says how to write the query without one:

| Instead of | Write |
|---|---|
| `'it\'s'` | `'it''s'` (double the quote) |
| a literal backslash, `'a\\b'` | `CONCAT('a', CHAR(92), 'b')` |
| `LIKE '%\_%'` | `LIKE '%!_%' ESCAPE '!'` |

It errs towards refusing. A model that cannot run its query gets a sentence explaining why; a
model that can run `SELECT ... INTO OUTFILE` gets a file.

The check reads the statement, not what it does: a `SELECT` that calls a stored function which
inserts a row passes it. So a statement that passes is sent after
`SET SESSION TRANSACTION READ ONLY; START TRANSACTION READ ONLY;`, and the database itself refuses
the writes the text could not show. The session setting is there as well as the transaction
because a statement that commits implicitly starts a new transaction in the session's mode. This
is a second layer behind the check, not a replacement for it: the check still answers first, and
a refused statement never reaches the database.

Results are capped at 200 rows, because a `SELECT * FROM sales_order` would otherwise be answered
with the whole context window.

## The write half, off by default

```bash
hm mcp --write
```

adds five tools, and lets two of the read tools do what they describe:

| Tool | Does |
|---|---|
| `run_tests` | Runs the unit or integration tests, optionally under a path, and says whether they passed |
| `cache_flush` | Flushes the whole cache |
| `cache_clean` | Cleans the cache types named |
| `reindex` | Rebuilds every index, or the one named |
| `config_set` | Sets a configuration value at default scope |
| `xdebug` | With `state`, turns xdebug on or off |
| `after_change` | With `apply`, cleans the caches and runs `di:compile` when the files need it; `setup:upgrade` only with `setup_upgrade` as well |

With the flag the server's instructions, sent to the client when it connects, name the tools that
can change the environment, read from each tool's `readOnlyHint`; without it they say that nothing
here changes anything. Without it, `xdebug` and `after_change` are offered as their read-only
selves, with no `state` or `apply` in their schema, and asking them to change something anyway is
refused with a sentence saying it takes `--write`.

Without the flag the five **do not exist** — they are absent from the tool list, not refused when
called. A tool that exists and says no is worse than no tool: the model sees it, plans around it,
reads the refusal and reaches for a shell.

### This is a smaller permission, not a bigger one

The instinct is that letting an agent write is a widening. Here it is the opposite. An agent that
has to flush a cache is given `hm magento` today, which runs anything Magento can do —
`setup:upgrade` included. A handful of typed tools replace that with a handful of things it can do
and nothing else.

Which is why the line is where it is. **Composer, `hm setup`, database imports and anything that
removes an environment are not offered at all**, with or without the flag. They are not slow
versions of the tools above; they are the operations whose failure costs an afternoon, and they
belong to a person at a terminal. `di:compile` and `setup:upgrade` are reachable only through
`after_change`, only for files that need them, and `setup:upgrade` only when asked for by name on
top of `apply`.

`config_set` checks that the path looks like `section/group/field` before it passes it on. That is
not a boundary against a hostile model and does not pretend to be one — it stops a plausible
mistake from becoming a command argument.

## Progress

`reindex` can take minutes. When a `tools/call` for it carries a `progressToken` in its `_meta`, the
server sends `notifications/progress` while the indexer runs, as the
[2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/basic/utilities/progress) and
[2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/utilities/progress)
revisions define them, then the response:

```json
{"jsonrpc":"2.0","method":"notifications/progress","params":{"progressToken":"abc","progress":0,"message":"Running hm magento indexer:reindex"}}
{"jsonrpc":"2.0","method":"notifications/progress","params":{"progressToken":"abc","progress":1,"message":"Catalog Search index has been rebuilt successfully in 00:00:07"}}
```

- One notification when the command starts and one per line it prints: the indexer's own line per
  index is the only measure of how far it has got, so `progress` counts lines and there is no
  `total`.
- The token is a string or an integer; anything else, or no token, means no notifications, and the
  call is answered exactly as before.
- The other tools answer in a moment and send none. `message` is part of the revisions from
  `2025-03-26`; a `2024-11-05` client receives it as a field it does not know.

From the command line the same idea is [`--progress=jsonl`](progress.md#for-a-program---progressjsonl).

## Wiring it up

```bash
hm mcp --config           # read-only
hm mcp --config --write   # with the write tools
```

prints the entry for a client's configuration, with the absolute command and this project's
directory:

```json
{
  "mcpServers": {
    "hm": { "command": "/path/to/hm/bin/hm", "args": ["mcp"], "cwd": "/path/to/project" }
  }
}
```

It prints and writes nothing: a client's configuration has other servers in it, and editing
somebody's file to save them a copy and paste is not a good trade.

With Claude Code the same thing is done with `claude mcp add hm -- /path/to/hm/bin/hm mcp`.

## If it does not answer

Everything the server prints on stdout is a protocol message, so a wrapped command writing to
stdout would be a parse error in the client and a server that appears to have died. Every command
it calls has its stderr redirected and its stdin closed for that reason.

### Protocol versions

The server speaks both kinds of revision. A client of the handshake revisions (`2025-06-18`,
`2025-03-26`, `2024-11-05`) opens with `initialize` and is answered as it always was. A client of
the stateless revision `2026-07-28` names its version in every request's `_meta` and is answered in
it: results carry `resultType`, the server's identity in `_meta`, and, for `tools/list`, how long
the client may cache it. Such a client can call `server/discover` first to learn the versions,
the capabilities and the instructions in one request. A request naming a version the server does
not speak is refused with `UnsupportedProtocolVersionError` (code `-32022`), which lists the ones
it does.

To see a session by hand:

```bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
              '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | hm mcp
```

or, in the stateless revision:

```bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}' | hm mcp
```

A tool that could not answer replies with a result marked as an error rather than a protocol
error: a model can read the sentence and try something else, where a JSON-RPC error is shown by
most clients as a broken server.
