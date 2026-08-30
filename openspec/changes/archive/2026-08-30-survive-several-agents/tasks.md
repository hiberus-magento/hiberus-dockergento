# Tasks

## The lock

- [x] `console/helpers/lock.sh`: acquire, release, break a stale lock, bounded wait
- [x] Used by the worktree registry, the proxy configuration and the hosts file

## Atomic writes

- [x] `mktemp` instead of the four fixed-name temporaries
- [x] The worktree registry written through a temporary and a rename

## The worktree's own configuration

- [x] Re-derive the properties directory and re-read the properties once resolved
- [x] `HM_AGENT` stamped for the agent profile

## Collisions

- [x] Refuse a compose project name already in use
- [x] Refuse an address already routed by another environment
- [x] Say which branch holds a name that a new one reduces to

## The hosts file

- [x] Mark the entries the tool adds
- [x] `hm set-host --remove`
- [x] `hm clean` reports the marked entries whose environment is gone

## Verification

- [x] Unit tests for the lock, including a stale one and the timeout
- [x] Integration test: two worktree creations at once leave one registry, not half of two
- [x] Integration test: the properties of a worktree are its own
- [x] Documentation, changelog and backlog
