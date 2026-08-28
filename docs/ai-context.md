# ai-context

Writes into the project what an agent needs to know before it starts.

```bash
hm ai-context
```

## What it writes

| File | What happens to it |
|---|---|
| `AGENTS.md` | A delimited block is created or refreshed. Everything outside it is yours |
| `CLAUDE.md` | Created pointing at `AGENTS.md` if it does not exist. **Never modified if it does** |
| `.mcp.json` | The `hm` server entry is merged in, keeping any other servers |

## Why

An agent starting on a Magento project needs half a dozen facts before it can do anything: the
PHP version, the container names, the storefront URL, the admin's real front name, how to run a
Magento command. None of it is written down in the project, so it works them out — from
`docker-compose.yml`, from `composer.lock`, from a guess — and gets the *configured* value rather
than the resolved one, or assumes `/admin` and reports the 404 as a bug.

`hm describe --json` has answered all of it since 1.5. What was missing is that anybody thought to
run it. This writes that down where an agent reads it before it starts.

## What the block contains

- The project as it is now: name, Magento version, deploy mode, state, storefront and admin URLs,
  services.
- How to run things — `hm magento`, `hm composer`, `hm exec`, `hm logs`, `hm mysql -q` — and the
  one trap: `hm bash` opens a shell and takes no command.
- The output contract: JSON when stdout is not a terminal, and what each exit code means.
- What not to read, and what not to run without being asked.

**No secrets.** The database is named; its password is not. An agent that needs to connect runs
`hm mysql`, which knows the credentials without printing them.

## It says whether the data is real

The block carries one line that matters more than the rest:

> **This database has not been anonymised.** Treat every row as real personal data: do not quote
> customer names, emails, addresses, phone numbers or order contents in your output.

or, when it has been:

> Anonymised on 2026-08-28. It is safe to quote rows in your output.

The state comes from `hm masquerade` having been run, and it **expires**: restoring a snapshot,
cloning a template, importing a dump or transferring a database clears it, because whatever those
brought in has not been anonymised. Three states — anonymised, not, unknown — and unknown is never
treated as safe.

An agent obeys what it reads. Everywhere else in this tool that is a risk to be managed; here it
is the mechanism.

## What not to read

Declared once, in `data/ai-exclusions.json`, with a reason each:

| Path | Why |
|---|---|
| `app/etc/env.php` | The encryption key and the database password |
| `var/log`, `var/report` | Whatever the site logged about real customers |
| `pub/media/customer` | Files real customers uploaded |
| `vendor`, `generated`, `var/cache`, `pub/static` | Not source, and megabytes of context |

That one list has two consumers, and they work differently. The generated block **explains** it,
which is all you can do for an agent whose tooling enforces nothing. [`hm permissions`](permissions.md)
turns it into `deny` rules, which **refuse**. Keeping both kinds of entry in one file with a
stated reason is what stops somebody from tidying away the dangerous ones later.

## It says when it is out of date

A context that describes PHP 7.4 in a project that moved to 8.2 is worse than no context, because
an agent obeys it. The block records the tool version and a fingerprint of the facts it was built
from, and `hm doctor` compares that against the project as it is now:

```
✗ The agent context describes a different configuration than this project
  → hm ai-context
```

A project with no generated context is not a failure — the check simply mentions that the command
exists.

## Editing it

Write whatever you like around the block; it is left alone. Inside the markers is regenerated, so
put your own notes outside them.

If the closing marker is deleted, the command refuses rather than rewriting from the opening
marker to the end of the file, which would take your text with it.
