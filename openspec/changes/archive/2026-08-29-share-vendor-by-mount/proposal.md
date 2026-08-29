# A worktree's dependencies are mounted, never linked

## Why

`hm worktree add` links `vendor/` and `node_modules/` to the main checkout on Linux. That is
wrong, and it is wrong in the way that does not announce itself.

Composer's generated autoloader computes `$baseDir` from `dirname($vendorDir)`, and PHP resolves
`__DIR__` to the real path behind a symlink. Run with a real PHP, in the two topologies:

```
symlink:     psr-4 Vendor\ -> <main>/app/code/Vendor
             autoload files -> <main>/app/etc/NonComposerComponentRegistration.php
             modules registered: the main checkout's

bind mount:  psr-4 Vendor\ -> <worktree>/app/code/Vendor
             modules registered: the worktree's
```

It is worse than a mislaid autoloader. `autoload_files.php` loads
`NonComposerComponentRegistration.php` from `$baseDir`, and that file globs the
`registration.php` of *its own* directory. **The worktree's modules are never registered**: an
agent edits its branch, sees no effect, and is running the main checkout's code.

## What Changes

- **Nothing is linked and nothing is copied.** The main checkout's `vendor/` and `node_modules/`
  are **mounted** into the worktree's containers at the path they belong to, so `__DIR__` resolves
  where it should.
- **Read-only.** Nothing writes to `vendor` while a site runs, and read-only is what stops a
  `composer require` in one branch from corrupting the dependencies of the main checkout and of
  every other worktree at once.
- **Shared only while the dependencies are the same.** If the branch's `composer.lock` differs
  from the main checkout's, nothing is mounted and the worktree is told to run its own
  `hm composer install` — which is the honest price of having changed the dependencies.
- **`hm composer` explains itself** in a worktree with shared dependencies, instead of letting
  Composer fail with a permissions error.
