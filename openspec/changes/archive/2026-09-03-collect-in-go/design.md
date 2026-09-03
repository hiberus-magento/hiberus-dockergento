# Design

## Survey and collect, kept apart

The survey touches nothing and the collection deletes only what the survey found. That is not
structure for its own sake: it is what lets the report be printed, confirmed and then acted on
without asking Docker the same questions twice and getting a different answer the second time.

## What cannot be attributed is not guessed

Two facts shape the whole command, and both are checked rather than assumed.

A data volume carries no labels of ours, only Compose's — so a volume can only be attributed
through the containers of its project, and where those are gone it cannot be attributed at all.
Those are listed and left alone.

A frozen data directory is the exception, and the largest volume this tool makes. It belongs to no
compose project, so the rule above cannot see it; it carries instead the project that made it and
where that project lived, which is the same question asked of everything else here.

## Testing it over the scene that already existed

The shell implementation's test builds a rich fixture: an environment whose directory is gone and
one whose directory is there, their volumes, a template of each, a stray volume, and two branch
registrations of which one is stale. Every assertion in it is about what the command *sees*.

So rather than building that twice, the two implementations are compared over it. That is what
caught the malformed JSON, and it is a stronger check than a second fixture would have been.
