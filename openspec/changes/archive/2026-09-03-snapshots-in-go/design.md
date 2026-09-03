# Design

## Size is measured, not recorded

A template carries its size in a label written when it was frozen, so both implementations read
the same string and neither has to compute it. A snapshot is a file, and its size is read from
disk every time it is listed — which means the two implementations have to agree.

`du` reports disk usage rather than the length of the file, and rounds up to the precision it
shows. Both were reproduced: the blocks the file occupies, scaled by 1024, rounded up. A copy
reported smaller than it is would be the wrong answer in the one direction that matters when a
disk is filling up.

## Where the copies live

Beside the cache, outside every project, keyed by the compose project name. Two reasons that
matter more than tidiness: `config/docker` is committed, so a copy there would travel in
somebody's commit; and a copy stored inside the environment would not survive `down -v`, which is
the one moment it is needed.

## Three endings, not two

Clearing can find nothing, be refused by the person, or happen. The first two delete nothing and
they are not the same answer, so what was found is carried out of the use case alongside what was
removed — otherwise the command has to guess which sentence to print.

## Capturing is not running

Three ways to run something in a container, and the difference between them is what the output is
for.

`Run` is for a query: the answer is small, somebody reads it, and a deadline is a kindness because
a query that hangs is a bug. `Feed` is for an import: no deadline, because a real dump takes as
long as it takes. `Capture` is for a copy being written to a file — no deadline for the same
reason, and the streams kept apart, because anything the command says about itself would otherwise
be written into the file as though it were data.
