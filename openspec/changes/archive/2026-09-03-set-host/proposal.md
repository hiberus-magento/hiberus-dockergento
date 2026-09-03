# `set-host`, and testing what it does to the machine

## Why

Every entry in the hosts file costs a password prompt, and they were never removed — twenty-three
of them on the machine this was written on, several from projects that no longer exist. Two things
follow, and both were in the shell implementation: nothing is added when the name already resolves
here, and what is added carries a marker so the tool can find its own and leave alone what a person
wrote.

None of it was ever tested, because testing it meant writing to `/etc/hosts`.

## What Changes

- **`set-host` is Go**, including `--remove` and `--no-database`.
- **The hosts file is read from somewhere that can be pointed elsewhere.** That is the whole
  difference: the behaviour is the same, and now there is a test that a name already resolving here
  is left alone, that a line somebody wrote by hand is not this tool's to delete, and that removing
  one leaves the rest of the file exactly as it was.
- **It writes directly when it can**, and asks for the password only when it cannot. On a file this
  user owns there is nothing to elevate, which is what makes the test possible at all — and on
  `/etc/hosts` it behaves as it always did.
- **A domain typed as a URL is written as a name.** `https://shop.test/` is a name with punctuation
  around it, and what goes in a hosts file is the name.

## A trap this found

The test first used `.test` domains and nothing was added. That was the command working: this
machine has a wildcard resolver for `.test`, which is exactly the case the command exists to
notice. The domains are `.invalid` now — reserved never to resolve — so what is being tested is the
adding rather than the machine.
