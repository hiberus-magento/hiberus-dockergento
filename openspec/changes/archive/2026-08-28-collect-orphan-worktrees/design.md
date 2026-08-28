# Design

## In `clean`, not in `worktree`

`hm worktree remove` acts on a branch environment that exists. This acts on the remains of one
that does not, which is what `hm clean` is: the command for what abandoned environments left
behind, with the same discipline — it lists by default and deletes only with `--force`.

Putting it there also means it is found. Nobody goes looking for a subcommand to clean up
something they have forgotten about; they run the cleanup command they already know.

## By name, because there is no configuration left

Everywhere else the tool talks to Compose, which reads the project's files. Those files were in
the worktree, and the worktree is what is missing.

So the containers are removed by their compose project label and the volumes by their name prefix
— both taken from the registration, which recorded them when the environment was created. That is
the reason the registration holds the compose project name rather than deriving it at read time.

## What is still not deleted

The branch. Removing a worktree does not remove what was committed on it, and neither does this.

The database snapshots of that environment, if anybody took any: they live under the project name
in `~/.hm/snapshots`, they are small, and `hm db clear` exists for them. Deleting somebody's
backups as part of a cleanup is exactly the surprise this command was built to avoid.
