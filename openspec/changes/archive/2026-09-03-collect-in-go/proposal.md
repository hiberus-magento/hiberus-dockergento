# `clean` in Go, and two defects in the one it replaces

## Why

The registry cannot become the live SQLite one while anything still reads the JSON files, and
`clean` was the last thing that did besides `worktree` itself. It is also the command whose whole
design is about what it must not touch, which makes it worth porting carefully rather than last.

## What Changes

- **`hm clean` is Go**: the survey, the report and the deletion, with the same document, the same
  text and the same refusal — all compared against the shell implementation over the fixture its
  own test already builds.
- **Looking stays the default.** A dry run somebody has to remember to type protects the people
  who were already being careful.
- **What it will not touch is unchanged**: a stopped project whose directory is still there is not
  rubbish; a volume whose project has no containers left could belong to anything, so it is listed
  and left alone; and entries in `/etc/hosts` are found and never removed, because that file needs
  a password and other things depend on it.

## Two defects in the shell implementation

**`hm clean --json` was malformed.** The accumulators are built with `\n` and `\t` inside double
quotes, which are two characters and not one. The text report prints them through `printf`, which
interprets them; `jq` does not. So `split("\n")` found nothing to split and every list arrived as
a **single entry with the whole blob inside it and a null reason** — for anything reading that
document, one unnamed environment instead of eight.

**The order depended on the machine's language.** `sort` without `LC_ALL=C` put `magento_dev`
before `magento-demo` here and the other way round elsewhere. It is the same defect that was
fixed in `collect_environments` in 1.7, in a file that was not looked at then.

Both are fixed in the shell implementation as well as ported, and both want backporting to 1.x.

## And one I introduced fixing the first

Interpreting the escapes by reassigning the variables broke the deletion: `$(...)` eats the
trailing newline, and the loops that delete read them with `while read`, which drops the last line
without one. Caught by the existing test, which is the second time that exact trailing newline has
cost something.
