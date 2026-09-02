# `worktree add` in Go

## Why

The last of the three, and the one the other two were waiting for. While `add` wrote registrations
in shell and the ported half read them, the storage could not change: a Go half reading SQLite
while a shell half wrote JSON is a branch environment `add` created that nothing else can see.

With all three in one implementation, swapping the registry for the SQLite one is a change of
adapter — which is the next step, on its own, and now possible.

## What Changes

- **`worktree add` is Go**, and what it writes is what the shell implementation writes: the
  registration and the overlay are compared byte for byte, and so is every refusal.
- **The overlay generator is pure and tested on its own.** The mistakes it can make — a service
  written twice, a router pointing at a service the profile removed, a mount that runs into the
  next key — are all invisible until something is running, so they are read in a test instead. It
  was also generated for all three profiles, with and without mounts, and diffed against the shell
  one.
- **The same lock, taken the same way.** Two agents creating a branch environment at once only
  take turns if both implementations agree on what the lock is. They do now, and a defect on the
  way would have broken exactly that: the pid the shell writes carries a newline, and reading it
  as-is made every lock it holds look like nobody's.

## What is kept, and why each one is there

- **Names are refused, never resolved.** Appending a number would give the environment a name
  nobody chose, and here the name decides which containers, which volumes and which database are
  used.
- **The proxy is required.** Without it every branch environment publishes its own ports, which is
  the collision the proxy was built to end, with as many environments as branches this time.
- **Everything that can be refused is refused before anything is created.** Half a branch
  environment — a worktree with no registration, a registration with no overlay — is the state
  nothing else in this tool knows how to talk about.
- **Nothing after the environment exists can fail the command.** By then it is created and
  registered, and a failure there leaves something somebody can look at and fix; returning an
  error would leave it created, registered, and reported as not having happened.
