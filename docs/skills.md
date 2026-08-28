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

## Knowing what you have

```bash
hm ai-doctor
```

Lists every skill and agent installed in this project, where it came from, and which of five
states it is in:

| State | Means |
|---|---|
| `current` | Matches what was installed |
| `outdated` | The tool now carries a newer copy — `hm ai-pull` will update it |
| `modified` | Changed since installation — **the next pull overwrites it** |
| `custom` | The tool never installed it. It is yours, and it is safe |
| `missing` | Tracked, and no longer there |

`modified` is the state worth knowing about. `hm ai-pull` keeps its promise to preserve custom
skills by leaving alone what it did not install — so a skill somebody improved *in place*, without
renaming it, is lost on the next pull. Renaming it is what keeps it.

For the skills that came with the tool, `outdated` is answered offline, by comparing against the
copy the installed tool carries. For a downloaded repository there is no such copy, so those are
reported with their origin and their date and nothing is claimed about freshness — which is the
honest consequence of following a branch rather than a version.

It changes nothing. It is what you run before deciding whether `hm ai-pull --force` is safe.

## Adding or changing one

Edit the `SKILL.md` and run the suite. If you used a command or an option that is not declared,
the test says which one and in which skill — and the answer is usually that the declaration is
missing, not the skill.
