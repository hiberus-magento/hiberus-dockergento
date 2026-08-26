# tunnel

Opens a temporary way in to a service that cannot be routed by name.

```bash
hm tunnel db          # the everyday case
hm tunnel rabbitmq
hm tunnel --close     # close whatever this project left open
```

## When you need it

A project [using the proxy](proxy.md) publishes no ports, and MySQL and AMQP cannot be routed by
domain — they carry no hostname. This gives them a door for as long as you hold it open:

```console
$ hm tunnel db

db is reachable at 127.0.0.1:61662

  Leave this running while you use it. Ctrl-C closes it.
```

Point TablePlus, Sequel Ace or PhpStorm at that address. The port is chosen from whatever is free,
so tunnels to several projects can be open at the same time, and the command tells you which one
it got.

## It runs in the foreground

Like `ssh -L`: the tunnel lasts as long as the command does, and Ctrl-C closes it and removes what
it started. Nothing is left running to find days later.

If the terminal window is closed instead of interrupted, the relay survives. The next
`hm tunnel` for the same service clears it first, and `hm tunnel --close` clears whatever this
project left behind.

## Choosing the port inside the container

Taken from the image when it declares exactly one. When it declares several — RabbitMQ exposes
five — it has to be said, because guessing between 5672 and 15691 is how you end up debugging the
wrong thing:

```console
$ hm tunnel rabbitmq
Error: 'rabbitmq' exposes 5 ports, so the one to forward has to be said
Try: hm tunnel rabbitmq 4369 5671 5672 15691 15692
```

## Not only for proxied projects

It works anywhere. In a project that still publishes its ports it is redundant, but harmless.
