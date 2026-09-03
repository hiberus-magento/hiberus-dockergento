# `setup`, and the files every other command reads

## Why

`setup` is how a project becomes an environment: it decides the three things a project is — what
its containers are called, what address it answers on, and where its code lives — and writes the
compose files that follow from those. Everything else in the tool then reads them.

It is the last command of the second batch, and it is the one that needed the list to be ported
first: two of its questions have a fixed set of answers.

## What Changes

- **The instruction is read before anything is created.** A dump path that does not exist stops the
  command rather than warning and walking into a question, which is the difference between a
  pipeline that fails and one that hangs.
- **The questions are asked only where they cannot be worked out.** A project that already has
  properties keeps them as the suggestions, so running `setup` again and pressing enter through it
  leaves the project exactly as it was.
- **The mail catcher is not asked about where nobody can answer**, because mailhog is the default
  and accepting everything has to leave a project as it was built before that choice existed. The
  database mode *is* asked even there, and refused rather than guessed: importing somebody else's
  database and installing a fresh Magento are not two spellings of the same thing.
- **The compose files are generated from the templates**, with the images the table gives for the
  Magento this project resolves to, and the machine overlays with the repository's own paths
  mounted into them — skipping what the template already mounts, because a repeated volume entry
  is a mount declared twice and Compose takes the last.
- **The proxy overlay is byte for byte the shell implementation's**, compared against it. A
  repeated key in YAML is not a merge — the last one wins and the earlier block disappears without
  a word — and a service that quietly keeps its published ports is a project that cannot be up
  beside another. It is removed when the project stops using the proxy, which matters as much:
  a leftover would take the ports away and answer on nothing.
- **The name is recorded only when it is a decision.** The file is committed, so writing the name
  the directory would have given anyway is what made a second clone inherit the first one's
  identity — same containers, same volumes, neither asked for.

## What is still the shell implementation's

Everything after the files: starting the environment, `composer install`, the Magento install, the
upgrade, the address and its certificate. Three of those are not ported yet, and running the other
three through the same door keeps the order in one place instead of half here and half there.
