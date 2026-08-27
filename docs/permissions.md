# permissions

Prints the permission configuration an agent should be given.

```bash
hm permissions           # what an agent needs to work
hm permissions --strict  # what an agent needs to only look
```

## The problem

Somebody has to decide what an agent may run without asking. Today each person decides in their own
configuration file, and no two lists look alike. On the machine this was built for, the answer was
`Bash` — everything allowed, which means an agent could run `hm down -v` and destroy a database
without anybody authorising it.

That is not carelessness. Keeping a list of sixty commands up to date by hand is not something
anybody does. So the list is derived instead.

## Three levels

Each command declares what it does, in the same file where it declares its description and its
group:

| Level | Meaning | Examples |
|---|---|---|
| `safe` | Changes nothing. Run it a thousand times | `describe`, `list`, `doctor`, `logs`, `verify` |
| `caution` | Changes things, recoverably | `start`, `magento`, `composer`, `setup` |
| `dangerous` | Destroys data, or reaches beyond the project | `down`, `clean`, `db`, `share`, `exec` |

Two levels would not do. Treat `hm start` like `hm down -v` and the agent asks about everything,
which means nobody reads the questions; treat it like `hm describe` and it protects nothing.

By default `safe` and `caution` are allowed and `dangerous` asks. `--strict` allows only `safe`.

## Two rules worth knowing

**A command that wraps others is classified by its worst subcommand.** `hm db` takes snapshots,
which is harmless, and restores them, which replaces a database — so it asks. Same for `hm clean`,
harmless to look at and destructive with `--force`.

**A command that runs whatever it is given is dangerous, however innocent its usual use.**
`hm exec`, `hm bash`, `hm mysql` and `hm docker-compose` can run anything at all — allowing them is
equivalent to allowing everything, so they ask.

The honest exception is `hm magento` and `hm composer`. They can also run destructive things
(`setup:upgrade`, `composer update`), and they are classified `caution` anyway, because an agent
that cannot run `hm magento cache:clean` cannot do its job. That is a judgement call and this is
where it is written down.

## It writes nothing

The configuration is printed. Your settings file is yours — it may have rules of your own, comments
and an order that matters — and writing into it to save a copy and paste is not a good trade.

## Why the classification cannot drift

The tool already sorted its commands in two hand-written lists: what a worktree refuses to run, and
what needs full container labels. This is a third consumer of the same idea, so tests assert that
all of them agree: nothing a worktree refuses can be declared harmless, every command has a level,
and every command file has an entry.

Those lists stay hardcoded in Bash on purpose — they run on every invocation, and reading JSON
there would cost more than everything the performance work saved. They are a fast copy of the
declaration, and a copy nobody checks is a copy that drifts.
