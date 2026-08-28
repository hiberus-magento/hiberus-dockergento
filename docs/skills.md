# skills

The skills that teach an agent to use this tool live in this repository, in `skills/`.

```bash
hm ai-pull    # installs them, along with whatever else is configured
```

## Why they are here

There were five Dockergento skills in `hiberus-magento/ai-tools`, and the two most used of them
taught commands the tool has never had:

- `hm bash <command>` appeared about 150 times. `bash.sh` understands only `-r`; any other
  argument is dropped and an interactive shell opens. An agent with no terminal hangs there.
- `hm bash -c <service>` was never a thing, and the services it named — `mysql`,
  `elasticsearch` — are called `db` and `search`.
- `hm mysql -e "..."` appeared 53 times, where the option is `-q`.
- Container names written by hand, and nothing at all from the last three versions.

Nobody was careless. They lived in a repository that has no way of knowing what this tool does,
they were written once, and **nothing checked them**. Documentation that nobody verifies drifts;
documentation an agent reads and obeys drifts into an agent that opens a shell and waits.

So they live next to the commands they describe, where two things are true that were not before.

**They are checked.** `tests/unit/skills_test.sh` extracts every `hm ...` from every skill and
asserts the command exists, the options are declared for it in `command_descriptions.json`, and
the services named are services of the stack. It is a whitelist: an option nobody declared fails
the suite. On its first run it found a gap — `hm down -v`, used everywhere and declared nowhere.

**They cannot separate from the source.** Adding a command already means editing
`command_descriptions.json`; the skill is now in the same tree and the same commit.

`ai-tools` keeps the wider set — Magento, PHP, Hyvä. What moved is only the part that describes
this tool.

## The four skills

| Skill | For |
|---|---|
| `dockergento-environment` | Start, stop, see what is running, reach a URL, read logs, diagnose |
| `dockergento-database` | Query, import, dump, snapshots, templates, anonymisation |
| `dockergento-debugging` | Xdebug, Varnish, caches and indexes, static checks, tests |
| `dockergento-agents` | Branch environments, MCP, permissions, the JSON and exit-code contract |

Split by area of work rather than one per command. An agent picks a skill by what it is trying to
do, which is why the previous set had seven hundred lines about Varnish and nothing about
`hm describe`.

Each is around a hundred lines. A skill is read to choose a command, not to learn shell
scripting.

## How they are installed

`hm ai-pull` installs them **from the installed copy of the tool**, not by downloading them.
That is the point of keeping them here: somebody running 1.5 gets the skills that came with 1.5,
describing the commands they actually have. Downloading `main` would be the same drift in a new
direction.

They are installed regardless of the skill types configured, because they describe the tool being
used rather than a technology somebody chose. Everything else is unchanged: the same platforms,
the same registration file, and a skill you wrote yourself is never overwritten without `--force`.

## Adding or changing one

Edit the `SKILL.md` and run the suite. If you used a command or an option that is not declared,
the test says which one and in which skill — and the answer is usually that the declaration is
missing, not the skill.
