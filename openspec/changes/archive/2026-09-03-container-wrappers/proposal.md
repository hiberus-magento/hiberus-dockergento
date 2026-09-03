# The commands that are one thing run in one container

## Why

Eight of them, and every one is a wrapper: `purge` knows which seven directories Magento can
generate again, `test-unit` knows which phpunit and which configuration, `mysqldump` knows that the
credentials are true inside the database container and nowhere else. Knowing those things is what
the tool is for.

What they must not do is differ from each other in how they reach the container, which is exactly
what the shell implementations had started doing.

## What Changes

- **`purge`, `npm`, `n98-magerun`, `test-unit`, `test-integration`, `mysqldump`, `varnish-on` and
  `varnish-off` are Go**, all of them through the same call.
- **`mysqldump` writes through this process** rather than through a shell redirection, which is
  what lets the two streams be told apart: what the dumper says about itself must not land inside
  the dump, and a file with a sentence in the middle of it is not a dump.
- **Turning Varnish off still clears what it cached.** Two things have to agree — the VCL, which
  decides whether Varnish passes everything through, and Magento's full page cache, which decides
  whether it is asked to — and pages cached before the change are served until they expire.

## A defect this found

**`hm test-unit` never ran the tests.** It passed the whole command line to `docker compose exec`
as a single argument, so Docker looked for a file whose name contained spaces and reported `stat
./vendor/bin/phpunit --config ...: no such file or directory`. The exit code came from somewhere
else and was zero, so a CI running it was told the tests passed without one having run.

Fixed in the shell implementation as well as ported, and the test compares both halves against a
phpunit that writes down how it was called — which is the only way to see that the tests ran
rather than that the command returned.
