# An agent's environment should not hold real customer data

## Why

A Magento development database is a copy of production: names, addresses, emails, phone numbers
and order history of real people. Anonymising it has always been good practice, and like most good
practice it happened when somebody remembered.

An agent changes what that means. It reads the database — `database_query` is one of the MCP tools
— and what it reads goes to a model, over a network, outside the company. That is no longer
untidiness; it is personal data leaving the building, and it is the kind of thing the GDPR has an
opinion about.

`hm masquerade` has existed all along. Three things were missing: nothing records whether it was
ever run, nothing notices when a database is replaced afterwards, and nothing runs it at the one
moment when it is obviously the right thing to do — creating an environment for an agent to work
in.

There is also a plain bug in the way: `masquerade_run` allocates a terminal unconditionally, so
running it from anything that is not a person at a keyboard fails with `the input device is not a
TTY`. It has never been usable from CI or from a script.

## What Changes

- **The state is recorded.** A successful anonymisation is written down, with its date, outside
  the checkout.
- **And invalidated when it stops being true.** Restoring a snapshot, cloning a template,
  importing a dump or transferring a database clears it: whatever those bring in has not been
  anonymised.
- **`hm worktree add --profile=agent` anonymises the cloned database by default.** `--no-anonymise`
  skips it, for the person who is reproducing a bug that only happens with the real data.
- **`hm describe` reports it**, and the generated agent context says it in words the agent will
  read: either the date it was anonymised, or that every row must be treated as real personal
  data.
- **`hm doctor` warns** when an environment marked as an agent's holds a database nobody has
  anonymised.
- `hm masquerade` works without a terminal.
