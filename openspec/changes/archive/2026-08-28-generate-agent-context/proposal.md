# Write down what an agent otherwise invents

## Why

An agent starting on a Magento project needs half a dozen facts before it can do anything: the
PHP version, the container names, the storefront URL, the admin's real front name, how to run a
Magento command. None of it is written down in the project, so it works them out — from
`docker-compose.yml`, from `composer.lock`, from a guess — and works out the configured value
rather than the resolved one, or invents `/admin` and reports a 404 as a bug.

`hm describe --json` has answered all of it since 1.5. What is missing is that anybody thought to
run it: the agent has to be told the command exists, in a file it reads before it starts.

The same file has to say what **not** to read. `app/etc/env.php` holds the encryption key and the
database password, `var/log` holds whatever the site logged about real customers, and
`pub/media/customer` holds their uploads. An agent reading a project reads those by default, and
whatever it reads may end up in a request to a third party.

## What Changes

- **`hm ai-context`** writes the project's agent context: an `AGENTS.md` block with the resolved
  facts, how to run things, the exit-code contract, and what not to read or run.
- It regenerates **a delimited block**, so a hand-written `AGENTS.md` keeps everything around it.
  A `CLAUDE.md` that already exists is never touched — the one line to add is printed instead.
- It merges the MCP server entry into `.mcp.json`, so `hm mcp` is available without anybody
  copying JSON by hand.
- **The exclusion list is declared once**, in `data/ai-exclusions.json`, with a reason per entry,
  and it feeds both the generated context and `hm permissions`, which now emits `deny` rules for
  those paths.
- **`hm doctor` says when the block is stale**: it records the version and the facts it was
  generated from, so a context describing PHP 7.4 in a project that moved to 8.2 is a check that
  fails rather than an agent that is quietly wrong.
- No secrets are written. The generated block names the database, never its password.
