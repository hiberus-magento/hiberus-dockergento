# Design

## One door into the container

Every one of these builds a command and hands it to the same call, with the same terminal rules:
a pseudo-terminal is asked for only when there is one, because asking for one where there is none
is how a command that works by hand fails in CI.

That is the whole design, and it is why they are one file. The temptation with wrappers is to let
each one reach the container the way that was convenient the day it was written, and the shell
implementations show where that ends: one of them passed a command line as a single argument for
years without anybody noticing that the tests were not running.

## Dumping is capturing

`mysqldump` uses the capture that exists for copies: the streams separated, and no deadline. A
person exporting a database is exporting one that takes as long as it takes, and a warning printed
into the middle of the file is a file that fails to load with an error naming a line rather than a
cause.
