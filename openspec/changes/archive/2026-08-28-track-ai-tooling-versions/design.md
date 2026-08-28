# Design

## The digest is the whole feature

Everything else follows from being able to say "this is not what we installed". So the digest is
in a helper of its own, it works on macOS and on Linux, and it digests a directory the way a
person would mean it: every file's contents, in a stable order, with the file names included, so
that a renamed or added file is a different skill.

`cksum` is the last fallback. This compares two copies of a file that is not under attack; it is
not a signature.

## Five states, and why each one is worth a word

| State | Means | Why it matters |
|---|---|---|
| `current` | Matches what was installed | Nothing to do |
| `outdated` | The tool now carries a newer copy | `ai-pull` will update it |
| `modified` | Changed since installation | **`ai-pull` will overwrite it** |
| `custom` | The tool never installed it | It is yours, and it is safe |
| `missing` | Tracked, and no longer there | Somebody deleted it |

`modified` is the one that pays for the feature. The command promises to preserve custom skills,
and it keeps that promise by leaving alone what it did not install — which means a skill somebody
improved in place, without renaming it, is lost on the next pull. Now it is said out loud, with
the advice that renaming it is what keeps it.

## Offline where offline is possible

For the skills that come with the tool there is a source of truth on the same disk: the installed
copy. So `outdated` is a real answer, computed without a network request, and it stays right on a
machine that has been offline for a week.

For a downloaded repository there is no such copy, and asking GitHub what the branch head is would
turn a local report into a network call that fails on a corporate connection. So those are
reported with their origin and their date, and nothing is claimed about freshness.

That asymmetry is the honest one: it is a consequence of following a branch rather than a version,
which is the thing the tool does and this command makes visible rather than hides.
