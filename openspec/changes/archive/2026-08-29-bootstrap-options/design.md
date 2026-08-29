# Design

## Parse first, then act

The options are now read by one function that assigns and validates, and the command runs
afterwards. That ordering is the fix for the dump bug: a path that does not exist is refused
before anything has been created, rather than three steps later when the question arrives.

It is also what makes it testable. Creating an environment takes minutes and a working Docker;
deciding what `--db-dump=./x.sql --clean-install` means takes neither, and that decision is where
the bugs were.

## Two names for two things, on purpose

`--install` and `--clean-install` do the same thing, as do `--dump` and `--db-dump`. Aliases are
usually a smell, and these are not: half the department has used Warden, whose bootstrap uses the
second name of each pair, and a tool that refuses the word somebody typed in order to be tidy is
being tidy at their expense.

The short forms `-i` and `-D` keep working. Nothing that ever worked stops working.

## `--yes` and the one question that could not be answered

`hm setup` asks four things. Three have defaults — the project name and the domain are derived
from the directory, the root directory is the current one — so `--yes` could always answer them.

The fourth, "import a dump or install Magento", has no safe default: choosing wrong either wipes
a database or spends twenty minutes installing something nobody wanted. So it refuses under
`--yes`, correctly, and the fix is not to invent a default but to make it answerable in advance.
With `--clean-install` or `--db-dump=` on the command line there is nothing left to ask.

## What is not in scope

`hm setup` still creates the environment the way it always has. This is about how it is told what
to do, not about what it does.
