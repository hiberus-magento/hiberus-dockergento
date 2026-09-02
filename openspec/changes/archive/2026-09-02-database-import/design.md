# Design

## Why it was not "can't"

Nothing about the import is hard. It is a keystone: it holds up three pieces that are shared, and
saying "it stays in shell" was choosing not to lay them yet. Once the question was asked properly
the answer was to lay them, because everything after this needs them.

## The three pieces

**Progress.** The engine says a step is beginning and gets back the function that ends it. The
command line draws a spinner; an HTTP adapter would send events. Whether it animates is the shell
implementation's decision, condition for condition — a terminal, text output, colour allowed,
nobody asking for silence, a TERM that can draw — because getting it wrong in either direction
means a spinner in a log file or a command that looks hung for a minute.

**Asking.** The engine says what it needs and what it would guess; who answers is not its
business. With nobody to ask, the suggestion wins, and a question with no suggestion fails with
something actionable rather than hanging — which is what a script needs and what the shell
implementation already did.

**A one-off container.** The anonymiser is a tool in its own image attached to the network the
database is on. It is deliberately not a service of the project: a container in the compose file
is a container somebody has to remember to remove.

## Streaming instead of copying

The DEFINER clauses are stripped as the dump is read. The shell implementation wrote the cleaned
copy to a file beside the original and left it there; on a Magento dump that is another gigabyte
on somebody's disk, every time.

The expression that does the stripping is the shell one's, kept exactly. It is the one that has
been run against real dumps, and a regular expression over SQL is not the place to be clever. It
was checked against the shell implementation's on both shapes of dump — the one `mysqldump`
produces and a hand-written one — and they agree, including where they are both crude.

## The order, and why it is that order

The record of the data having been anonymised is cleared as soon as the new contents are in and
before anything else can fail. Whatever the dump brought, nobody anonymised it, and a reassuring
"yes" left over from before an import is worse than no record at all.

The domain is read *before* the import, because the import replaces the row it is read from. And
only for a project that never declared one: a project that has said what its domain is has said it.

## What a failure afterwards means

The import can succeed and the configuration fail — no php container, a broken Magento. The data
is in and the store still carries the addresses of wherever the dump was taken, which will redirect
to production the first time somebody opens it. That is worth its own sentence, with the exit code
the shell implementation gives, rather than passing on whatever Docker said.
