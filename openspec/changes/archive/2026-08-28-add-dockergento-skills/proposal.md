# Skills that teach commands that exist

## Why

There are five Dockergento skills published in `hiberus-magento/ai-tools`, and the two most used
of them teach commands the tool has never had:

- `hm bash <command>` appears about 150 times across them. `bash.sh` understands only `-r`; any
  other argument is dropped and an interactive shell opens. An agent with no terminal hangs there
  or runs nothing at all.
- `hm bash -c <service>` (27 uses) was never a thing, and the services it names — `mysql`,
  `elasticsearch` — are called `db` and `search`.
- `hm mysql -e "..."` appears 53 times. The option parser accepts `-i`, `-q`, `-d` and `-a`; `-e`
  leaves through the error branch with exit code 2.
- Container names written by hand (`dockergento_php`) and the user `www-data`, where they are
  `<project>-phpfpm-1` and `app`.
- None of them mentions anything added since 1.5: `describe`, `list`, `doctor`, `logs`, `launch`,
  `db`, `worktree`, `verify`, `permissions`, `mcp`, or the `--json` contract they all speak.

This is not carelessness by whoever wrote them. They live in a repository that has no way of
knowing what the tool does, they were written once, and nothing checks them. Documentation that
nobody verifies drifts; documentation an agent reads and obeys drifts into an agent that opens a
shell and waits forever.

## What Changes

- Four skills live in this repository, in `skills/`, one per area of work: the environment and its
  lifecycle, the database, debugging, and branch environments with the agent tooling.
- They are short and they teach the commands that exist, the `--json` contract and the exit codes,
  because that is what an agent needs and none of it is written down for one today.
- **A test asserts they cannot drift**: every `hm ...` in every skill must name a command that
  exists, every option must be declared for that command, and the services they name must be
  services of the compose template.
- `hm ai-pull` installs them from the installed copy of the tool rather than downloading them, so
  the skills a person gets are the ones that match their version.
- `ai-tools` keeps the wider set — Magento, PHP, Hyvä — and stops being the place where
  Dockergento's own commands are described from memory.
