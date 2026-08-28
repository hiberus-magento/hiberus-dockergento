---
name: dockergento-debugging
description: Diagnose and debug a Hiberus Dockergento (hm) Magento project - Xdebug, Varnish, caches and indexes, logs, static analysis and tests. Use when something does not work, when a page is slow or wrong, or before handing changed PHP back.
---

# Dockergento: debugging

## Start with the diagnosis, not with a theory

```bash
hm doctor --json              # every check, with a hint each
hm doctor --only=ports        # one of them
hm describe --json            # versions, services, URLs, state
hm logs -n 200 phpfpm         # the last lines of one service
```

`hm doctor` already knows the failures this stack actually has: Docker not running, ports taken
by another project, a bind mount outside what the VM shares, a domain that does not resolve, an
image that cannot be pulled. Read it before guessing.

## Xdebug

```bash
hm debug-on
hm debug-off
```

It restarts the PHP container. Leave it off when you are done: every request is slower with it on,
and a test suite noticeably so.

## Varnish

```bash
hm varnish-on
hm varnish-off
```

Turn it off while debugging anything about a page's content: a full page cache in front of the
site will happily serve you the answer to the previous question. Turn it back on before drawing
conclusions about performance.

## Caches and indexes

```bash
hm magento cache:status
hm magento cache:flush
hm magento cache:clean config layout
hm magento indexer:status
hm magento indexer:reindex catalog_product_price
```

A change that "does nothing" is usually a cache, and a catalogue that looks wrong is usually an
index. Check both before reading code.

`hm purge` removes generated code, static content and preprocessed views. It is slow and
destructive to nothing but derived files, so it is safe — but it is a last resort, not a first
step.

## Static checks, before handing code back

```bash
hm verify              # every check this project has installed
hm verify --changed    # only what git says changed
hm verify --all        # plus the test suite and dependency injection compilation
```

`hm verify` discovers what the project actually has in `vendor/bin` — phpcs, phpstan,
php-cs-fixer, phpunit — and runs it inside the container with the right PHP version. It exits
non-zero if anything failed, so it can be branched on.

## Tests

```bash
hm test-unit
hm test-unit app/code/Vendor/Module/Test/Unit
hm test-integration
```

## Running things by hand

```bash
hm exec php -i                  # any command in the php container
hm exec -r cat /etc/nginx/conf.d/default.conf
hm exec php -d memory_limit=-1 bin/magento setup:upgrade
```

`hm bash` opens an interactive shell and takes no command — it understands only `-r`. Passing it
a command silently opens a shell instead, which from an agent means hanging or doing nothing.

For Magento and Composer use `hm magento` and `hm composer`: on macOS the code lives in a volume
and both do the synchronisation that a plain shell does not.

## Where the logs are

```bash
hm logs -f varnish              # container logs, per service
hm exec tail -n 100 var/log/system.log
hm exec tail -n 100 var/log/exception.log
```

Magento's own logs are inside the project, in `var/log/`. The container logs are what the service
itself printed — a PHP fatal error is usually in `phpfpm`, a 502 in `nginx`.
