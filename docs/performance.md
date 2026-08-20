# Performance

In a Bash CLI, performance is **how many processes you spawn**. There is no algorithm to
optimise here: there are processes not to spawn. On a developer machine a `jq` costs about
36 ms and a call to Docker between 70 and 190 ms, so a loop that asks the same question
forty times is measured in seconds.

## Where it went

| Path | Before | After |
|---|---|---|
| `hm --help` | 5 660 ms, **143 `jq`** | **~390 ms, 3 `jq`** |
| `hm doctor` | 3 850 ms, 12 checks in series | **~1 300 ms, in parallel** |
| `hm list` | 1 160 ms | ~890 ms |
| `hm describe` | 1 220 ms | ~1 160 ms |
| startup floor (`hm --version`) | 300 ms | ~280 ms |

## What was done

**One query instead of one per item.** Listing the commands asked `jq` three questions per
command against the same 13 KB file. Now a single invocation returns the whole table and the
loop happens in Bash.

**Nothing is computed until someone needs it.** Detecting Docker Compose used to run
`docker compose version` twice on every invocation, ~190 ms each. Reading the Magento
version out of a 1.6 MB `composer.lock` costs 77 ms and only matters when containers are
created. Both are resolved on demand now.

**Two disk caches, in `$HOME` and never in the project.** Docker Compose detection and its
version are cached against the modification time of the docker binary: they only change when
Docker is reinstalled. Validation of the compose configuration is cached against the
modification time of the compose files.

They live in `~/.hm/cache/`, not in the project, because `config/docker/` is versioned in
real projects: a cache file there would show up in every `git status`, end up committed, and
force a `.gitignore` entry in every repository.

**Checks in parallel.** The twelve diagnostics are independent and spend their time waiting
on Docker. They run concurrently and are read back in file-name order, so the report is
identical to the sequential one.

## Budgets

`tests/performance/budget_test.sh` fails if a path goes over budget:

| Path | Budget |
|---|---|
| `hm --help` | 800 ms |
| `hm --version` | 600 ms |
| `hm doctor` | 2 500 ms |

The budgets are loose on purpose. The same `docker compose config -q` measured 72 ms warm
and 325 ms cold on the same machine, so a tight budget would fail for reasons that have
nothing to do with the code. These exist to catch an order-of-magnitude regression — the
kind that turns `hm --help` back into a six second wait.

Set `HM_SKIP_PERF=1` to skip them on a machine too loaded to measure anything.

## A caveat worth keeping in mind

The cost of an operation measured in isolation is not its cost inside the flow. Merging
three `git rev-parse` calls into one looked like an 86 ms saving when the calls were timed
separately; measured A/B on the real command it was **~14 ms**, below the noise band. Each
isolated call was paying a cold start that the real flow does not pay.

Measure the command, not the piece.
