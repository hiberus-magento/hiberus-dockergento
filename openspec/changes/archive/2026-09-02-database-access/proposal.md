# `mysql` in Go, and the capability underneath it

## Why

`hm mysql -q` is what a script and an agent use to ask the database anything, and it is also how
the tool itself will read the project's domains out of `core_config_data` once the steps that run
after `start` on Linux are ported. `install` needs it too, and so does half of `db`.

So what this ports is not really a command. It is a capability — run a statement against this
project's database and give back what it says — with the command sitting on top of it. Anything
else means three implementations of "find the database container and talk to it", and the first
time one of them changes, two of them are wrong.

## What Changes

- **`hm mysql` is Go**: a statement, a session, or a dump on the input, with the same output, the
  same exit codes and the same refusal when the database is not running.
- **The statement travels in the environment**, not in the command, which is what the shell
  implementation does and for a reason worth keeping: a query is full of quotes and backticks, and
  passing it as an argument through a shell is a quoting bug waiting for the right query. There is
  a test with both in it.
- **The container is found by label**, not by name. `docker ps -f name=db` matches by substring
  across every project on the machine, so with two environments up it can return several ids or
  another project's database.
- **Capture and attach are two different things**, and both are needed: an interactive client
  wants the terminal, a query whose answer somebody reads wants the bytes. The second is a new
  adapter over the daemon's exec.

## What stays in shell

`hm mysql -i`. The import does not only import: it cleans DEFINER clauses, optionally anonymises,
and then configures Magento for local development — which prompts for a domain and writes into the
project. That sequence stays whole where it is until the things it depends on are ported, and the
condition that routes it there is one function with a test.
