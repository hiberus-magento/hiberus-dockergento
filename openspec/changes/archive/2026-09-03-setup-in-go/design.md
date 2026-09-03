# Design

## Deciding, saving, writing, installing

Four steps that can be read on their own, and the first three can be tested without Docker. That
is not structure for its own sake: creating an environment takes minutes and a working daemon,
while deciding what `--db-dump=./x.sql --clean-install` means takes neither — and that is where
the mistakes were.

## When the files are regenerated

A compose file of ours is left alone. Regenerating it recreates the containers, which is a morning
of somebody's work for no change. One that is not ours is a project that has not been set up yet —
an empty checkout, or a compose file from somewhere else — and writing one is what the command is
for. Asking for it explicitly overrides both.

## The installation is a list of commands

It is written as one, and run through the bridge command by command, so that the order lives in a
single place and each step becomes a direct call as it gets ported. A step that fails stops the
rest and says which one it was: an environment half installed is worth saying out loud, and the
sentence names the command to run on its own.
