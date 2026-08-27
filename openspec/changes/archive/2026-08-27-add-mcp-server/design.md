# Design

## Why the server can be a shell script

The stdio transport is newline-delimited JSON-RPC: read a line, answer a line. There is no
framing to get wrong, no socket to manage and no concurrency — the client sends one request at a
time and waits. What is left is parsing and building JSON, which is `jq`, and calling commands,
which is what this tool is.

Writing it in the same language as everything else means the tools cannot drift from the commands
they wrap, and that the department can read them.

## Nothing but protocol goes to stdout

Everything the server prints on stdout is a JSON-RPC message. Every command it calls has its
stderr redirected and its output captured, because one stray warning printed by a wrapped command
would be a parse error in the client and a server that "just stopped working".

Errors have somewhere to go: a failed tool returns a result with `isError` set and the message in
its content, which is what a model can act on, rather than a JSON-RPC error, which most clients
surface as a broken server.

## Every tool wraps a command that already speaks JSON

`describe_project` is `hm describe --json`, `list_environments` is `hm list --json`, and so on.
The server unwraps the envelope and hands over `data`, so there is exactly one implementation of
"what is this project" — the one that has tests.

This is also why the read-only half comes first and alone: these commands are already classified
`safe` or `caution` by AI-02, and the tools inherit that classification instead of inventing a
second, quieter way to run something dangerous.

## `database_query` is a SELECT, not a database connection

The query is checked before it is run: comments are stripped, the statement must begin with
`SELECT`, `SHOW`, `DESCRIBE` or `EXPLAIN`, it may not contain a second statement, and
`INTO OUTFILE` and `INTO DUMPFILE` are refused because they write files as the database user.

The check is a whitelist and it errs towards refusing. A model that cannot run its query gets a
sentence explaining why; a model that can run `SELECT ... INTO OUTFILE` gets a file on the host.

Results come back as rows with column names, capped, because a `SELECT * FROM sales_order` would
otherwise be answered with the context window.

## `--config` prints, it does not install

Like `hm permissions`, it prints the entry and lets the person put it where it belongs. An MCP
client's configuration is a file with other servers in it, and editing somebody's configuration to
save them a copy and paste is not a good trade.
