# Tasks

- [x] Schema: projects, worktrees, data state, allocations — both topologies from the outset
- [x] A pure-Go SQLite store, so the Linux binaries stay cross-compilable
- [x] Slots handed out in a transaction, reused, and bounded by what Redis has
- [x] The allocation derived from the slot, pure and tested without a database
- [x] Import what the shell implementation wrote, idempotently
- [x] Tests, including twelve allocations at once
- [x] `hm-go-registry` so it can be looked at before any command owns it
