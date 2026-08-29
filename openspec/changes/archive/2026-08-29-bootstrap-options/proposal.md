# The options `setup` documents are the options `setup` takes

## Why

`hm setup` documents seven options — `--domain`, `--dump`, `--force`, `--install`,
`--project-name`, `--root-directory`, `--use-default` — and accepts none of them. Its parser is a
`getopts` string that understands the short forms only, so `hm setup --dump=dump.sql` is read as
an unknown option and the command asks the question it was told the answer to.

That is a documentation bug and a CI bug at the same time. Worse is what happens with a dump path
that does not exist: it prints a warning, carries on, and then asks interactively — so an
automated bootstrap with a wrong path hangs instead of failing.

None of this can be reached from a script, which is what makes `hm setup` the one command an
agent or a pipeline cannot use.

## What Changes

- **The long options work**: `--domain=`, `--dump=`, `--install`, `--project-name=`,
  `--root-directory=`, `--force`, `--use-default`, in the `--option=value` and `--option value`
  forms.
- **`--clean-install` and `--db-dump=`** as names for the same two things, matching Warden's
  bootstrap, because that is what people coming from it type.
- **A dump that is not there is a usage error**, with the exit code that says so. Warning and
  carrying on into a question is the behaviour that hangs a pipeline.
- **`hm setup --yes --clean-install`** completes without asking anything: the project name, the
  domain and the root directory already have defaults, and the one question with no safe default
  is now answerable on the command line.
- `hm install` takes `--use-default` as well as `-u`.
- The parsing is separated from the doing, so it can be tested without creating an environment.
