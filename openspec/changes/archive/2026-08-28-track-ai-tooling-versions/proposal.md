# Know what AI tooling is installed and where it came from

## Why

`hm ai-pull --force` goes blind. It downloads, it overwrites, and it leaves nothing behind that
can answer the questions somebody asks a week later: which skills do I have, who wrote them, are
they the current ones, and did I edit that one myself?

The registration file was meant to answer exactly that, and it does not, for a reason worth
stating: the checksum it records is computed with `sha256sum`, which does not exist on macOS and
cannot digest a directory in any case — and a skill *is* a directory. So every entry written on a
Mac recorded an empty string that looks like a checksum, and the repository each one came from was
never recorded at all.

The result is that "custom skills are preserved" — the promise the command makes — rests on
whether a directory happens to be listed, and nothing can tell an updated skill from one somebody
spent an afternoon on.

## What Changes

- **A content digest that works on both platforms**, over a file or a whole directory, with the
  file names in it so an added or renamed file changes the result.
- **Provenance is recorded at install time**: the repository, its branch or the tool version, and
  when. `ai-pull` sets it for every resource it installs.
- **`hm ai-doctor`** lists what is installed with, for each one, where it came from and which of
  five states it is in: `current`, `outdated`, `modified`, `custom` or `missing`.
- For what came with the tool, `outdated` is answered **offline**, by comparing against the copy
  the installed tool carries. For a downloaded repository the answer is where and when, because
  freshness there is a network question and saying otherwise would be a guess.
- It changes nothing. It is the command you run before deciding whether `ai-pull --force` is safe.
