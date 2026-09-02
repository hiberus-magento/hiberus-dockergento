# Design

## A capability, not a command

Three things are going to need to ask the database a question: `hm mysql -q`, the step that reads
the project's domains after starting on Linux, and the install. Writing it once, in the engine,
is the difference between one implementation and three that drift.

So the shape is `Query(project, statement, out)` in the engine, and the command is the part that
decides which of the three things is being asked for. The web adapter gets it for free, which is
the point of the engine being a library.

## Capture and attach

The orchestrator's `Exec` attaches the terminal, which is right for a session and wrong for a
query: there is nothing to attach and somebody has to read the answer. So there is a second way in,
over the daemon's exec API, that captures.

That is not the same operation twice. Without a terminal the daemon multiplexes stdout and stderr
into one stream with a header per chunk, so capturing means taking that apart — which is exactly
the code an attaching exec does not have.

## The two things the shell implementation got right

**The statement goes in the environment.** `docker exec -e QUERY=...` and then `-e "$QUERY"` inside.
Passing it as an argument works until somebody's query has the wrong quote in it.

**The client is chosen inside the container.** MariaDB 11 removed the `mysql` name and the images
this tool ships span both sides of that: 10.2 has only `mysql`, 11 has only `mariadb`. Choosing
inside is also what lets the root password and the database name be read from the container's own
environment rather than carried in from outside.

Both are kept exactly, and both have a test that says why.

## Where the boundary is

`-i` is not "import". It is: clean the DEFINER clauses, import, clear the record of the data having
been anonymised, optionally anonymise and record that, then configure Magento for local
development — which prompts for a domain if there is none and writes it into the project.

Porting the import and leaving the rest would split one sequence across two implementations to
save nothing. So the whole invocation goes to the shell one, decided by a single function, the same
shape used for the Composer invocation that rewrites the host's dependency tree.
