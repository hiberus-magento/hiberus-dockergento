# Design

## Two things that fail differently

Pointing a name at this machine needs the system password and touches a file every program reads.
Telling Magento what it answers on is a row in a database. They are done in that order and reported
separately, because the second failing does not undo the first and the first failing means the
second is pointless.

## Asking about the result, not about who produces it

Whether an entry is needed is decided by asking the machine to resolve the name and looking at what
comes back: every answer a loopback address means it already reaches here, whoever arranged that. A
name that resolves to a real internet address belongs to somebody, and pointing it at this machine
is exactly what is wanted then.

## The marker is the point

An entry with nothing to say where it came from accumulates for as long as the machine lives and
nobody dares delete one. With a marker there are two different questions — does this name resolve
here, and did this tool arrange that — and only the second one licenses removing a line.
