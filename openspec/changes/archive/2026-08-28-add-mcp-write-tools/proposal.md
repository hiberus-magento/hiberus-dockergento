# The write half of the MCP server, bounded

## Why

The read-only server answers questions. The work an agent actually does on a Magento project also
involves three or four actions it has to perform constantly and that nobody would call risky:
clean a cache, flush it, reindex, set a configuration value. Today it does them through
`hm magento`, which means through a shell — allowed to run anything, classified dangerous for
exactly that reason.

A typed tool for each of those is a smaller permission than a shell, not a larger one. That is the
whole argument for this change: `cache_flush` can flush a cache and cannot do anything else, where
`Bash(hm magento:*)` can run `setup:upgrade` on a Friday afternoon.

## What Changes

- **`hm mcp --write`** adds four tools: `cache_clean`, `cache_flush`, `reindex` and `config_set`.
- **Without the flag they do not exist.** Not refused at call time — absent from `tools/list`, so
  a model never sees them. Turning them on is a decision a person takes once, when they wire the
  server up.
- Every tool declares what it does through the protocol's own annotations, so a client that shows
  "this tool modifies your project" can show it.
- **What stays out**: `setup:upgrade`, `composer`, `di:compile`, database imports and anything
  that removes an environment. They are not slow versions of the tools above; they are the
  operations whose failure costs an afternoon, and they belong to a person at a terminal.
- `config_set` takes a configuration path and a value, and refuses anything that is not a path.
- `hm mcp --config --write` prints the client entry with the flag in it.
