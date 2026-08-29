# Tasks

- [x] Compare `composer.lock` and record `vendor: shared | own` in the registration
- [x] Mount `vendor/` and `node_modules/` read-only from the overlay, no symlinks
- [x] Keep the macOS copy, and record the `subpath` improvement as 2.0 work
- [x] `hm composer` refuses, with an explanation, when dependencies are shared
- [x] Unit tests: the sharing decision and the generated mounts
- [x] Integration test: the resolved configuration keeps the code mount and adds the dependency one
- [x] `docs/worktree.md`, changelog, and ADR-004 in the 2.0 document
