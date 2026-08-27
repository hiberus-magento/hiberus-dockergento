# A read-only MCP server for the project

## Why

An agent working on a Magento project spends its first minutes finding out what it is working on:
which PHP version, which containers are up, what the URL is, why the page is failing. It does that
by running shell commands and reading their human-readable output — which is why `hm describe`,
`hm list` and `hm logs` learned to speak JSON in the first place.

Even so, every agent has to be told those commands exist, and every agent invents its own way of
asking. A tool that is *described* to the model — name, arguments, what it answers — is asked for
by name instead of guessed at, and it can be given boundaries the shell does not have.

The boundary is the point of doing the read-only half first. `hm exec` and `hm mysql` are already
classified dangerous because they run whatever they are given (AI-02). An MCP server made of typed
tools is the opposite shape: a fixed list of questions with fixed answers, where "run this SQL"
means a `SELECT` and nothing else.

## What Changes

- **`hm mcp`** speaks the Model Context Protocol over stdin and stdout, offering read-only tools:
  - `describe_project` — what this project is: versions, services, URLs, state
  - `list_environments` — every environment on the machine and what it belongs to
  - `service_logs` — the last lines of one service's log
  - `database_query` — one `SELECT`, refused if it is anything else
  - `check_environment` — the diagnostics `hm doctor` runs
- Every tool is a wrapper over a command that already answers in JSON, so the server adds no
  second source of truth about the environment.
- **`hm mcp --config`** prints the entry to add to an MCP client's configuration, because the
  common failure of an MCP server is not being wired up.
- Nothing here can change the environment. The write half (AI-04) is deliberately a separate
  decision, with its own review.
