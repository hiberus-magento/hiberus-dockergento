# The import in Go, whole — and two defects it uncovered

## Why

`hm mysql -i` was the last piece of that command left in shell, and the reason I gave for leaving
it was that it drags in three things nothing had ported: a progress indicator, a way to ask a
question, and a way to run a container that belongs to no environment.

That was a scoping answer dressed as a technical one. All three are needed by the rest of the
batch — `db` asks four questions, `setup` two, `worktree` one; `db`, `install` and `setup` all
have long steps; and the anonymiser is a command of its own. Porting them here means the next
commands find them done, and it removes a command that meant two different things depending on
which implementation ran it.

## What Changes

- **`hm mysql -i` is Go**, the whole sequence: strip the DEFINER clauses, import, clear the record
  of the data having been anonymised, anonymise when asked, and point the store at this machine.
- **Three shared pieces**: the spinner, with the shell implementation's own conditions for when to
  animate; asking, which falls back to the suggestion when there is nobody to ask and fails with
  something actionable when there is no suggestion; and running a one-off container, which the
  anonymiser needs and `hm masquerade` will use.
- **The dump is streamed**, not copied. The shell implementation wrote a `-cleaned.sql` beside the
  user's dump and never removed it, which on a real Magento database is another gigabyte.
- **A failure after the import says what state things are in.** The data is in and the store is
  still pointing at wherever the dump came from, which is worth a sentence rather than whatever
  Docker returned.

## Two defects found on the way

**`-d` was declared as taking an argument.** `hm mysql -d -i dump.sql` — the form this tool
documents in its own README — read `-i` as the argument of `-d`, stopped at the file name,
**imported nothing and exited 0**. The other order was refused outright. So there was no way to
ask for a DEFINER-cleaned import at all, and the failure looked like success.

**The domain read from the database was written through the file format the tool used to have.**
That file is converted into `properties.json` on the next invocation and replaced it, so importing
a dump into a project with no domain left it with a properties.json holding nothing but the
domain — no `COMPOSE_PROJECT_NAME`, which is the name its containers, volumes and database answer
to. Fixed separately, before this, because it can happen today in 1.x.
