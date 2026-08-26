# Domains and how they resolve

A project is reached by its domain, and that domain has to point at your machine. There are two
ways for that to be true, and the tool now checks which before doing anything.

## `/etc/hosts`, the old way

`hm set-host` adds a line and asks for your system password to do it. One prompt per project, and
the lines are never removed — on the machine this was written on there were twenty-three, several
belonging to projects that no longer exist.

## A wildcard resolver, the way that needs nothing

If `*.test` resolves to `127.0.0.1`, then **every** domain under it works with nothing written
anywhere:

```console
$ dscacheutil -q host -a name anything-at-all.test
ip_address: 127.0.0.1
```

When that is the case, `hm set-host` says so and leaves `/etc/hosts` alone:

```console
$ hm set-host shop.test
shop.test already resolves to this machine, so /etc/hosts was left alone.
```

`hm doctor` reports which of the two is in play, or that the domain does not resolve at all.

## `.local` cannot work this way, and that is not a bug

macOS routes `.local` to mDNS — Bonjour — not to DNS:

```console
$ scutil --dns
resolver #2
  domain   : local
  options  : mdns
```

A DNS server never sees those queries. `.local` domains work today because `/etc/hosts` is
consulted before any resolution happens, and they keep working exactly as before.

So this only helps projects on another TLD. The TLD is each project's choice; `.test` is the one
reserved for exactly this, and the one Warden and DDEV use.

## Getting wildcard resolution

The tool does not install a resolver of its own — on many machines one is already there, put by
something else, and fighting it for port 53 would break a working setup to save a prompt.

You may already have one. Check:

```bash
dscacheutil -q host -a name anything-at-all.test    # macOS
getent hosts anything-at-all.test                   # Linux
```

If it answers `127.0.0.1`, there is nothing to do. Laravel Herd, DDEV and Valet all set this up,
and any of them is enough.

If it does not, dnsmasq is the usual answer:

```bash
brew install dnsmasq
echo 'address=/.test/127.0.0.1' >> $(brew --prefix)/etc/dnsmasq.conf
sudo brew services start dnsmasq
sudo mkdir -p /etc/resolver
echo 'nameserver 127.0.0.1' | sudo tee /etc/resolver/test
```

One setup, once, instead of a password prompt per project.

## What is never done for you

Nothing is removed from `/etc/hosts`. Lines already there keep working — `/etc/hosts` takes
precedence over DNS anyway, and both point to the same place. Deleting entries from a system file
that somebody may have put there by hand is not a development tool's business.
