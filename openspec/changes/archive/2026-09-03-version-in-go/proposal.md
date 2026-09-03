# `version`, and the end of the first scaffold

## Why

Two entries in the router were scaffolding, marked with an underscore and each with a written
condition for disappearing. `hm _binary` existed for one reason: while the discipline is
byte-for-byte parity with the shell implementation, a ported command cannot report anything the
shell one does not, so which build of the binary is running had nowhere to be looked at.

`version` is where it belongs, so porting `version` is what retires it.

## What Changes

- **`hm version` is Go**: the installation as its own checkout describes it, and the Docker and
  Compose underneath it. Compared against the shell implementation, document and words.
- **One field is new**, and it is the one the shell half cannot answer about itself: which build of
  the binary is running. It is the last line, because nobody needs it until they are reporting
  something about the ported half.
- **`hm _binary` is gone.**
- **Not being able to reach Docker is reported rather than failed on.** A report that says
  "docker: not available" is more useful than no report, and the reason somebody is running this
  command may be that Docker is not running.

## What is still described the same way

`git describe --tags` without `--abbrev=0`, on purpose: rounded to the nearest tag it said "1.4.5"
while eleven commits ahead of it, so whoever reported a bug could not say what they were reporting
it against. And only tracked changes count as uncommitted: untracked files are the user's own and
do not say which version is installed.
