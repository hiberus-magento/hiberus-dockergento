# Getting started

From an installed `hm` ([installation](installation.md)) to a running store, then the features you
reach for once one project is up.

## An existing project

From your project's directory:

```bash
cd <your_project>
hm setup                    # answers a few questions and writes the Docker configuration
hm start                    # brings the environment up
hm mysql -i dump.sql        # imports a database and points the store at this machine
hm launch                   # opens the storefront in the browser
```

`hm setup` writes `config/docker/properties.json`, `docker-compose.yml` and the platform overlays
(`docker-compose.dev.mac.yml`, `docker-compose.dev.linux.yml`). To run it without questions, see
[non-interactive mode](output.md#non-interactive-mode).

### Docker files outside the project

If you want the Docker files outside your project (not tracked by git), run `hm setup` from a
wrapper directory and answer the `Magento root dir` question with the relative path of your
project:

```
 ./wrapper_folder
    |__ config/
    |__ docker-compose.yml
    |__ docker-compose.dev.linux.yml
    |__ docker-compose.dev.mac.yml
    |__ <your_project>/
        |__ app/
        |__ ...
```

See [setup](setup.md) and [mysql](mysql.md).

## A new project

From a new, empty directory:

```bash
hm create-project
```

See [create-project](create-project.md).

## When something does not work

```bash
hm doctor             # diagnose the machine and the project
hm logs -f phpfpm     # follow a service's log
hm describe           # versions, services, URLs and state of this project
hm status             # how the environment is doing: containers, last errors, URLs answering
hm version            # what to paste into a bug report: hm, Docker and Compose versions
```

See [doctor](doctor.md), [logs](logs.md), [describe](describe.md), [status](status.md) and
[version](version.md).

## The dashboard

`hm tui` opens every environment on this machine on one screen: what each one is doing,
which branch it is on, the warnings from `doctor`, and the actions one key away — start,
stop, restart, follow logs, open in the browser.

It does not need to run inside a project, and it has no privileges of its own: the data is
the JSON from `list`, `describe` and `doctor`, and every action runs the real command inside
that environment's directory, so the same protections apply. See [tui](tui.md).

## Several projects and branches at once

**The proxy.** Without it every project publishes ports 80, 443, 3306... on the machine, so only
one can be up at a time. With `hm proxy up` and `"USE_PROXY": "true"` in a project's
`config/docker/properties.json`, projects stop publishing ports and are told apart by their
domain. See [proxy](proxy.md) and [dns](dns.md).

**Worktrees.** `hm worktree add <branch>` creates a git worktree for the branch and gives it an
environment of its own — its own address, containers and database — next to the main one:

```bash
hm worktree add feature/checkout
hm worktree list
hm worktree remove feature-checkout
```

From a git worktree that has no environment of its own, the commands that would act on the main
environment are refused (exit `6`) instead of repointing or destroying it; `--force` runs one of
them anyway, for that invocation only. See [worktree](worktree.md).

**Database copies.** `hm db snapshot` before something risky and `hm db restore` after;
`hm db freeze` and `hm db clone` stand a second environment's database up in seconds. See
[db](db.md).

**Sharing.** `hm share` gives the project a temporary public address until you press Ctrl-C;
`hm tunnel <service>` opens a temporary way in to a service such as `db` or `rabbitmq`. See
[share](share.md) and [tunnel](tunnel.md).

## Shell completion

`hm completion bash` and `hm completion zsh` print the completion script, built from the
commands the binary itself declares. The installer writes it to `~/.hm/completion/` for you; to
load it by hand, add `source <(hm completion bash)` to `~/.bashrc`, or
`source <(hm completion zsh)` to `~/.zshrc` after `compinit`. See [completion](completion.md).

## Next

- [Commands](README.md#commands): every command, grouped as `hm --help` groups them.
- [Output, exit codes and non-interactive use](output.md): for scripts, CI and agents.
- [Project commands](project-commands.md): add your own `hm <name>`.
