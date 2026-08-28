# Design

## Absent, not refused

A tool that exists and refuses is worse than no tool: the model sees it, plans around it, calls
it, reads the refusal, and tries something else — usually a shell. So when the server runs without
`--write`, the write tools are not in `tools/list` at all. There is nothing to plan around.

That also makes the permission a single decision, taken once, in the place where such decisions
belong: the client's configuration, by a person, when they wire the server up.

## Bounded is smaller than a shell, not larger

The instinct is that letting an agent write is a widening. It is the opposite of what happens
here. Today an agent that has to flush a cache is given `hm magento`, which runs anything Magento
can do — including `setup:upgrade` and `setup:di:compile`. Four typed tools replace that with four
things it can do and nothing else.

Which is why the line is drawn where it is. Cleaning a cache costs a slow page. `setup:upgrade` on
a database somebody cares about costs an afternoon, and no annotation makes that acceptable
unattended.

## Annotations

Each tool carries the protocol's hints: `readOnlyHint` false for these four, `destructiveHint`
false (none of them destroys data), `idempotentHint` true for the cache and index tools. A client
that surfaces them can warn without knowing anything about Magento.

The read tools gain `readOnlyHint: true` at the same time, which they should have had from the
start.

## `config_set` is the one that needs a rule

The others take a list of cache types or an index name, and a wrong one is an error message. This
one writes to `core_config_data`.

The path is checked against the shape Magento uses — segments of lowercase letters, digits and
underscores, separated by slashes — before it is passed on. That is not a security boundary
against a hostile model, and it is not pretending to be one; it is what stops a plausible mistake
from becoming a shell argument.

The scope stays default. Setting values per website or per store view is a thing people do
deliberately, in the admin, and it is not what an agent needs while working on a branch.
